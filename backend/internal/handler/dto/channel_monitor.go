package dto

import "time"

// ChannelMonitorExtraModelStatus 渠道监控附加模型最近一次状态。
// 同时被 admin handler（List 响应）与 user handler（List 响应）复用，
// 字段必须保持一致以保证前端拿到统一结构。
type ChannelMonitorExtraModelStatus struct {
	Model     string `json:"model"`
	Status    string `json:"status"`
	LatencyMs *int   `json:"latency_ms"`
}

// ChannelMonitorProbeAttempt 探测异常记录里单次尝试的明细
// （channel_monitor_probe_anomalies.attempts，JSONB 数组元素）。
type ChannelMonitorProbeAttempt struct {
	Seq           int       `json:"seq"`
	Status        string    `json:"status"`
	LatencyMs     *int      `json:"latency_ms"`
	PingLatencyMs *int      `json:"ping_latency_ms"`
	Message       string    `json:"message"`
	CheckedAt     time.Time `json:"checked_at"`
}

// ChannelMonitorProbeAnomaly 探测异常记录（管理员列表接口响应行）。
// 只读查询 DTO：写入路径见 service.ProbeAnomalyRow。
type ChannelMonitorProbeAnomaly struct {
	ID                 int64                        `json:"id"`
	MonitorID          int64                        `json:"monitor_id"`
	MonitorName        string                       `json:"monitor_name"`
	Model              string                       `json:"model"`
	Trigger            string                       `json:"trigger"`         // scheduled / manual
	TriggerStatus      string                       `json:"trigger_status"`  // 首次失败状态：error / failed
	Resolution         string                       `json:"resolution"`      // substituted / real_written
	Reason             string                       `json:"reason"`          // real_written 时：no_reference / too_many_consecutive
	ConsecutiveSeq     int                          `json:"consecutive_seq"` // 连续异常周期序号，从 1 开始
	AttemptCount       int                          `json:"attempt_count"`
	Attempts           []ChannelMonitorProbeAttempt `json:"attempts"`
	ReferenceHistoryID *int64                       `json:"reference_history_id"`
	ReferenceCheckedAt *time.Time                   `json:"reference_checked_at"`
	WrittenHistoryID   int64                        `json:"written_history_id"`
	CycleStartedAt     time.Time                    `json:"cycle_started_at"`
	CycleFinishedAt    time.Time                    `json:"cycle_finished_at"`
	CreatedAt          time.Time                    `json:"created_at"`
}
