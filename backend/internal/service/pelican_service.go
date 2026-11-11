package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const (
	// PelicanTestID is the only test supported in the first version.
	PelicanTestID = "pelican-riding"
	// PelicanTestTitle is shown in the page header and history.
	PelicanTestTitle = "鹈鹕骑行"
	// PelicanPromptVersion is incremented whenever PelicanPrompt changes.
	PelicanPromptVersion = "pelican-riding-animation-v1"

	// PelicanPrompt is the exact user prompt for the animation benchmark.
	PelicanPrompt = "请生成可直接运行的单文件HTML，使用内联SVG绘制鹈鹕骑自行车的二维循环动画。画面以鹈鹕和自行车为主体，展示清晰的身体结构、踩踏动作和车轮转动，配合协调的背景、配色与层次。动画应流畅自然、衔接连续，并适配不同屏幕尺寸。禁止依赖外部资源，只输出完整HTML，不要代码围栏或解释文字。"

	pelicanMaxUpstreamErrorBytes  = 64 << 10
	pelicanMaxModelsResponseBytes = 2 << 20
)

// PelicanReasoningLevels mirrors the gateway's supported reasoning effort names.
var PelicanReasoningLevels = []string{"none", "minimal", "low", "medium", "high", "xhigh", "max"}

// PelicanModel is one model returned by the upstream OpenAI-compatible /v1/models.
type PelicanModel struct {
	ID      string `json:"id"`
	OwnedBy string `json:"owned_by,omitempty"`
}

// PelicanTest describes the fixed benchmark prompt exposed to the page.
type PelicanTest struct {
	ID            string `json:"id"`
	Title         string `json:"title"`
	PromptVersion string `json:"prompt_version"`
	Prompt        string `json:"prompt"`
}

// PelicanUpstreamError carries a sanitized upstream status/message.
type PelicanUpstreamError struct {
	StatusCode int
	Message    string
}

func (e *PelicanUpstreamError) Error() string {
	return fmt.Sprintf("upstream status %d: %s", e.StatusCode, e.Message)
}

// PelicanService talks to an OpenAI-compatible endpoint with the caller's key.
// It never persists requests or responses.
type PelicanService struct {
	settingService *SettingService
	client         *http.Client
	allowPrivate   bool
}

// NewPelicanService builds the service with an isolated HTTP transport so the
// pelican traffic does not share gateway/account-proxy connection pools.
func NewPelicanService(settingService *SettingService) *PelicanService {
	transport := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           (&net.Dialer{Timeout: 15 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
		TLSHandshakeTimeout:   15 * time.Second,
		ResponseHeaderTimeout: 180 * time.Second,
		IdleConnTimeout:       90 * time.Second,
		DisableCompression:    true,
		ForceAttemptHTTP2:     true,
	}
	return &PelicanService{
		settingService: settingService,
		client:         &http.Client{Transport: transport},
		allowPrivate:   strings.EqualFold(strings.TrimSpace(os.Getenv("PELICAN_ALLOW_PRIVATE_BASE_URL")), "true"),
	}
}

// Test returns the fixed prompt metadata.
func (s *PelicanService) Test() PelicanTest {
	return PelicanTest{
		ID:            PelicanTestID,
		Title:         PelicanTestTitle,
		PromptVersion: PelicanPromptVersion,
		Prompt:        PelicanPrompt,
	}
}

// ResolveBaseURL resolves the base URL in priority order:
// manual override > PELICAN_TEST_BASE_URL > api_base_url > frontend URL > request origin.
// The source is returned for display in the page.
func (s *PelicanService) ResolveBaseURL(ctx context.Context, override, frontendURL, requestOrigin string) (string, string, error) {
	if base, err := normalizePelicanBaseURL(override); err != nil {
		return "", "", err
	} else if base != "" {
		return base, "manual", nil
	}
	if env := strings.TrimSpace(os.Getenv("PELICAN_TEST_BASE_URL")); env != "" {
		base, err := normalizePelicanBaseURL(env)
		if err != nil {
			return "", "", err
		}
		return base, "env", nil
	}
	if s.settingService != nil {
		if raw := strings.TrimSpace(s.settingService.GetAPIBaseURL(ctx)); raw != "" {
			if base, err := normalizePelicanBaseURL(raw); err == nil && base != "" {
				return base, "api_base_url", nil
			}
		}
	}
	if base, err := normalizePelicanBaseURL(frontendURL); err == nil && base != "" {
		return base, "frontend_url", nil
	}
	base, err := normalizePelicanBaseURL(requestOrigin)
	if err != nil {
		return "", "", err
	}
	if base == "" {
		return "", "", errors.New("base URL is not configured")
	}
	return base, "request", nil
}

// EnsurePublicHost rejects manual base URLs that point at private/loopback
// addresses, so the authenticated endpoint cannot be used as an SSRF probe.
// Administrator-provided defaults (settings/env) are considered trusted.
func (s *PelicanService) EnsurePublicHost(ctx context.Context, baseURL string) error {
	if s.allowPrivate {
		return nil
	}
	u, err := url.Parse(baseURL)
	if err != nil || u.Hostname() == "" {
		return errors.New("invalid base URL")
	}
	host := u.Hostname()
	if ip := net.ParseIP(host); ip != nil {
		if isDisallowedPelicanIP(ip) {
			return errors.New("base URL must not point to a private address")
		}
		return nil
	}
	lookupCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	ips, err := net.DefaultResolver.LookupIPAddr(lookupCtx, host)
	if err != nil {
		// If DNS fails the request will fail anyway; do not turn this into a
		// separate validation error.
		return nil
	}
	for _, item := range ips {
		if isDisallowedPelicanIP(item.IP) {
			return errors.New("base URL must not point to a private address")
		}
	}
	return nil
}

// ListModels calls {base}/v1/models with the user's key.
func (s *PelicanService) ListModels(ctx context.Context, baseURL, apiKey string) ([]PelicanModel, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, buildPelicanAPIURL(baseURL, "/models"), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "sub2api-pelican/1.0")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("模型列表请求失败: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, pelicanMaxModelsResponseBytes))
	if err != nil {
		return nil, fmt.Errorf("模型列表读取失败: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &PelicanUpstreamError{
			StatusCode: resp.StatusCode,
			Message:    extractPelicanErrorMessage(body),
		}
	}
	var payload struct {
		Data []PelicanModel `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("模型列表响应无效: %w", err)
	}
	models := make([]PelicanModel, 0, len(payload.Data))
	for _, model := range payload.Data {
		if strings.TrimSpace(model.ID) == "" {
			continue
		}
		models = append(models, model)
	}
	return models, nil
}

// OpenChatStream starts a streaming chat completion with stream usage enabled.
// The caller owns the response body and must close it.
func (s *PelicanService) OpenChatStream(ctx context.Context, baseURL, apiKey, model, reasoningEffort string) (*http.Response, error) {
	payload := map[string]any{
		"model":          model,
		"stream":         true,
		"stream_options": map[string]any{"include_usage": true},
		"messages": []map[string]string{
			{"role": "user", "content": PelicanPrompt},
		},
	}
	if effort := strings.TrimSpace(reasoningEffort); effort != "" {
		payload["reasoning_effort"] = effort
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, buildPelicanAPIURL(baseURL, "/chat/completions"), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("User-Agent", "sub2api-pelican/1.0")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("模型请求失败: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		defer resp.Body.Close()
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, pelicanMaxUpstreamErrorBytes))
		return nil, &PelicanUpstreamError{
			StatusCode: resp.StatusCode,
			Message:    extractPelicanErrorMessage(raw),
		}
	}
	return resp, nil
}

func buildPelicanAPIURL(baseURL, endpoint string) string {
	base := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if strings.HasSuffix(base, "/v1") {
		return base + endpoint
	}
	return base + "/v1" + endpoint
}

func normalizePelicanBaseURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", errors.New("Base URL 格式无效")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", errors.New("Base URL 必须以 http:// 或 https:// 开头")
	}
	if u.Host == "" || u.Hostname() == "" {
		return "", errors.New("Base URL 缺少主机名")
	}
	if u.User != nil {
		return "", errors.New("Base URL 不能包含账号密码")
	}
	if u.RawQuery != "" || u.Fragment != "" {
		return "", errors.New("Base URL 不能包含查询参数或片段")
	}
	path := strings.TrimRight(u.Path, "/")
	if strings.HasSuffix(path, "/v1") {
		path = strings.TrimSuffix(path, "/v1")
	}
	return u.Scheme + "://" + u.Host + path, nil
}

func isDisallowedPelicanIP(ip net.IP) bool {
	return ip.IsLoopback() ||
		ip.IsPrivate() ||
		ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() ||
		ip.IsUnspecified() ||
		ip.IsMulticast()
}

func extractPelicanErrorMessage(body []byte) string {
	var payload struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &payload); err == nil {
		if msg := strings.TrimSpace(payload.Error.Message); msg != "" {
			return truncatePelicanMessage(redactPelicanSecrets(msg))
		}
		if msg := strings.TrimSpace(payload.Message); msg != "" {
			return truncatePelicanMessage(redactPelicanSecrets(msg))
		}
	}
	text := strings.TrimSpace(string(body))
	if text == "" {
		return "上游返回了空错误信息"
	}
	return truncatePelicanMessage(redactPelicanSecrets(text))
}

func truncatePelicanMessage(message string) string {
	runes := []rune(message)
	if len(runes) <= 300 {
		return message
	}
	return string(runes[:300]) + "…"
}

func redactPelicanSecrets(message string) string {
	parts := strings.Fields(message)
	for i, part := range parts {
		if strings.Contains(part, "sk-") {
			parts[i] = "[已隐藏]"
		}
	}
	return strings.Join(parts, " ")
}
