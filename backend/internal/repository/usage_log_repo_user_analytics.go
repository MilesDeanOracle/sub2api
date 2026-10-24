package repository

import (
	"context"

	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
)

// userAnalyticsSelect 用户分析接口共用的聚合列：
// 请求数 / 请求 Token 数（输入+输出）/ 实际金额（actual_cost）/ 成本（total_cost）/
// 首字 P95 / 总耗时 P95 / 输出 TPS P95。
// 输出 TPS 按单请求 output_tokens / 生成耗时（duration-first_token）计算，
// 仅统计流式且有有效首字耗时的请求。
const userAnalyticsColumns = `
	COUNT(*) AS requests,
	COALESCE(SUM(ul.input_tokens + ul.output_tokens), 0) AS total_tokens,
	COALESCE(SUM(ul.actual_cost), 0) AS actual_cost,
	COALESCE(SUM(ul.account_cost), 0) AS account_cost,
	percentile_cont(0.95) WITHIN GROUP (ORDER BY ul.first_token_ms)
		FILTER (WHERE ul.first_token_ms IS NOT NULL AND ul.first_token_ms > 0) AS ttft_p95_ms,
	percentile_cont(0.95) WITHIN GROUP (ORDER BY ul.duration_ms)
		FILTER (WHERE ul.duration_ms IS NOT NULL AND ul.duration_ms > 0) AS duration_p95_ms,
	percentile_cont(0.95) WITHIN GROUP (
		ORDER BY ul.output_tokens * 1000.0 / (ul.duration_ms - ul.first_token_ms)
	) FILTER (
		WHERE ul.duration_ms IS NOT NULL
			AND ul.first_token_ms IS NOT NULL
			AND ul.first_token_ms > 0
			AND ul.duration_ms > ul.first_token_ms
			AND ul.output_tokens > 0
	) AS output_tps_p95`

// GetUserModelUsageStats 返回用户按「分组 × 模型」的用量统计。
// group_id 为 NULL 的记录归入 0（前端显示为未分组）。
func (r *usageLogRepository) GetUserModelUsageStats(ctx context.Context, userID int64) ([]usagestats.UserModelUsageStat, error) {
	query := `
		SELECT
			COALESCE(ul.group_id, 0) AS group_id,
			ul.model,
			` + userAnalyticsColumns + `
		FROM usage_logs ul
		WHERE ul.user_id = $1
		GROUP BY COALESCE(ul.group_id, 0), ul.model
		ORDER BY actual_cost DESC
	`

	rows, err := r.sql.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := make([]usagestats.UserModelUsageStat, 0)
	for rows.Next() {
		var row usagestats.UserModelUsageStat
		if err := rows.Scan(
			&row.GroupID,
			&row.Model,
			&row.Requests,
			&row.TotalTokens,
			&row.ActualCost,
			&row.AccountCost,
			&row.TtftP95Ms,
			&row.DurationP95Ms,
			&row.OutputTpsP95,
		); err != nil {
			return nil, err
		}
		results = append(results, row)
	}
	return results, rows.Err()
}

// GetUserGroupUsageStats 返回用户按「分组」的用量统计（分组级 P95 为独立聚合，非模型行折算）。
func (r *usageLogRepository) GetUserGroupUsageStats(ctx context.Context, userID int64) ([]usagestats.UserGroupUsageStat, error) {
	query := `
		SELECT
			COALESCE(ul.group_id, 0) AS group_id,
			` + userAnalyticsColumns + `
		FROM usage_logs ul
		WHERE ul.user_id = $1
		GROUP BY COALESCE(ul.group_id, 0)
		ORDER BY actual_cost DESC
	`

	rows, err := r.sql.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := make([]usagestats.UserGroupUsageStat, 0)
	for rows.Next() {
		var row usagestats.UserGroupUsageStat
		if err := rows.Scan(
			&row.GroupID,
			&row.Requests,
			&row.TotalTokens,
			&row.ActualCost,
			&row.AccountCost,
			&row.TtftP95Ms,
			&row.DurationP95Ms,
			&row.OutputTpsP95,
		); err != nil {
			return nil, err
		}
		results = append(results, row)
	}
	return results, rows.Err()
}

// GetUserLifetimeUsage 返回用户全生命周期消费汇总（轻量 SUM，供概览卡片使用）。
func (r *usageLogRepository) GetUserLifetimeUsage(ctx context.Context, userID int64) (*usagestats.UserLifetimeUsage, error) {
	query := `
		SELECT
			COUNT(*),
			COALESCE(SUM(input_tokens + output_tokens), 0),
			COALESCE(SUM(actual_cost), 0),
			COALESCE(SUM(account_cost), 0)
		FROM usage_logs
		WHERE user_id = $1
	`

	var out usagestats.UserLifetimeUsage
	if err := r.db.QueryRowContext(ctx, query, userID).Scan(
		&out.TotalRequests,
		&out.TotalTokens,
		&out.TotalActualCost,
		&out.TotalAccountCost,
	); err != nil {
		return nil, err
	}
	return &out, nil
}
