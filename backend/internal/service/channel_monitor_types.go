package service

import (
	"time"

	"github.com/Wei-Shaw/sub2api/internal/domain"
)

// MonitorBodyOverrideMode 自定义请求体处理模式。
//
//   - off     使用 adapter 默认 body（忽略 BodyOverride）
//   - merge   adapter 默认 body 与 BodyOverride 浅合并（用户优先；
//     model/messages/contents 等关键字段在 checker 黑名单内会被静默丢弃）
//   - replace 完全用 BodyOverride 作为 body；跳过 challenge 校验，
//     改成 HTTP 2xx + 响应非空即视为可用（用户负责构造 body）
const (
	MonitorBodyOverrideModeOff     = "off"
	MonitorBodyOverrideModeMerge   = "merge"
	MonitorBodyOverrideModeReplace = "replace"
)

// MonitorAPIMode 描述 OpenAI provider 的请求协议。
//
//   - chat_completions  OpenAI-compatible Chat Completions: /v1/chat/completions + messages
//   - responses         OpenAI Responses API: /v1/responses + instructions/input
//
// 非 OpenAI provider 固定使用 chat_completions 作为占位默认值，避免为每个 provider 单独扩表。
const (
	MonitorAPIModeChatCompletions = "chat_completions"
	MonitorAPIModeResponses       = "responses"
)

// ChannelMonitor 渠道监控配置（service 层模型，不直接暴露 ent 类型）。
type ChannelMonitor struct {
	ID              int64
	Name            string
	Provider        string
	APIMode         string
	Endpoint        string
	APIKey          string // 解密后的明文 API Key（仅在 service 内部使用，handler 层不应直接序列化返回）
	PrimaryModel    string
	ExtraModels     []string
	GroupName       string
	Enabled         bool
	IntervalSeconds int
	JitterSeconds   int // 每次调度 ± [0, jitter] 的随机偏移（秒），0 = 固定间隔
	LastCheckedAt   *time.Time
	CreatedBy       int64
	CreatedAt       time.Time
	UpdatedAt       time.Time

	// 配额模式（check_mode = quota / quota_probe）：
	// 关联已有账号复用账号侧用量服务，Endpoint/APIKey 可为空（quota 模式）。
	CheckMode string // probe（默认）/ quota / quota_probe；空串按 probe 处理
	AccountID *int64 // 关联账号 ID；账号删除后被 DB 置空（监控保留并报「账号未关联」）

	// 请求自定义快照（来自模板拷贝 or 用户手填，运行时直接读取）
	TemplateID       *int64            // 仅用于 UI 分组 + 一键应用，运行时不用
	ExtraHeaders     map[string]string // 与 adapter 默认 headers 合并，用户优先
	BodyOverrideMode string            // off / merge / replace
	BodyOverride     map[string]any    // 仅 mode != off 时使用

	// DuplicateOperationID is internal persistence metadata used to recover an
	// already committed duplicate after an ambiguous idempotency-store failure.
	// Repository implementations must keep it out of ExtraHeaders so it can
	// never be serialized to clients or forwarded to an upstream provider.
	DuplicateOperationID string

	// APIKeyDecryptFailed 表示 APIKey 字段无法解密（密钥不一致或损坏）。
	// 此时 APIKey 为空字符串，runner / RunCheck 必须跳过该监控并提示重填。
	APIKeyDecryptFailed bool
}

// ChannelMonitorListParams 列表查询过滤参数。
type ChannelMonitorListParams struct {
	Page     int
	PageSize int
	Provider string
	Enabled  *bool
	Search   string
}

// ChannelMonitorCreateParams 创建参数。
type ChannelMonitorCreateParams struct {
	Name             string
	Provider         string
	APIMode          string
	Endpoint         string
	APIKey           string
	PrimaryModel     string
	ExtraModels      []string
	GroupName        string
	Enabled          bool
	IntervalSeconds  int
	JitterSeconds    int
	CreatedBy        int64
	TemplateID       *int64
	ExtraHeaders     map[string]string
	BodyOverrideMode string
	BodyOverride     map[string]any

	// 配额模式：CheckMode 空串默认 probe；quota/quota_probe 必须关联账号。
	CheckMode string
	AccountID *int64
}

// ChannelMonitorUpdateParams 更新参数（指针字段表示"未提供则不更新"）。
type ChannelMonitorUpdateParams struct {
	Name            *string
	Provider        *string
	APIMode         *string
	Endpoint        *string
	APIKey          *string // 空字符串表示不修改；非空字符串覆盖
	PrimaryModel    *string
	ExtraModels     *[]string
	GroupName       *string
	Enabled         *bool
	IntervalSeconds *int
	JitterSeconds   *int
	// 自定义快照字段：指针为 nil 表示不更新，非 nil 覆盖
	// TemplateID *(*int64)：用 ** 表达三态：nil=不更新；&nil=清空；&&id=设为 id。
	// 简化处理：用 ClearTemplate 显式标志 + TemplateID（普通指针）
	TemplateID       *int64
	ClearTemplate    bool // true 时无视 TemplateID，把监控的 template_id 置空
	ExtraHeaders     *map[string]string
	BodyOverrideMode *string
	BodyOverride     *map[string]any

	// 配额模式：CheckMode nil = 不更新；AccountID nil = 不更新，
	// 指向 0 = 清空关联（退回 probe 模式时由 CheckMode 分支兜底）。
	CheckMode *string
	AccountID *int64
}

// CheckResult 单个模型一次检测的结果。
type CheckResult struct {
	Model         string
	Status        string // operational / degraded / failed / error
	LatencyMs     *int
	PingLatencyMs *int
	Message       string
	CheckedAt     time.Time
	// Quota 配额模式附带快照（quota 模式唯一数据；quota_probe 挂在主模型行）。
	Quota *domain.MonitorQuotaSnapshot
}

// ---- V1 探测重试周期（channel-monitor-v1-probe-retry） ----

// 探测周期的触发方式（ProbeCycle.Trigger / ProbeAnomaly.Trigger）。
const (
	ProbeTriggerScheduled = "scheduled" // 调度器周期触发
	ProbeTriggerManual    = "manual"    // 管理员「立即检测」
)

// 探测异常记录的落库结论（ProbeAnomaly.Resolution）。
const (
	// ProbeAnomalyResolutionSubstituted 重试后仍失败但写入了「上一次正常结果」的复制行。
	ProbeAnomalyResolutionSubstituted = "substituted"
	// ProbeAnomalyResolutionRealWritten 重试后仍失败且写入了真实失败结果。
	ProbeAnomalyResolutionRealWritten = "real_written"
)

// real_written 的具体原因（ProbeAnomaly.Reason；substituted 时为空串）。
const (
	// ProbeAnomalyReasonNoReference 找不到可复制的参考行（新建监控、从未成功过）。
	ProbeAnomalyReasonNoReference = "no_reference"
	// ProbeAnomalyReasonTooManyConsecutive 连续异常周期数超过复制上限。
	ProbeAnomalyReasonTooManyConsecutive = "too_many_consecutive"
)

// ChannelMonitorHistoryRef 复制行参考行的简明信息（不含 message 等大字段）。
type ChannelMonitorHistoryRef struct {
	ID            int64
	LatencyMs     *int
	PingLatencyMs *int
	CheckedAt     time.Time
}

// ProbeAnomalyRow 探测异常记录入库行（service 向 repository 提交）。
// HistoryRowIndex 指向同事务内 FinalizePersist.History 的下标，
// 事务内插入历史行拿到自增 id 后回填 WrittenHistoryID。
type ProbeAnomalyRow struct {
	MonitorID          int64
	Model              string
	Trigger            string // scheduled / manual
	TriggerStatus      string // 首次失败状态：error / failed
	Resolution         string // substituted / real_written
	Reason             string // real_written 时：no_reference / too_many_consecutive
	ConsecutiveSeq     int
	Attempts           []domain.MonitorProbeAttempt
	ReferenceHistoryID *int64
	ReferenceCheckedAt *time.Time
	HistoryRowIndex    int
	CycleStartedAt     time.Time
	CycleFinishedAt    time.Time
}

// ProbeCyclePersist 一次探测周期的落库集合：历史行（每模型一行）+ 异常记录
// （仅失败策略落空的模型）+ last_checked_at。repository 在同一事务内写入。
type ProbeCyclePersist struct {
	MonitorID int64
	CheckedAt time.Time
	History   []*ChannelMonitorHistoryRow
	Anomalies []*ProbeAnomalyRow
}

// ProbeAnomaly 探测异常记录查询返回行。
type ProbeAnomaly struct {
	ID                 int64
	MonitorID          int64
	MonitorName        string // 列表查询 JOIN channel_monitors 填充；单条查询可为空
	Model              string
	Trigger            string
	TriggerStatus      string
	Resolution         string
	Reason             string
	ConsecutiveSeq     int
	AttemptCount       int
	Attempts           []domain.MonitorProbeAttempt
	ReferenceHistoryID *int64
	ReferenceCheckedAt *time.Time
	WrittenHistoryID   int64
	CycleStartedAt     time.Time
	CycleFinishedAt    time.Time
	CreatedAt          time.Time
}

// ProbeAnomalyListParams 异常列表查询过滤参数（全局列表专用 MonitorID；单监控接口固定）。
type ProbeAnomalyListParams struct {
	MonitorID  int64
	Model      string
	Resolution string
	From       *time.Time
	To         *time.Time
	Page       int
	PageSize   int
}

// UserMonitorView 用户只读视图：监控概览（含主模型最近状态 + 7d 可用率 + 附加模型最近状态）。
type UserMonitorView struct {
	ID                   int64
	Name                 string
	Provider             string
	GroupName            string
	PrimaryModel         string
	PrimaryStatus        string
	PrimaryLatencyMs     *int
	PrimaryPingLatencyMs *int    // 主模型最近一次 ping 延迟
	Availability7d       float64 // 0-100
	ExtraModels          []ExtraModelStatus
	Timeline             []UserMonitorTimelinePoint // 主模型最近 N 个历史点（按 checked_at DESC，最新在前）
	// LatestQuota 主模型最近一次配额快照；channel_monitor_show_quota=false
	// 时由 handler 服务端剥离。
	LatestQuota *domain.MonitorQuotaSnapshot
}

// UserMonitorTimelinePoint 用户视图 timeline 单点数据（去除 message 以减小响应体）。
type UserMonitorTimelinePoint struct {
	Status        string    `json:"status"`
	LatencyMs     *int      `json:"latency_ms"`
	PingLatencyMs *int      `json:"ping_latency_ms"`
	CheckedAt     time.Time `json:"checked_at"`
}

// ExtraModelStatus 附加模型最近一次状态。
type ExtraModelStatus struct {
	Model     string
	Status    string
	LatencyMs *int
}

// UserMonitorDetail 用户只读视图：监控详情（含全部模型 7d/15d/30d 可用率与平均延迟）。
type UserMonitorDetail struct {
	ID        int64
	Name      string
	Provider  string
	GroupName string
	Models    []ModelDetail
}

// ModelDetail 单个模型的可用率/延迟统计。
type ModelDetail struct {
	Model           string
	LatestStatus    string
	LatestLatencyMs *int
	Availability7d  float64 // 0-100
	Availability15d float64
	Availability30d float64
	AvgLatency7dMs  *int
}

// ChannelMonitorHistoryRow 历史记录入库行（service 层向 repository 提交的数据）。
type ChannelMonitorHistoryRow struct {
	MonitorID     int64
	Model         string
	Status        string
	LatencyMs     *int
	PingLatencyMs *int
	Message       string
	CheckedAt     time.Time
	Quota         *domain.MonitorQuotaSnapshot
}

// ChannelMonitorHistoryEntry 历史记录查询返回行（含 ent 主键 ID）。
type ChannelMonitorHistoryEntry struct {
	ID            int64
	Model         string
	Status        string
	LatencyMs     *int
	PingLatencyMs *int
	Message       string
	CheckedAt     time.Time
	Quota         *domain.MonitorQuotaSnapshot
	// Substituted / AnomalyID：管理员历史接口 LEFT JOIN
	// channel_monitor_probe_anomalies 得出的计算字段，标记该行是否为
	// 「上一次正常结果」的复制行及关联的异常记录；用户侧接口不填充。
	Substituted bool
	AnomalyID   *int64
}

// ChannelMonitorLatest 最近一次检测的简明信息（用于 UserMonitorView 聚合）。
type ChannelMonitorLatest struct {
	Model         string
	Status        string
	LatencyMs     *int
	PingLatencyMs *int
	CheckedAt     time.Time
	Quota         *domain.MonitorQuotaSnapshot
}

// ChannelMonitorAvailability 单个模型在某窗口内的可用率与平均延迟（用于 UserMonitorDetail 聚合）。
type ChannelMonitorAvailability struct {
	Model             string
	WindowDays        int
	TotalChecks       int
	OperationalChecks int // operational + degraded 视为可用
	AvailabilityPct   float64
	AvgLatencyMs      *int
}

// MonitorStatusSummary 监控状态聚合（admin list 用，单次 repo 查询消除前端 N+1）。
// PrimaryStatus / PrimaryLatencyMs 描述主模型最近状态；Availability7d 是主模型 7 天可用率；
// ExtraModels 描述附加模型最近状态（用于 hover 展示）。
type MonitorStatusSummary struct {
	PrimaryStatus    string // 空字符串表示无历史
	PrimaryLatencyMs *int
	Availability7d   float64 // 0-100，无历史时为 0
	ExtraModels      []ExtraModelStatus
	LatestQuota      *domain.MonitorQuotaSnapshot // 主模型最近配额快照（配额模式）
}
