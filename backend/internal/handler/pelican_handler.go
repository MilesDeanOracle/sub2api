package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

const (
	pelicanMaxRequestBodyBytes = 64 << 10
	pelicanRunTimeout          = 10 * time.Minute
	pelicanSSEHeartbeat        = 30 * time.Second
	// pelicanManualKeyHeader carries a manually entered API key for the model
	// list. It is a header (not a query parameter) so the key does not appear
	// in access-log URLs.
	pelicanManualKeyHeader = "X-Pelican-Api-Key"
)

// PelicanHandler exposes the fixed "pelican riding a bicycle" animation test.
// It proxies a normal OpenAI chat completion with the caller's API key and
// streams the upstream SSE response back unchanged.
type PelicanHandler struct {
	apiKeyService  *service.APIKeyService
	settingService *service.SettingService
	pelicanService *service.PelicanService

	activeMu sync.Mutex
	active   map[int64]int
}

func NewPelicanHandler(
	apiKeyService *service.APIKeyService,
	settingService *service.SettingService,
	pelicanService *service.PelicanService,
) *PelicanHandler {
	return &PelicanHandler{
		apiKeyService:  apiKeyService,
		settingService: settingService,
		pelicanService: pelicanService,
		active:         make(map[int64]int),
	}
}

type pelicanRunRequest struct {
	KeyID           int64  `json:"key_id"`
	APIKey          string `json:"api_key"`
	Model           string `json:"model"`
	ReasoningEffort string `json:"reasoning_effort"`
	BaseURL         string `json:"base_url"`
	TestID          string `json:"test_id"`
}

// Config returns the default/fallback base URL, the fixed prompt and limits.
func (h *PelicanHandler) Config(c *gin.Context) {
	if _, ok := h.userID(c); !ok {
		return
	}
	ctx := c.Request.Context()
	defaultBase := strings.TrimSpace(h.settingService.GetAPIBaseURL(ctx))
	configuredFrontend := strings.TrimSpace(h.settingService.GetFrontendURL(ctx))
	fallback := configuredFrontend
	requestOrigin := pelicanRequestOrigin(c)
	if fallback == "" {
		fallback = requestOrigin
	}
	source := "request"
	if defaultBase != "" {
		source = "api_base_url"
	} else if configuredFrontend != "" {
		source = "frontend_url"
	}

	response.Success(c, gin.H{
		"default_base_url":  defaultBase,
		"fallback_base_url": fallback,
		"base_url_source":   source,
		"test":              h.pelicanService.Test(),
		"reasoning_levels":  service.PelicanReasoningLevels,
		"limits": gin.H{
			"max_duration_seconds": int(pelicanRunTimeout / time.Second),
			"max_concurrent_runs":  1,
		},
	})
}

// Models proxies {base}/v1/models with the selected key or a manually entered
// key from the X-Pelican-Api-Key header.
func (h *PelicanHandler) Models(c *gin.Context) {
	userID, ok := h.userID(c)
	if !ok {
		return
	}
	manualKey := strings.TrimSpace(c.GetHeader(pelicanManualKeyHeader))
	apiKeyValue := ""
	if manualKey != "" {
		if err := validateManualPelicanAPIKey(manualKey); err != nil {
			response.ErrorWithDetails(c, http.StatusBadRequest, err.Error(), "KEY_INVALID", nil)
			return
		}
		apiKeyValue = manualKey
	} else {
		keyID, err := strconv.ParseInt(strings.TrimSpace(c.Query("key_id")), 10, 64)
		if err != nil || keyID <= 0 {
			response.ErrorWithDetails(c, http.StatusBadRequest, "请选择 API 密钥", "KEY_REQUIRED", nil)
			return
		}
		apiKey, ok := h.ownedKey(c, userID, keyID)
		if !ok {
			return
		}
		apiKeyValue = apiKey.Key
	}
	baseURL, source, ok := h.resolveBaseURL(c, strings.TrimSpace(c.Query("base_url")))
	if !ok {
		return
	}
	if source == "manual" {
		if err := h.pelicanService.EnsurePublicHost(c.Request.Context(), baseURL); err != nil {
			response.ErrorWithDetails(c, http.StatusBadRequest, "Base URL 不能指向内网或本机地址", "BASE_URL_PRIVATE", nil)
			return
		}
	}
	modelsCtx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	models, err := h.pelicanService.ListModels(modelsCtx, baseURL, apiKeyValue)
	if err != nil {
		h.writeUpstreamError(c, err)
		return
	}
	response.Success(c, gin.H{
		"base_url":        baseURL,
		"base_url_source": source,
		"models":          models,
		"cached":          false,
	})
}

// Run starts one streaming chat completion and pipes SSE to the browser.
func (h *PelicanHandler) Run(c *gin.Context) {
	userID, ok := h.userID(c)
	if !ok {
		return
	}

	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, pelicanMaxRequestBodyBytes)
	var req pelicanRunRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorWithDetails(c, http.StatusBadRequest, "请求参数无效", "INVALID_REQUEST", nil)
		return
	}
	if req.TestID != "" && req.TestID != service.PelicanTestID {
		response.ErrorWithDetails(c, http.StatusBadRequest, "不支持的检测项目", "TEST_NOT_SUPPORTED", nil)
		return
	}
	if strings.TrimSpace(req.Model) == "" {
		response.ErrorWithDetails(c, http.StatusBadRequest, "请选择模型", "MODEL_REQUIRED", nil)
		return
	}
	if len(req.Model) > 200 {
		response.ErrorWithDetails(c, http.StatusBadRequest, "模型名称过长", "MODEL_INVALID", nil)
		return
	}
	if effort := strings.TrimSpace(req.ReasoningEffort); effort != "" && !isPelicanReasoningLevel(effort) {
		response.ErrorWithDetails(c, http.StatusBadRequest, "不支持的推理级别", "REASONING_LEVEL_INVALID", nil)
		return
	}
	apiKeyValue := ""
	if manualKey := strings.TrimSpace(req.APIKey); manualKey != "" {
		if err := validateManualPelicanAPIKey(manualKey); err != nil {
			response.ErrorWithDetails(c, http.StatusBadRequest, err.Error(), "KEY_INVALID", nil)
			return
		}
		apiKeyValue = manualKey
	} else {
		apiKey, ok := h.ownedKey(c, userID, req.KeyID)
		if !ok {
			return
		}
		apiKeyValue = apiKey.Key
	}
	baseURL, source, ok := h.resolveBaseURL(c, strings.TrimSpace(req.BaseURL))
	if !ok {
		return
	}
	if source == "manual" {
		if err := h.pelicanService.EnsurePublicHost(c.Request.Context(), baseURL); err != nil {
			response.ErrorWithDetails(c, http.StatusBadRequest, "Base URL 不能指向内网或本机地址", "BASE_URL_PRIVATE", nil)
			return
		}
	}
	if !h.tryStart(userID) {
		response.ErrorWithDetails(c, http.StatusTooManyRequests, "已有检测在进行，请等待完成", "RUN_CONCURRENCY_LIMIT", nil)
		return
	}
	defer h.finish(userID)

	ctx, cancel := context.WithTimeout(c.Request.Context(), pelicanRunTimeout)
	defer cancel()

	upstream, err := h.pelicanService.OpenChatStream(ctx, baseURL, apiKeyValue, strings.TrimSpace(req.Model), strings.TrimSpace(req.ReasoningEffort))
	if err != nil {
		h.writeUpstreamError(c, err)
		return
	}
	defer upstream.Body.Close()

	contentType := strings.ToLower(upstream.Header.Get("Content-Type"))
	if !strings.Contains(contentType, "text/event-stream") {
		response.ErrorWithDetails(c, http.StatusBadGateway, "上游未返回流式响应", "UPSTREAM_NOT_STREAM", nil)
		return
	}

	c.Header("Content-Type", "text/event-stream; charset=utf-8")
	c.Header("Cache-Control", "no-cache, no-store")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	c.Status(http.StatusOK)

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		return
	}
	if _, err := fmt.Fprint(c.Writer, ": connected\n\n"); err != nil {
		return
	}
	flusher.Flush()

	err = copyPelicanSSE(ctx, upstream.Body, c.Writer, flusher)
	if err == nil || errors.Is(err, io.EOF) || errors.Is(err, context.Canceled) {
		return
	}
	if errors.Is(err, context.DeadlineExceeded) {
		writePelicanSSEError(c.Writer, "STREAM_TIMEOUT", "检测超时（10 分钟）")
		return
	}
	writePelicanSSEError(c.Writer, "UPSTREAM_STREAM_ERROR", "上游连接中断，请重试")
}

func (h *PelicanHandler) userID(c *gin.Context) (int64, bool) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 {
		response.Unauthorized(c, "User not authenticated")
		return 0, false
	}
	return subject.UserID, true
}

// ownedKey loads the key, verifies ownership/status/quota, and writes the error
// response itself so callers only need to return on false.
func (h *PelicanHandler) ownedKey(c *gin.Context, userID, keyID int64) (*service.APIKey, bool) {
	if keyID <= 0 {
		response.ErrorWithDetails(c, http.StatusBadRequest, "请选择 API 密钥", "KEY_REQUIRED", nil)
		return nil, false
	}
	apiKey, err := h.apiKeyService.GetByID(c.Request.Context(), keyID)
	if err != nil {
		response.ErrorWithDetails(c, http.StatusNotFound, "API 密钥不存在", "KEY_NOT_FOUND", nil)
		return nil, false
	}
	if apiKey.UserID != userID {
		response.ErrorWithDetails(c, http.StatusForbidden, "无权使用该密钥", "KEY_FORBIDDEN", nil)
		return nil, false
	}
	if !apiKey.IsActive() {
		response.ErrorWithDetails(c, http.StatusConflict, "该密钥不可用：已停用", "KEY_INACTIVE", nil)
		return nil, false
	}
	if err := h.apiKeyService.CheckAPIKeyQuotaAndExpiry(apiKey); err != nil {
		message := "该密钥不可用"
		reason := "KEY_UNAVAILABLE"
		switch {
		case errors.Is(err, service.ErrAPIKeyExpired):
			message = "该密钥已过期"
			reason = "KEY_EXPIRED"
		case errors.Is(err, service.ErrAPIKeyQuotaExhausted):
			message = "该密钥额度已用尽"
			reason = "KEY_QUOTA_EXHAUSTED"
		}
		response.ErrorWithDetails(c, http.StatusConflict, message, reason, nil)
		return nil, false
	}
	return apiKey, true
}

func (h *PelicanHandler) resolveBaseURL(c *gin.Context, override string) (string, string, bool) {
	ctx := c.Request.Context()
	baseURL, source, err := h.pelicanService.ResolveBaseURL(
		ctx,
		override,
		strings.TrimSpace(h.settingService.GetFrontendURL(ctx)),
		pelicanRequestOrigin(c),
	)
	if err != nil {
		response.ErrorWithDetails(c, http.StatusBadRequest, err.Error(), "BASE_URL_INVALID", nil)
		return "", "", false
	}
	return baseURL, source, true
}

func (h *PelicanHandler) writeUpstreamError(c *gin.Context, err error) {
	var upstreamErr *service.PelicanUpstreamError
	if errors.As(err, &upstreamErr) {
		status := http.StatusBadGateway
		reason := "UPSTREAM_ERROR"
		switch upstreamErr.StatusCode {
		case http.StatusTooManyRequests:
			status = http.StatusTooManyRequests
			reason = "UPSTREAM_RATE_LIMITED"
		case http.StatusBadRequest, http.StatusNotFound:
			status = http.StatusBadRequest
			reason = "UPSTREAM_REQUEST_REJECTED"
		case http.StatusUnauthorized, http.StatusForbidden:
			reason = "UPSTREAM_AUTH_FAILED"
		}
		message := upstreamErr.Message
		if message == "" {
			message = "上游请求失败"
		}
		response.ErrorWithDetails(c, status, message, reason, nil)
		return
	}
	if errors.Is(err, context.DeadlineExceeded) {
		response.ErrorWithDetails(c, http.StatusGatewayTimeout, "检测超时，请重试", "STREAM_TIMEOUT", nil)
		return
	}
	response.ErrorWithDetails(c, http.StatusBadGateway, "模型服务暂时不可用，请重试", "UPSTREAM_UNAVAILABLE", nil)
}

func (h *PelicanHandler) tryStart(userID int64) bool {
	h.activeMu.Lock()
	defer h.activeMu.Unlock()
	if h.active[userID] > 0 {
		return false
	}
	h.active[userID]++
	return true
}

func (h *PelicanHandler) finish(userID int64) {
	h.activeMu.Lock()
	defer h.activeMu.Unlock()
	if h.active[userID] <= 1 {
		delete(h.active, userID)
		return
	}
	h.active[userID]--
}

type pelicanReadResult struct {
	data []byte
	err  error
}

// copyPelicanSSE forwards upstream bytes and emits a comment heartbeat while
// the upstream is silent, so proxies do not close an idle connection.
func copyPelicanSSE(ctx context.Context, body io.Reader, w io.Writer, flusher http.Flusher) error {
	ch := make(chan pelicanReadResult, 1)
	go func() {
		defer close(ch)
		buf := make([]byte, 32<<10)
		for {
			n, err := body.Read(buf)
			if n > 0 {
				data := make([]byte, n)
				copy(data, buf[:n])
				select {
				case ch <- pelicanReadResult{data: data}:
				case <-ctx.Done():
					return
				}
			}
			if err != nil {
				select {
				case ch <- pelicanReadResult{err: err}:
				case <-ctx.Done():
				}
				return
			}
		}
	}()

	ticker := time.NewTicker(pelicanSSEHeartbeat)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if _, err := fmt.Fprint(w, ": ping\n\n"); err != nil {
				return err
			}
			flusher.Flush()
		case result, ok := <-ch:
			if !ok {
				return nil
			}
			if len(result.data) > 0 {
				if _, err := w.Write(result.data); err != nil {
					return err
				}
				flusher.Flush()
			}
			if result.err != nil {
				if errors.Is(result.err, io.EOF) {
					return nil
				}
				return result.err
			}
		}
	}
}

func writePelicanSSEError(w io.Writer, code, message string) {
	payload, err := json.Marshal(map[string]string{"code": code, "message": message})
	if err != nil {
		return
	}
	_, _ = fmt.Fprintf(w, "event: pelican-error\ndata: %s\n\n", payload)
}

func isPelicanReasoningLevel(value string) bool {
	for _, level := range service.PelicanReasoningLevels {
		if level == value {
			return true
		}
	}
	return false
}

// validateManualPelicanAPIKey accepts the user's own key for testing when they
// do not want to create a stored key. The key is used only for this request.
func validateManualPelicanAPIKey(value string) error {
	if value == "" {
		return errors.New("请填写 API Key")
	}
	if len(value) > 512 {
		return errors.New("API Key 过长")
	}
	if strings.ContainsAny(value, " \t\r\n") {
		return errors.New("API Key 不能包含空格或换行")
	}
	return nil
}

func pelicanRequestOrigin(c *gin.Context) string {
	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}
	if proto := firstForwardedValue(c.GetHeader("X-Forwarded-Proto")); proto != "" {
		scheme = proto
	}
	// Deliberately use Request.Host (the host the client connected to) instead
	// of X-Forwarded-Host: the fallback URL becomes an outbound request target,
	// and a spoofed forwarded host would turn this endpoint into an SSRF probe.
	host := c.Request.Host
	if host == "" {
		return ""
	}
	return scheme + "://" + host
}

func firstForwardedValue(value string) string {
	if idx := strings.Index(value, ","); idx >= 0 {
		value = value[:idx]
	}
	return strings.TrimSpace(value)
}
