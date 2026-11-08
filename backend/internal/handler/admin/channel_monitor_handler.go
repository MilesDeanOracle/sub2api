package admin

import (
	"context"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

const (
	// monitorMaxPageSize 列表分页上限。
	monitorMaxPageSize = 100
	// monitorAPIKeyMaskPrefix 脱敏时保留的明文前缀长度。
	monitorAPIKeyMaskPrefix = 4
	// monitorAPIKeyMaskSuffix 脱敏后追加的占位字符串。
	monitorAPIKeyMaskSuffix = "***"
)

// ChannelMonitorHandler 渠道监控管理后台 handler。
type ChannelMonitorHandler struct {
	monitorService *service.ChannelMonitorService
}

// NewChannelMonitorHandler 创建 handler。
func NewChannelMonitorHandler(monitorService *service.ChannelMonitorService) *ChannelMonitorHandler {
	return &ChannelMonitorHandler{monitorService: monitorService}
}

// --- Request / Response ---

type channelMonitorCreateRequest struct {
	Name             string            `json:"name" binding:"required,max=100"`
	Provider         string            `json:"provider" binding:"required,oneof=openai anthropic gemini grok antigravity kimi zhipu deepseek minimax opencode_go"`
	APIMode          string            `json:"api_mode" binding:"omitempty,oneof=chat_completions responses"`
	Endpoint         string            `json:"endpoint" binding:"omitempty,max=500"`
	APIKey           string            `json:"api_key" binding:"omitempty,max=2000"`
	PrimaryModel     string            `json:"primary_model" binding:"max=200"`
	ExtraModels      []string          `json:"extra_models"`
	GroupName        string            `json:"group_name" binding:"max=100"`
	Enabled          *bool             `json:"enabled"`
	IntervalSeconds  int               `json:"interval_seconds" binding:"required,min=15,max=3600"`
	JitterSeconds    int               `json:"jitter_seconds" binding:"omitempty,min=0,max=3585"`
	TemplateID       *int64            `json:"template_id"`
	ExtraHeaders     map[string]string `json:"extra_headers"`
	BodyOverrideMode string            `json:"body_override_mode" binding:"omitempty,oneof=off merge replace"`
	BodyOverride     map[string]any    `json:"body_override"`

	// CheckMode: probe（默认）/ quota / quota_probe。quota 模式 endpoint/api_key
	// 可空（条件必填校验在 service 层按模式分支）。
	CheckMode string `json:"check_mode" binding:"omitempty,oneof=probe quota quota_probe"`
	// AccountID: 配额模式关联的账号 ID。
	AccountID *int64 `json:"account_id"`
}

type channelMonitorUpdateRequest struct {
	Name             *string            `json:"name" binding:"omitempty,max=100"`
	Provider         *string            `json:"provider" binding:"omitempty,oneof=openai anthropic gemini grok antigravity kimi zhipu deepseek minimax opencode_go"`
	APIMode          *string            `json:"api_mode" binding:"omitempty,oneof=chat_completions responses"`
	Endpoint         *string            `json:"endpoint" binding:"omitempty,max=500"`
	APIKey           *string            `json:"api_key" binding:"omitempty,max=2000"`
	PrimaryModel     *string            `json:"primary_model" binding:"omitempty,max=200"`
	ExtraModels      *[]string          `json:"extra_models"`
	GroupName        *string            `json:"group_name" binding:"omitempty,max=100"`
	Enabled          *bool              `json:"enabled"`
	IntervalSeconds  *int               `json:"interval_seconds" binding:"omitempty,min=15,max=3600"`
	JitterSeconds    *int               `json:"jitter_seconds" binding:"omitempty,min=0,max=3585"`
	TemplateID       *int64             `json:"template_id"`
	ClearTemplate    bool               `json:"clear_template"` // true 时把 template_id 置空，忽略 TemplateID
	ExtraHeaders     *map[string]string `json:"extra_headers"`
	BodyOverrideMode *string            `json:"body_override_mode" binding:"omitempty,oneof=off merge replace"`
	BodyOverride     *map[string]any    `json:"body_override"`

	// CheckMode/AccountID：nil = 不更新；AccountID 指向 0 = 清空关联。
	CheckMode *string `json:"check_mode" binding:"omitempty,oneof=probe quota quota_probe"`
	AccountID *int64  `json:"account_id"`
}

type channelMonitorResponse struct {
	ID                  int64                                `json:"id"`
	Name                string                               `json:"name"`
	Provider            string                               `json:"provider"`
	APIMode             string                               `json:"api_mode"`
	Endpoint            string                               `json:"endpoint"`
	APIKeyMasked        string                               `json:"api_key_masked"`
	APIKeyDecryptFailed bool                                 `json:"api_key_decrypt_failed"`
	PrimaryModel        string                               `json:"primary_model"`
	ExtraModels         []string                             `json:"extra_models"`
	GroupName           string                               `json:"group_name"`
	Enabled             bool                                 `json:"enabled"`
	IntervalSeconds     int                                  `json:"interval_seconds"`
	JitterSeconds       int                                  `json:"jitter_seconds"`
	LastCheckedAt       *string                              `json:"last_checked_at"`
	CreatedBy           int64                                `json:"created_by"`
	CreatedAt           string                               `json:"created_at"`
	UpdatedAt           string                               `json:"updated_at"`
	PrimaryStatus       string                               `json:"primary_status"`
	PrimaryLatencyMs    *int                                 `json:"primary_latency_ms"`
	Availability7d      float64                              `json:"availability_7d"`
	ExtraModelsStatus   []dto.ChannelMonitorExtraModelStatus `json:"extra_models_status"`
	// 请求自定义快照：前端编辑 / 展示「高级设置」用
	TemplateID       *int64            `json:"template_id"`
	ExtraHeaders     map[string]string `json:"extra_headers"`
	BodyOverrideMode string            `json:"body_override_mode"`
	BodyOverride     map[string]any    `json:"body_override"`

	// 配额模式：check_mode + 关联账号 + 主模型最近配额快照
	// （LatestQuota 由 List handler 批量聚合后填充；管理端不受 channel_monitor_show_quota 影响）。
	CheckMode   string                       `json:"check_mode"`
	AccountID   *int64                       `json:"account_id"`
	LatestQuota *domain.MonitorQuotaSnapshot `json:"latest_quota,omitempty"`
}

type channelMonitorCheckResultResponse struct {
	Model         string                       `json:"model"`
	Status        string                       `json:"status"`
	LatencyMs     *int                         `json:"latency_ms"`
	PingLatencyMs *int                         `json:"ping_latency_ms"`
	Message       string                       `json:"message"`
	CheckedAt     string                       `json:"checked_at"`
	Quota         *domain.MonitorQuotaSnapshot `json:"quota,omitempty"`
}

type channelMonitorHistoryItemResponse struct {
	ID            int64                        `json:"id"`
	Model         string                       `json:"model"`
	Status        string                       `json:"status"`
	LatencyMs     *int                         `json:"latency_ms"`
	PingLatencyMs *int                         `json:"ping_latency_ms"`
	Message       string                       `json:"message"`
	CheckedAt     string                       `json:"checked_at"`
	Quota         *domain.MonitorQuotaSnapshot `json:"quota,omitempty"`
	// Substituted / AnomalyID：该行是否为「上一次正常结果」的复制行
	// 及关联的探测异常记录 id（点击可查看重试明细）；普通行为 false / null。
	Substituted bool   `json:"substituted"`
	AnomalyID   *int64 `json:"anomaly_id"`
}

// maskAPIKey 对 API Key 明文做脱敏：前 4 字符 + "***"，长度 ≤ 4 时只显示 "***"。
func maskAPIKey(plain string) string {
	if len(plain) <= monitorAPIKeyMaskPrefix {
		return monitorAPIKeyMaskSuffix
	}
	return plain[:monitorAPIKeyMaskPrefix] + monitorAPIKeyMaskSuffix
}

func channelMonitorToResponse(m *service.ChannelMonitor) *channelMonitorResponse {
	if m == nil {
		return nil
	}
	extras := m.ExtraModels
	if extras == nil {
		extras = []string{}
	}
	headers := m.ExtraHeaders
	if headers == nil {
		headers = map[string]string{}
	}
	resp := &channelMonitorResponse{
		ID:                  m.ID,
		Name:                m.Name,
		Provider:            m.Provider,
		APIMode:             m.APIMode,
		Endpoint:            m.Endpoint,
		APIKeyMasked:        maskAPIKey(m.APIKey),
		APIKeyDecryptFailed: m.APIKeyDecryptFailed,
		PrimaryModel:        m.PrimaryModel,
		ExtraModels:         extras,
		GroupName:           m.GroupName,
		Enabled:             m.Enabled,
		IntervalSeconds:     m.IntervalSeconds,
		JitterSeconds:       m.JitterSeconds,
		CreatedBy:           m.CreatedBy,
		CreatedAt:           m.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:           m.UpdatedAt.UTC().Format(time.RFC3339),
		TemplateID:          m.TemplateID,
		ExtraHeaders:        headers,
		BodyOverrideMode:    m.BodyOverrideMode,
		BodyOverride:        m.BodyOverride,
		CheckMode:           m.CheckMode,
		AccountID:           m.AccountID,
		// PrimaryStatus / PrimaryLatencyMs / Availability7d / LatestQuota
		// 由 List handler 在批量聚合后填充。
	}
	if m.LastCheckedAt != nil {
		s := m.LastCheckedAt.UTC().Format(time.RFC3339)
		resp.LastCheckedAt = &s
	}
	return resp
}

func checkResultToResponse(r *service.CheckResult) channelMonitorCheckResultResponse {
	return channelMonitorCheckResultResponse{
		Model:         r.Model,
		Status:        r.Status,
		LatencyMs:     r.LatencyMs,
		PingLatencyMs: r.PingLatencyMs,
		Message:       r.Message,
		CheckedAt:     r.CheckedAt.UTC().Format(time.RFC3339),
		Quota:         r.Quota,
	}
}

func historyEntryToResponse(e *service.ChannelMonitorHistoryEntry) channelMonitorHistoryItemResponse {
	return channelMonitorHistoryItemResponse{
		ID:            e.ID,
		Model:         e.Model,
		Status:        e.Status,
		LatencyMs:     e.LatencyMs,
		PingLatencyMs: e.PingLatencyMs,
		Message:       e.Message,
		CheckedAt:     e.CheckedAt.UTC().Format(time.RFC3339),
		Quota:         e.Quota,
		Substituted:   e.Substituted,
		AnomalyID:     e.AnomalyID,
	}
}

// ParseChannelMonitorID 提取并校验路径参数 :id（admin 与 user handler 共享）。
// 校验失败时已写入 4xx 响应，调用方只需 return。
func ParseChannelMonitorID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.ErrorFrom(c, infraerrors.BadRequest("INVALID_MONITOR_ID", "invalid monitor id"))
		return 0, false
	}
	return id, true
}

// parseListEnabled 解析 enabled query 参数：true/false 转为 *bool，空或非法则返回 nil。
func parseListEnabled(raw string) *bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "true", "1", "yes":
		v := true
		return &v
	case "false", "0", "no":
		v := false
		return &v
	default:
		return nil
	}
}

// --- Handlers ---

// List GET /api/v1/admin/channel-monitors
func (h *ChannelMonitorHandler) List(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	if pageSize > monitorMaxPageSize {
		pageSize = monitorMaxPageSize
	}

	params := service.ChannelMonitorListParams{
		Page:     page,
		PageSize: pageSize,
		Provider: strings.TrimSpace(c.Query("provider")),
		Enabled:  parseListEnabled(c.Query("enabled")),
		Search:   strings.TrimSpace(c.Query("search")),
	}

	items, total, err := h.monitorService.List(c.Request.Context(), params)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	summaries := h.batchSummaryFor(c, items)
	out := make([]*channelMonitorResponse, 0, len(items))
	for _, m := range items {
		out = append(out, buildListItemResponse(m, summaries[m.ID]))
	}
	response.Paginated(c, out, total, page, pageSize)
}

// batchSummaryFor 批量聚合 latest + 7d 可用率，避免每行 2 次 SQL（消除 N+1）。
func (h *ChannelMonitorHandler) batchSummaryFor(c *gin.Context, items []*service.ChannelMonitor) map[int64]service.MonitorStatusSummary {
	ids := make([]int64, 0, len(items))
	primaryByID := make(map[int64]string, len(items))
	extrasByID := make(map[int64][]string, len(items))
	for _, m := range items {
		ids = append(ids, m.ID)
		primaryByID[m.ID] = m.PrimaryModel
		extrasByID[m.ID] = m.ExtraModels
	}
	return h.monitorService.BatchMonitorStatusSummary(c.Request.Context(), ids, primaryByID, extrasByID)
}

// buildListItemResponse 把 monitor + summary 装成 admin list 的响应行。
func buildListItemResponse(m *service.ChannelMonitor, summary service.MonitorStatusSummary) *channelMonitorResponse {
	resp := channelMonitorToResponse(m)
	resp.PrimaryStatus = summary.PrimaryStatus
	resp.PrimaryLatencyMs = summary.PrimaryLatencyMs
	resp.Availability7d = summary.Availability7d
	resp.LatestQuota = summary.LatestQuota
	resp.ExtraModelsStatus = make([]dto.ChannelMonitorExtraModelStatus, 0, len(summary.ExtraModels))
	for _, e := range summary.ExtraModels {
		resp.ExtraModelsStatus = append(resp.ExtraModelsStatus, dto.ChannelMonitorExtraModelStatus{
			Model:     e.Model,
			Status:    e.Status,
			LatencyMs: e.LatencyMs,
		})
	}
	return resp
}

// Get GET /api/v1/admin/channel-monitors/:id
func (h *ChannelMonitorHandler) Get(c *gin.Context) {
	id, ok := ParseChannelMonitorID(c)
	if !ok {
		return
	}
	m, err := h.monitorService.Get(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, channelMonitorToResponse(m))
}

// Create POST /api/v1/admin/channel-monitors
func (h *ChannelMonitorHandler) Create(c *gin.Context) {
	var req channelMonitorCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorFrom(c, infraerrors.BadRequest("VALIDATION_ERROR", err.Error()))
		return
	}

	subject, _ := middleware2.GetAuthSubjectFromContext(c)

	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	m, err := h.monitorService.Create(c.Request.Context(), service.ChannelMonitorCreateParams{
		Name:             req.Name,
		Provider:         req.Provider,
		APIMode:          req.APIMode,
		Endpoint:         req.Endpoint,
		APIKey:           req.APIKey,
		PrimaryModel:     req.PrimaryModel,
		ExtraModels:      req.ExtraModels,
		GroupName:        req.GroupName,
		Enabled:          enabled,
		IntervalSeconds:  req.IntervalSeconds,
		JitterSeconds:    req.JitterSeconds,
		CreatedBy:        subject.UserID,
		TemplateID:       req.TemplateID,
		ExtraHeaders:     req.ExtraHeaders,
		BodyOverrideMode: req.BodyOverrideMode,
		BodyOverride:     req.BodyOverride,
		CheckMode:        req.CheckMode,
		AccountID:        req.AccountID,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Created(c, channelMonitorToResponse(m))
}

// Duplicate POST /api/v1/admin/channel-monitors/:id/duplicate
func (h *ChannelMonitorHandler) Duplicate(c *gin.Context) {
	id, ok := ParseChannelMonitorID(c)
	if !ok {
		return
	}
	subject, _ := middleware2.GetAuthSubjectFromContext(c)
	actorScope := adminActorScope(c)

	result, err := executeAdminIdempotent(
		c,
		"admin.channel_monitors.duplicate",
		struct {
			MonitorID int64 `json:"monitor_id"`
		}{MonitorID: id},
		service.DefaultWriteIdempotencyTTL(),
		func(ctx context.Context) (any, error) {
			monitor, err := h.monitorService.Duplicate(
				ctx,
				id,
				subject.UserID,
				actorScope,
				c.GetHeader("Idempotency-Key"),
			)
			if err != nil {
				return nil, err
			}
			return channelMonitorToResponse(monitor), nil
		},
	)
	if err != nil {
		reason := infraerrors.Reason(err)
		if reason == infraerrors.Reason(service.ErrIdempotencyInProgress) || reason == infraerrors.Reason(service.ErrIdempotencyStoreUnavail) {
			recovered, recoverErr := h.monitorService.RecoverDuplicate(
				c.Request.Context(),
				id,
				actorScope,
				c.GetHeader("Idempotency-Key"),
			)
			if recoverErr != nil {
				slog.Warn("channel_monitor_duplicate_recovery_failed", "monitor_id", id, "actor_scope", actorScope, "reason", reason, "error", recoverErr)
			} else if recovered != nil {
				c.Header("X-Idempotency-Recovered", "true")
				response.Success(c, channelMonitorToResponse(recovered))
				return
			}
		}
		response.ErrorFrom(c, err)
		return
	}

	if result != nil && result.Replayed {
		c.Header("X-Idempotency-Replayed", "true")
	}
	response.Success(c, result.Data)
}

// Update PUT /api/v1/admin/channel-monitors/:id
func (h *ChannelMonitorHandler) Update(c *gin.Context) {
	id, ok := ParseChannelMonitorID(c)
	if !ok {
		return
	}
	var req channelMonitorUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorFrom(c, infraerrors.BadRequest("VALIDATION_ERROR", err.Error()))
		return
	}

	m, err := h.monitorService.Update(c.Request.Context(), id, service.ChannelMonitorUpdateParams{
		Name:             req.Name,
		Provider:         req.Provider,
		APIMode:          req.APIMode,
		Endpoint:         req.Endpoint,
		APIKey:           req.APIKey,
		PrimaryModel:     req.PrimaryModel,
		ExtraModels:      req.ExtraModels,
		GroupName:        req.GroupName,
		Enabled:          req.Enabled,
		IntervalSeconds:  req.IntervalSeconds,
		JitterSeconds:    req.JitterSeconds,
		TemplateID:       req.TemplateID,
		ClearTemplate:    req.ClearTemplate,
		ExtraHeaders:     req.ExtraHeaders,
		BodyOverrideMode: req.BodyOverrideMode,
		BodyOverride:     req.BodyOverride,
		CheckMode:        req.CheckMode,
		AccountID:        req.AccountID,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, channelMonitorToResponse(m))
}

// Delete DELETE /api/v1/admin/channel-monitors/:id
func (h *ChannelMonitorHandler) Delete(c *gin.Context) {
	id, ok := ParseChannelMonitorID(c)
	if !ok {
		return
	}
	if err := h.monitorService.Delete(c.Request.Context(), id); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, nil)
}

// Run POST /api/v1/admin/channel-monitors/:id/run
// 内部走探测重试周期（probe / quota_probe；quota 模式不重试）：
// 同步执行完整重试后返回最终结果，接口形态与响应结构与旧版一致；
// 失败渠道常见耗时约 1 分钟（退避等待 3+10+30s），前端 loading 时间会变长。
// 请求被网关 / 前端超时切断时本次周期作废（不落库、不写异常记录）。
func (h *ChannelMonitorHandler) Run(c *gin.Context) {
	id, ok := ParseChannelMonitorID(c)
	if !ok {
		return
	}
	results, err := h.monitorService.RunCheckWithRetry(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	out := make([]channelMonitorCheckResultResponse, 0, len(results))
	for _, r := range results {
		out = append(out, checkResultToResponse(r))
	}
	response.Success(c, gin.H{"results": out})
}

// History GET /api/v1/admin/channel-monitors/:id/history
func (h *ChannelMonitorHandler) History(c *gin.Context) {
	id, ok := ParseChannelMonitorID(c)
	if !ok {
		return
	}
	limit := parseHistoryLimit(c.Query("limit"))
	model := strings.TrimSpace(c.Query("model"))

	entries, err := h.monitorService.ListHistory(c.Request.Context(), id, model, limit)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	out := make([]channelMonitorHistoryItemResponse, 0, len(entries))
	for _, e := range entries {
		out = append(out, historyEntryToResponse(e))
	}
	response.Success(c, gin.H{"items": out})
}

// parseHistoryLimit 解析 history 接口的 limit query。
// 使用 service 包的统一上下限常量，避免在 handler 重复定义同名魔法值。
func parseHistoryLimit(raw string) int {
	if strings.TrimSpace(raw) == "" {
		return service.MonitorHistoryDefaultLimit
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v <= 0 {
		return service.MonitorHistoryDefaultLimit
	}
	if v > service.MonitorHistoryMaxLimit {
		return service.MonitorHistoryMaxLimit
	}
	return v
}

// --- 探测异常列表（channel-monitor-v1-probe-retry） ---

// probeAnomalyToResponse 把 service.ProbeAnomaly 转成 admin DTO。
func probeAnomalyToResponse(a *service.ProbeAnomaly) dto.ChannelMonitorProbeAnomaly {
	out := dto.ChannelMonitorProbeAnomaly{
		ID:               a.ID,
		MonitorID:        a.MonitorID,
		MonitorName:      a.MonitorName,
		Model:            a.Model,
		Trigger:          a.Trigger,
		TriggerStatus:    a.TriggerStatus,
		Resolution:       a.Resolution,
		Reason:           a.Reason,
		ConsecutiveSeq:   a.ConsecutiveSeq,
		AttemptCount:     a.AttemptCount,
		Attempts:         make([]dto.ChannelMonitorProbeAttempt, 0, len(a.Attempts)),
		WrittenHistoryID: a.WrittenHistoryID,
		CycleStartedAt:   a.CycleStartedAt,
		CycleFinishedAt:  a.CycleFinishedAt,
		CreatedAt:        a.CreatedAt,
	}
	for _, at := range a.Attempts {
		out.Attempts = append(out.Attempts, dto.ChannelMonitorProbeAttempt{
			Seq:           at.Seq,
			Status:        at.Status,
			LatencyMs:     at.LatencyMs,
			PingLatencyMs: at.PingLatencyMs,
			Message:       at.Message,
			CheckedAt:     at.CheckedAt,
		})
	}
	if a.ReferenceHistoryID != nil {
		v := *a.ReferenceHistoryID
		out.ReferenceHistoryID = &v
	}
	if a.ReferenceCheckedAt != nil {
		v := *a.ReferenceCheckedAt
		out.ReferenceCheckedAt = &v
	}
	return out
}

// parseProbeAnomalyParams 解析异常列表的公共 query 参数。
// 校验失败时已写入 4xx 响应，调用方只需 return。
func parseProbeAnomalyParams(c *gin.Context) (service.ProbeAnomalyListParams, bool) {
	params := service.ProbeAnomalyListParams{
		Model:      strings.TrimSpace(c.Query("model")),
		Resolution: strings.TrimSpace(c.Query("resolution")),
	}
	if params.Resolution != "" &&
		params.Resolution != service.ProbeAnomalyResolutionSubstituted &&
		params.Resolution != service.ProbeAnomalyResolutionRealWritten {
		response.ErrorFrom(c, infraerrors.BadRequest("INVALID_PROBE_ANOMALY_RESOLUTION",
			"resolution must be one of substituted/real_written"))
		return params, false
	}
	for name, field := range map[string]**time.Time{"from": &params.From, "to": &params.To} {
		raw := strings.TrimSpace(c.Query(name))
		if raw == "" {
			continue
		}
		t, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			response.ErrorFrom(c, infraerrors.BadRequest("INVALID_PROBE_ANOMALY_TIME",
				name+" must be an RFC3339 timestamp"))
			return params, false
		}
		*field = &t
	}
	params.Page, params.PageSize = response.ParsePagination(c)
	if params.PageSize > monitorMaxPageSize {
		params.PageSize = monitorMaxPageSize
	}
	return params, true
}

// ProbeAnomalies GET /api/v1/admin/channel-monitors/:id/probe-anomalies
// 单个监控的探测异常列表。
func (h *ChannelMonitorHandler) ProbeAnomalies(c *gin.Context) {
	id, ok := ParseChannelMonitorID(c)
	if !ok {
		return
	}
	params, ok := parseProbeAnomalyParams(c)
	if !ok {
		return
	}
	params.MonitorID = id

	items, total, err := h.monitorService.ListProbeAnomalies(c.Request.Context(), params)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	out := make([]dto.ChannelMonitorProbeAnomaly, 0, len(items))
	for _, a := range items {
		out = append(out, probeAnomalyToResponse(a))
	}
	response.Paginated(c, out, total, params.Page, params.PageSize)
}

// GlobalProbeAnomalies GET /api/v1/admin/channel-monitor-probe-anomalies
// 跨监控的全局异常列表（可按 monitor_id 过滤）。
func (h *ChannelMonitorHandler) GlobalProbeAnomalies(c *gin.Context) {
	params, ok := parseProbeAnomalyParams(c)
	if !ok {
		return
	}
	if raw := strings.TrimSpace(c.Query("monitor_id")); raw != "" {
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || id <= 0 {
			response.ErrorFrom(c, infraerrors.BadRequest("INVALID_MONITOR_ID", "invalid monitor id"))
			return
		}
		params.MonitorID = id
	}

	items, total, err := h.monitorService.ListProbeAnomalies(c.Request.Context(), params)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	out := make([]dto.ChannelMonitorProbeAnomaly, 0, len(items))
	for _, a := range items {
		out = append(out, probeAnomalyToResponse(a))
	}
	response.Paginated(c, out, total, params.Page, params.PageSize)
}
