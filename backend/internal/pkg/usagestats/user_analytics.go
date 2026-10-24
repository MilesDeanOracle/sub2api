package usagestats

// UserAnalyticsFilters — 预留：当前用户分析接口仅按 user_id 全量聚合。

// UserModelUsageStat 用户维度按「分组 × 模型」聚合的用量统计。
// P95 指标基于 usage_logs 逐请求原始值计算（PostgreSQL percentile_cont），
// 样本不足或全部为 NULL 时为 nil。
type UserModelUsageStat struct {
	GroupID       int64    `json:"group_id"`
	Model         string   `json:"model"`
	Requests      int64    `json:"requests"`
	TotalTokens   int64    `json:"total_tokens"`
	ActualCost    float64  `json:"actual_cost"`
	AccountCost   float64  `json:"account_cost"`
	TtftP95Ms     *float64 `json:"ttft_p95_ms"`
	DurationP95Ms *float64 `json:"duration_p95_ms"`
	OutputTpsP95  *float64 `json:"output_tps_p95"`
}

// UserGroupUsageStat 用户维度按「分组」聚合的用量统计（模型行的上层汇总）。
type UserGroupUsageStat struct {
	GroupID       int64    `json:"group_id"`
	Requests      int64    `json:"requests"`
	TotalTokens   int64    `json:"total_tokens"`
	ActualCost    float64  `json:"actual_cost"`
	AccountCost   float64  `json:"account_cost"`
	TtftP95Ms     *float64 `json:"ttft_p95_ms"`
	DurationP95Ms *float64 `json:"duration_p95_ms"`
	OutputTpsP95  *float64 `json:"output_tps_p95"`
}

// UserLifetimeUsage 用户全生命周期消费汇总（轻量 SUM 查询，无百分位）。
type UserLifetimeUsage struct {
	TotalRequests    int64   `json:"total_requests"`
	TotalTokens      int64   `json:"total_tokens"`
	TotalActualCost  float64 `json:"total_actual_cost"`
	TotalAccountCost float64 `json:"total_account_cost"`
}
