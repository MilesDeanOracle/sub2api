package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// channel_monitor_probe_repo.go 实现 V1 探测重试（channel-monitor-v1-probe-retry）
// 需要的仓储方法：参考行 / 连续序号查询、周期事务落库、异常列表与清理。
// 全部走原生 SQL：
//   - 单行索引查询（(monitor_id, model, created_at) / 现有历史索引）保证走索引；
//   - 事务内 INSERT ... RETURNING 拿历史行自增 id 回填异常记录的 written_history_id；
//   - 列名 "trigger" 是 PG 保留字，所有 SQL 必须带双引号。

// probeAnomalySelectColumns 异常记录查询的共用列清单。
// %s 占位用于插入 monitor_name 列（仅列表查询 JOIN channel_monitors 时使用）。
const probeAnomalySelectColumns = `
    a.id, a.monitor_id, %s a.model, a."trigger", a.trigger_status, a.resolution, a.reason,
    a.consecutive_seq, a.attempt_count, a.attempts,
    a.reference_history_id, a.reference_checked_at, a.written_history_id,
    a.cycle_started_at, a.cycle_finished_at, a.created_at`

// scanProbeAnomalyRow 把一行异常记录扫为 service.ProbeAnomaly。
// withName 表示 monitor_id 之后、model 之前多一个 monitor_name 列。
func scanProbeAnomalyRow(row interface{ Scan(dest ...any) error }, withName bool) (*service.ProbeAnomaly, error) {
	out := &service.ProbeAnomaly{}
	var attempts []byte
	var refID sql.NullInt64
	var refCheckedAt sql.NullTime
	dest := []any{
		&out.ID, &out.MonitorID,
	}
	if withName {
		dest = append(dest, &out.MonitorName)
	}
	dest = append(dest,
		&out.Model, &out.Trigger, &out.TriggerStatus, &out.Resolution, &out.Reason,
		&out.ConsecutiveSeq, &out.AttemptCount, &attempts,
		&refID, &refCheckedAt, &out.WrittenHistoryID,
		&out.CycleStartedAt, &out.CycleFinishedAt, &out.CreatedAt,
	)
	if err := row.Scan(dest...); err != nil {
		return nil, err
	}
	if len(attempts) > 0 {
		_ = json.Unmarshal(attempts, &out.Attempts) // JSONB 由写入侧构造，解析失败保持 nil 不阻断
	}
	if refID.Valid {
		v := refID.Int64
		out.ReferenceHistoryID = &v
	}
	if refCheckedAt.Valid {
		v := refCheckedAt.Time
		out.ReferenceCheckedAt = &v
	}
	return out, nil
}

// FindOperationalReference 取 (monitor_id, model) 最新一条 operational 历史行
// 作为复制行的参考行。走现有 (monitor_id, model, checked_at) 索引；无符合行返回 (nil, nil)。
func (r *channelMonitorRepository) FindOperationalReference(ctx context.Context, monitorID int64, model string) (*service.ChannelMonitorHistoryRef, error) {
	const q = `
		SELECT id, latency_ms, ping_latency_ms, checked_at
		FROM channel_monitor_histories
		WHERE monitor_id = $1 AND model = $2 AND status = 'operational'
		ORDER BY checked_at DESC
		LIMIT 1
	`
	ref := &service.ChannelMonitorHistoryRef{}
	var latency, ping sql.NullInt64
	err := r.db.QueryRowContext(ctx, q, monitorID, model).
		Scan(&ref.ID, &latency, &ping, &ref.CheckedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find operational reference: %w", err)
	}
	assignNullInt(&ref.LatencyMs, latency)
	assignNullInt(&ref.PingLatencyMs, ping)
	return ref, nil
}

// FindLatestProbeAnomaly 取 (monitor_id, model) 最新一条异常记录；无记录返回 (nil, nil)。
// 走 idx_cm_probe_anomalies_monitor_model_created。
func (r *channelMonitorRepository) FindLatestProbeAnomaly(ctx context.Context, monitorID int64, model string) (*service.ProbeAnomaly, error) {
	q := fmt.Sprintf(`
		SELECT %s
		FROM channel_monitor_probe_anomalies a
		WHERE a.monitor_id = $1 AND a.model = $2
		ORDER BY a.created_at DESC, a.id DESC
		LIMIT 1`, fmt.Sprintf(probeAnomalySelectColumns, ""))

	var out *service.ProbeAnomaly
	rows, err := r.db.QueryContext(ctx, q, monitorID, model)
	if err != nil {
		return nil, fmt.Errorf("find latest probe anomaly: %w", err)
	}
	defer func() { _ = rows.Close() }()
	if rows.Next() {
		out, err = scanProbeAnomalyRow(rows, false)
		if err != nil {
			return nil, fmt.Errorf("scan latest probe anomaly: %w", err)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// FindLatestHistoryEntry 取 (monitor_id, model) 最新一条历史行（任意状态）；无行返回 (nil, nil)。
func (r *channelMonitorRepository) FindLatestHistoryEntry(ctx context.Context, monitorID int64, model string) (*service.ChannelMonitorHistoryEntry, error) {
	const q = `
		SELECT id, model, status, latency_ms, ping_latency_ms, message, checked_at
		FROM channel_monitor_histories
		WHERE monitor_id = $1 AND model = $2
		ORDER BY checked_at DESC, id DESC
		LIMIT 1
	`
	entry := &service.ChannelMonitorHistoryEntry{}
	var latency, ping sql.NullInt64
	err := r.db.QueryRowContext(ctx, q, monitorID, model).
		Scan(&entry.ID, &entry.Model, &entry.Status, &latency, &ping, &entry.Message, &entry.CheckedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find latest history entry: %w", err)
	}
	assignNullInt(&entry.LatencyMs, latency)
	assignNullInt(&entry.PingLatencyMs, ping)
	return entry, nil
}

// PersistProbeCycle 在同一事务内完成周期落库（§4.3）：
//  1. 逐行插入历史行（每模型一行），拿回各自自增 id；
//  2. 插入异常记录，written_history_id 回填对应历史行 id；
//  3. MarkChecked(monitor_id, checkedAt)。
//
// 任一步失败整体回滚。无历史行的退化周期只更新 last_checked_at。
func (r *channelMonitorRepository) PersistProbeCycle(ctx context.Context, persist *service.ProbeCyclePersist) ([]int64, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("persist probe cycle: begin tx: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	ids := make([]int64, 0, len(persist.History))
	for _, row := range persist.History {
		var quotaVal any
		if row.Quota != nil {
			payload, mErr := json.Marshal(row.Quota)
			if mErr != nil {
				return nil, fmt.Errorf("persist probe cycle: marshal quota: %w", mErr)
			}
			quotaVal = payload
		}
		var id int64
		err := tx.QueryRowContext(ctx, `
			INSERT INTO channel_monitor_histories
			    (monitor_id, model, status, latency_ms, ping_latency_ms, message, checked_at, quota)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			RETURNING id`,
			row.MonitorID, row.Model, row.Status, row.LatencyMs, row.PingLatencyMs,
			row.Message, row.CheckedAt, quotaVal).Scan(&id)
		if err != nil {
			return nil, fmt.Errorf("persist probe cycle: insert history (model=%s): %w", row.Model, err)
		}
		ids = append(ids, id)
	}

	for _, a := range persist.Anomalies {
		if a.HistoryRowIndex < 0 || a.HistoryRowIndex >= len(ids) {
			return nil, fmt.Errorf("persist probe cycle: anomaly history row index %d out of range (model=%s)", a.HistoryRowIndex, a.Model)
		}
		attempts, mErr := json.Marshal(a.Attempts)
		if mErr != nil {
			return nil, fmt.Errorf("persist probe cycle: marshal attempts (model=%s): %w", a.Model, mErr)
		}
		_, err := tx.ExecContext(ctx, `
			INSERT INTO channel_monitor_probe_anomalies
			    (monitor_id, model, "trigger", trigger_status, resolution, reason,
			     consecutive_seq, attempt_count, attempts,
			     reference_history_id, reference_checked_at, written_history_id,
			     cycle_started_at, cycle_finished_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)`,
			a.MonitorID, a.Model, a.Trigger, a.TriggerStatus, a.Resolution, a.Reason,
			a.ConsecutiveSeq, len(a.Attempts), attempts,
			a.ReferenceHistoryID, a.ReferenceCheckedAt, ids[a.HistoryRowIndex],
			a.CycleStartedAt, a.CycleFinishedAt)
		if err != nil {
			return nil, fmt.Errorf("persist probe cycle: insert anomaly (model=%s): %w", a.Model, err)
		}
	}

	if _, err := tx.ExecContext(ctx,
		`UPDATE channel_monitors SET last_checked_at = $2, updated_at = NOW() WHERE id = $1`,
		persist.MonitorID, persist.CheckedAt); err != nil {
		return nil, fmt.Errorf("persist probe cycle: mark checked: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("persist probe cycle: commit: %w", err)
	}
	committed = true
	return ids, nil
}

// ListProbeAnomalies 分页查询异常记录（JOIN 监控名），按 created_at DESC。
func (r *channelMonitorRepository) ListProbeAnomalies(ctx context.Context, params service.ProbeAnomalyListParams) ([]*service.ProbeAnomaly, int64, error) {
	where := []string{"1=1"}
	args := []any{}
	appendFilter := func(clause string, value any) {
		args = append(args, value)
		where = append(where, fmt.Sprintf(clause, len(args)))
	}
	if params.MonitorID > 0 {
		appendFilter("a.monitor_id = $%d", params.MonitorID)
	}
	if m := strings.TrimSpace(params.Model); m != "" {
		appendFilter("a.model = $%d", m)
	}
	if res := strings.TrimSpace(params.Resolution); res != "" {
		appendFilter("a.resolution = $%d", res)
	}
	if params.From != nil {
		appendFilter("a.created_at >= $%d", *params.From)
	}
	if params.To != nil {
		appendFilter("a.created_at < $%d", *params.To)
	}
	whereSQL := strings.Join(where, " AND ")

	var total int64
	if err := r.db.QueryRowContext(ctx,
		fmt.Sprintf(`SELECT COUNT(*) FROM channel_monitor_probe_anomalies a WHERE %s`, whereSQL),
		args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count probe anomalies: %w", err)
	}
	if total == 0 {
		return []*service.ProbeAnomaly{}, 0, nil
	}

	page := params.Page
	if page < 1 {
		page = 1
	}
	pageSize := params.PageSize
	if pageSize < 1 || pageSize > 200 {
		pageSize = 20
	}
	args = append(args, pageSize, (page-1)*pageSize)
	q := fmt.Sprintf(`
		SELECT %s
		FROM channel_monitor_probe_anomalies a
		JOIN channel_monitors m ON m.id = a.monitor_id
		WHERE %s
		ORDER BY a.created_at DESC, a.id DESC
		LIMIT $%d OFFSET $%d`,
		fmt.Sprintf(probeAnomalySelectColumns, "m.name AS monitor_name,"),
		whereSQL, len(args)-1, len(args))

	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list probe anomalies: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := make([]*service.ProbeAnomaly, 0)
	for rows.Next() {
		item, sErr := scanProbeAnomalyRow(rows, true)
		if sErr != nil {
			return nil, 0, fmt.Errorf("scan probe anomaly row: %w", sErr)
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

// DeleteProbeAnomaliesBefore 物理删 created_at < before 的异常记录。
// 异常记录只在失败策略落空时写入，量级远小于历史表，无需分批。
func (r *channelMonitorRepository) DeleteProbeAnomaliesBefore(ctx context.Context, before time.Time) (int64, error) {
	res, err := r.db.ExecContext(ctx,
		`DELETE FROM channel_monitor_probe_anomalies WHERE created_at < $1`, before)
	if err != nil {
		return 0, fmt.Errorf("delete probe anomalies before %s: %w", before.Format(time.RFC3339), err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("delete probe anomalies rows affected: %w", err)
	}
	return n, nil
}

// 编译期确认本文件补齐了扩展后的接口方法。
var _ service.ChannelMonitorRepository = (*channelMonitorRepository)(nil)
