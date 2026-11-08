package domain

import "time"

// 渠道监控 V1 探测重试的尝试明细类型（channel-monitor-v1-probe-retry 方案）。
//
// 每个异常周期（channel_monitor_probe_anomalies.attempts，JSONB 数组）记录
// 该模型在周期内的全部探测尝试，供管理员回看真实失败过程。
//
// 与 MonitorQuotaSnapshot 相同，类型放在 domain 包是因为 ent schema 需要引用
// 它做 field.JSON 序列化；service 不能被 ent import（会造成循环依赖）。

// MonitorProbeAttempt 单次探测尝试的明细。
type MonitorProbeAttempt struct {
	Seq           int       `json:"seq"` // 从 1 开始
	Status        string    `json:"status"`
	LatencyMs     *int      `json:"latency_ms"`
	PingLatencyMs *int      `json:"ping_latency_ms"`
	Message       string    `json:"message"`
	CheckedAt     time.Time `json:"checked_at"`
}
