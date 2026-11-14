package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
)

type operationsRepository struct{ db *sql.DB }

func NewOperationsRepository(db *sql.DB) service.OperationsRepository {
	return &operationsRepository{db: db}
}

type operationsScanner interface{ Scan(...any) error }

func scanOperationsTask(row operationsScanner) (*service.OperationsTask, error) {
	var raw []byte
	if err := row.Scan(&raw); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, service.ErrOperationsNotFound
		}
		return nil, err
	}
	var t service.OperationsTask
	err := json.Unmarshal(raw, &t)
	return &t, err
}

const operationsResultColumns = `r.payload, r.id,r.task_id,r.run_id,r.account_id,r.status,r.group_snapshots,r.pool_slot,r.worker_token,r.heartbeat_at,r.execution_deadline_at,r.storage_binding_encrypted,r.object_key,r.version_id,r.queued_at,r.expires_at,r.cleanup_status,r.cleanup_attempts,r.cleanup_error,r.created_at,r.updated_at`

func scanOperationsResult(row operationsScanner) (*service.OperationsResult, error) {
	var raw, groups []byte
	var r service.OperationsResult
	if err := row.Scan(&raw, &r.ID, &r.TaskID, &r.RunID, &r.AccountID, &r.Status, &groups, &r.PoolSlot, &r.WorkerToken, &r.HeartbeatAt, &r.DeadlineAt, &r.StorageBinding, &r.ObjectKey, &r.VersionID, &r.QueuedAt, &r.ExpiresAt, &r.CleanupStatus, &r.CleanupAttempts, &r.CleanupError, &r.CreatedAt, &r.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, service.ErrOperationsNotFound
		}
		return nil, err
	}
	// Indexed columns are authoritative; payload holds snapshots and metrics only.
	var p service.OperationsResult
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, err
	}
	p.ID = r.ID
	p.TaskID = r.TaskID
	p.RunID = r.RunID
	p.AccountID = r.AccountID
	p.Status = r.Status
	p.PoolSlot = r.PoolSlot
	p.WorkerToken = r.WorkerToken
	p.HeartbeatAt = r.HeartbeatAt
	p.DeadlineAt = r.DeadlineAt
	p.StorageBinding = r.StorageBinding
	p.ObjectKey = r.ObjectKey
	p.VersionID = r.VersionID
	p.QueuedAt = r.QueuedAt
	p.ExpiresAt = r.ExpiresAt
	p.CleanupStatus = r.CleanupStatus
	p.CleanupAttempts = r.CleanupAttempts
	p.CleanupError = r.CleanupError
	p.CreatedAt = r.CreatedAt
	p.UpdatedAt = r.UpdatedAt
	if err := json.Unmarshal(groups, &p.Groups); err != nil {
		return nil, err
	}
	return &p, nil
}
func collectOperationsResults(rows *sql.Rows) ([]*service.OperationsResult, error) {
	defer rows.Close()
	out := []*service.OperationsResult{}
	for rows.Next() {
		r, e := scanOperationsResult(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
func (r *operationsRepository) ListTasks(ctx context.Context) ([]*service.OperationsTask, error) {
	rows, e := r.db.QueryContext(ctx, `SELECT to_jsonb(t) || jsonb_build_object('progress',COALESCE((SELECT jsonb_object_agg(s.status,s.n) FROM (SELECT status,count(*) n FROM operations_pelican_results WHERE run_id=(SELECT run_id FROM operations_pelican_results WHERE task_id=t.id ORDER BY id DESC LIMIT 1) GROUP BY status) s),'{}'::jsonb)) FROM operations_pelican_tasks t WHERE deleted_at IS NULL ORDER BY id DESC`)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []*service.OperationsTask{}
	for rows.Next() {
		t, e := scanOperationsTask(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, t)
	}
	return out, rows.Err()
}
func (r *operationsRepository) GetTask(ctx context.Context, id int64) (*service.OperationsTask, error) {
	return scanOperationsTask(r.db.QueryRowContext(ctx, `SELECT to_jsonb(t) FROM operations_pelican_tasks t WHERE id=$1 AND deleted_at IS NULL`, id))
}
func operationsInsertRound(ctx context.Context, tx *sql.Tx, t *service.OperationsTask, items []*service.OperationsResult, kind, key string, actor int64) (*service.OperationsRound, error) {
	var runID string
	e := tx.QueryRowContext(ctx, `SELECT run_id FROM operations_pelican_results WHERE task_id=$1 AND (trigger_key=$2 OR (payload->'trigger_aliases') ? $2) LIMIT 1`, t.ID, key).Scan(&runID)
	if e == nil {
		return &service.OperationsRound{RunID: runID, Existing: true}, nil
	}
	if !errors.Is(e, sql.ErrNoRows) {
		return nil, e
	}
	e = tx.QueryRowContext(ctx, `SELECT run_id FROM operations_pelican_results WHERE task_id=$1 AND status IN ('queued','running','uploading') LIMIT 1`, t.ID).Scan(&runID)
	if e == nil {
		// Bind a coalesced manual request to the existing round too. Otherwise
		// retrying that request after the round completes could generate again.
		_, e = tx.ExecContext(ctx, `UPDATE operations_pelican_results SET payload=jsonb_set(payload,'{trigger_aliases}',COALESCE(payload->'trigger_aliases','[]'::jsonb)||jsonb_build_array($2::text)) WHERE id=(SELECT id FROM operations_pelican_results WHERE run_id=$1 ORDER BY id LIMIT 1)`, runID, key)
		if e != nil {
			return nil, e
		}
		return &service.OperationsRound{RunID: runID, Existing: true}, nil
	}
	if !errors.Is(e, sql.ErrNoRows) {
		return nil, e
	}
	runID = uuid.NewString()
	now := time.Now().UTC()
	for _, v := range items {
		v.TaskID = t.ID
		v.TaskName = t.Name
		v.RunID = runID
		v.TriggerType = kind
		v.TriggeredBy = actor
		v.ConfigSnapshot = *t
		v.ConfigSnapshot.Progress = nil
		v.Status = "queued"
		v.QueuedAt = now
		v.CreatedAt = now
		v.ExpiresAt = now.Add(7 * 24 * time.Hour)
		body, err := json.Marshal(v)
		if err != nil {
			return nil, err
		}
		groups, err := json.Marshal(v.Groups)
		if err != nil {
			return nil, err
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO operations_pelican_results(task_id,run_id,account_id,trigger_key,status,group_snapshots,payload,expires_at,queued_at,storage_binding_encrypted) VALUES($1,$2,$3,$4,'queued',$5,$6,$7,$8,$9)`, t.ID, runID, v.AccountID, key, string(groups), string(body), v.ExpiresAt, now, v.StorageBinding); err != nil {
			return nil, err
		}
	}
	if _, e = tx.ExecContext(ctx, `UPDATE operations_pelican_tasks SET last_triggered_at=$2 WHERE id=$1`, t.ID, now); e != nil {
		return nil, e
	}
	return &service.OperationsRound{RunID: runID, Queued: len(items)}, nil
}
func (r *operationsRepository) SaveTask(ctx context.Context, t *service.OperationsTask, items []*service.OperationsResult, key string) (*service.OperationsTask, error) {
	tx, e := r.db.BeginTx(ctx, nil)
	if e != nil {
		return nil, e
	}
	defer tx.Rollback()
	now := time.Now().UTC()
	var old *service.OperationsTask
	if t.ID > 0 {
		old, e = scanOperationsTask(tx.QueryRowContext(ctx, `SELECT to_jsonb(t) FROM operations_pelican_tasks t WHERE id=$1 AND deleted_at IS NULL FOR UPDATE`, t.ID))
		if e != nil {
			return nil, e
		}
		if old.ConfigVersion != t.ConfigVersion {
			return nil, service.ErrOperationsConflict
		}
	}
	accounts, e := json.Marshal(t.AccountConfigs)
	if e != nil {
		return nil, e
	}
	immediate := t.Enabled && (old == nil || !old.Enabled)
	if t.Enabled {
		if old == nil || !old.Enabled || old.IntervalHours != t.IntervalHours {
			next := now.Add(time.Duration(t.IntervalHours) * time.Hour)
			t.NextRunAt = &next
		} else {
			t.NextRunAt = old.NextRunAt
		}
	} else {
		t.NextRunAt = nil
	}
	if old == nil {
		e = tx.QueryRowContext(ctx, `INSERT INTO operations_pelican_tasks(name,enabled,interval_hours,account_configs,reasoning_effort,prompt,storage_source,created_by,updated_by,next_run_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) RETURNING id`, t.Name, t.Enabled, t.IntervalHours, string(accounts), t.ReasoningEffort, t.Prompt, t.StorageSource, t.CreatedBy, t.UpdatedBy, t.NextRunAt).Scan(&t.ID)
		t.ConfigVersion = 1
	} else {
		t.ConfigVersion++
		_, e = tx.ExecContext(ctx, `UPDATE operations_pelican_tasks SET name=$2,enabled=$3,interval_hours=$4,account_configs=$5,reasoning_effort=$6,prompt=$7,storage_source=$8,updated_by=$9,next_run_at=$10,config_version=$11,updated_at=now() WHERE id=$1`, t.ID, t.Name, t.Enabled, t.IntervalHours, string(accounts), t.ReasoningEffort, t.Prompt, t.StorageSource, t.UpdatedBy, t.NextRunAt, t.ConfigVersion)
	}
	if e != nil {
		return nil, e
	}
	if immediate {
		if _, e = operationsInsertRound(ctx, tx, t, items, "scheduled", key, t.UpdatedBy); e != nil {
			return nil, e
		}
	}
	if !t.Enabled && old != nil && old.Enabled {
		if _, e = tx.ExecContext(ctx, `UPDATE operations_pelican_results SET status='cancelled',payload=payload||jsonb_build_object('finished_at',now(),'error_code','TASK_DISABLED'),expires_at=now()+interval '7 days',updated_at=now() WHERE task_id=$1 AND status='queued'`, t.ID); e != nil {
			return nil, e
		}
	}
	if e = tx.Commit(); e != nil {
		return nil, e
	}
	return r.GetTask(ctx, t.ID)
}
func (r *operationsRepository) DeleteTask(ctx context.Context, id int64) error {
	tx, e := r.db.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	res, e := tx.ExecContext(ctx, `UPDATE operations_pelican_tasks SET enabled=false,deleted_at=now(),next_run_at=NULL,updated_at=now() WHERE id=$1 AND deleted_at IS NULL`, id)
	if e != nil {
		return e
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return service.ErrOperationsNotFound
	}
	_, e = tx.ExecContext(ctx, `UPDATE operations_pelican_results SET status='cancelled',payload=payload||jsonb_build_object('finished_at',now(),'error_code','TASK_DELETED'),expires_at=now()+interval '7 days',updated_at=now() WHERE task_id=$1 AND status='queued'`, id)
	if e != nil {
		return e
	}
	return tx.Commit()
}
func (r *operationsRepository) Enqueue(ctx context.Context, id int64, kind, key string, actor int64, items []*service.OperationsResult, due *time.Time) (*service.OperationsRound, error) {
	tx, e := r.db.BeginTx(ctx, nil)
	if e != nil {
		return nil, e
	}
	defer tx.Rollback()
	t, e := scanOperationsTask(tx.QueryRowContext(ctx, `SELECT to_jsonb(t) FROM operations_pelican_tasks t WHERE id=$1 AND deleted_at IS NULL FOR UPDATE`, id))
	if e != nil {
		return nil, e
	}
	if len(items) > 0 && items[0].ConfigSnapshot.ConfigVersion != t.ConfigVersion {
		return nil, service.ErrOperationsConflict
	}
	if due != nil {
		if !t.Enabled || t.NextRunAt == nil || !t.NextRunAt.Equal(*due) {
			return nil, service.ErrOperationsConflict
		}
	}
	out, e := operationsInsertRound(ctx, tx, t, items, kind, key, actor)
	if e != nil {
		return nil, e
	}
	if due != nil {
		next := *due
		step := time.Duration(t.IntervalHours) * time.Hour
		now := time.Now()
		for !next.After(now) {
			next = next.Add(step)
		}
		_, e = tx.ExecContext(ctx, `UPDATE operations_pelican_tasks SET next_run_at=$2 WHERE id=$1`, id, next)
		if e != nil {
			return nil, e
		}
	}
	if e = tx.Commit(); e != nil {
		return nil, e
	}
	return out, nil
}
func (r *operationsRepository) Claim(ctx context.Context) (*service.OperationsResult, error) {
	tx, e := r.db.BeginTx(ctx, nil)
	if e != nil {
		return nil, e
	}
	defer tx.Rollback()
	if _, e = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(739102004)`); e != nil {
		return nil, e
	}
	var slot int
	e = tx.QueryRowContext(ctx, `SELECT s FROM generate_series(1,4) s WHERE NOT EXISTS(SELECT 1 FROM operations_pelican_results WHERE pool_slot=s) ORDER BY s LIMIT 1`).Scan(&slot)
	if errors.Is(e, sql.ErrNoRows) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	v, e := scanOperationsResult(tx.QueryRowContext(ctx, `SELECT `+operationsResultColumns+` FROM operations_pelican_results r WHERE status='queued' AND expires_at>now() ORDER BY queued_at,id FOR UPDATE SKIP LOCKED LIMIT 1`))
	if errors.Is(e, service.ErrOperationsNotFound) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	now := time.Now().UTC()
	deadline := now.Add(service.OperationsPelicanTimeout)
	token := uuid.NewString()
	v.Status = "running"
	v.PoolSlot = &slot
	v.WorkerToken = token
	v.HeartbeatAt = &now
	v.DeadlineAt = &deadline
	v.StartedAt = &now
	body, e := json.Marshal(v)
	if e != nil {
		return nil, e
	}
	_, e = tx.ExecContext(ctx, `UPDATE operations_pelican_results SET status='running',pool_slot=$2,worker_token=$3,heartbeat_at=$4,execution_deadline_at=$5,payload=$6::jsonb||jsonb_build_object('trigger_aliases',COALESCE(payload->'trigger_aliases','[]'::jsonb)),updated_at=now() WHERE id=$1`, v.ID, slot, token, now, deadline, string(body))
	if e != nil {
		return nil, e
	}
	if e = tx.Commit(); e != nil {
		return nil, e
	}
	return v, nil
}
func (r *operationsRepository) Heartbeat(ctx context.Context, id int64, token string) (bool, error) {
	res, e := r.db.ExecContext(ctx, `UPDATE operations_pelican_results SET heartbeat_at=now() WHERE id=$1 AND worker_token=$2 AND status IN ('running','uploading') AND execution_deadline_at>now()`, id, token)
	if e != nil {
		return false, e
	}
	n, e := res.RowsAffected()
	return n == 1, e
}
func (r *operationsRepository) UpdateResult(ctx context.Context, v *service.OperationsResult) error {
	body, e := json.Marshal(v)
	if e != nil {
		return e
	}
	var slot *int
	if v.Status == "running" || v.Status == "uploading" {
		slot = v.PoolSlot
	}
	res, e := r.db.ExecContext(ctx, `UPDATE operations_pelican_results SET status=$3,payload=$4::jsonb||jsonb_build_object('trigger_aliases',COALESCE(payload->'trigger_aliases','[]'::jsonb)),pool_slot=$5,storage_binding_encrypted=$6,object_key=$7,version_id=$8,expires_at=$9,updated_at=now() WHERE id=$1 AND worker_token=$2 AND status IN ('running','uploading') AND execution_deadline_at>now()`, v.ID, v.WorkerToken, v.Status, string(body), slot, v.StorageBinding, v.ObjectKey, v.VersionID, v.ExpiresAt)
	if e != nil {
		return e
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return service.ErrOperationsConflict
	}
	return nil
}
func (r *operationsRepository) GetResult(ctx context.Context, id int64) (*service.OperationsResult, error) {
	return scanOperationsResult(r.db.QueryRowContext(ctx, `SELECT `+operationsResultColumns+` FROM operations_pelican_results r WHERE id=$1`, id))
}
func (r *operationsRepository) ListResults(ctx context.Context, f service.OperationsResultFilter) (*service.OperationsPage, error) {
	args := []any{}
	where := []string{"r.expires_at>now()"}
	if f.Scope == "current" || f.Scope == "past" {
		where = append(where, operationsScopePredicate(f.Scope))
		if f.GroupID != 0 {
			operator := "IN"
			if f.Scope == "past" {
				operator = "NOT IN"
			}
			where = append(where, fmt.Sprintf("%d %s (SELECT group_id FROM configured_members WHERE account_id=r.account_id)", f.GroupID, operator))
		}
	}
	add := func(s string, v any) { args = append(args, v); where = append(where, fmt.Sprintf(s, len(args))) }
	source := `operations_pelican_results`
	if !f.History && !f.GroupByAccount {
		source = `(WITH terminal AS (SELECT DISTINCT ON(task_id,account_id) * FROM operations_pelican_results WHERE expires_at>now() AND status IN ('succeeded','failed','cancelled') ORDER BY task_id,account_id,id DESC), active AS (SELECT DISTINCT ON(task_id,account_id) * FROM operations_pelican_results WHERE expires_at>now() AND status IN ('queued','running','uploading') ORDER BY task_id,account_id,id DESC) SELECT * FROM terminal UNION ALL SELECT a.* FROM active a WHERE NOT EXISTS(SELECT 1 FROM terminal t WHERE t.task_id=a.task_id AND t.account_id=a.account_id))`
	}
	if f.TaskID > 0 {
		add("r.task_id=$%d", f.TaskID)
	}
	if f.AccountID > 0 {
		add("r.account_id=$%d", f.AccountID)
	}
	if f.GroupID == -1 {
		where = append(where, "r.group_snapshots='[]'::jsonb")
	} else if f.GroupID > 0 {
		add("r.group_snapshots @> $%d::jsonb", fmt.Sprintf(`[{"id":%d}]`, f.GroupID))
	}
	if f.Status != "" {
		add("r.status=$%d", f.Status)
	}
	if f.GroupByAccount && f.BeforeAccountID > 0 {
		add("r.account_id<$%d", f.BeforeAccountID)
	} else if !f.GroupByAccount && f.BeforeID > 0 {
		if f.History {
			add("(r.created_at,r.id)<(SELECT created_at,id FROM operations_pelican_results WHERE id=$%d)", f.BeforeID)
		} else {
			add("r.id<$%d", f.BeforeID)
		}
	}
	if f.From != nil {
		add("r.created_at>=$%d", *f.From)
	}
	if f.To != nil {
		add("r.created_at<=$%d", *f.To)
	}
	limit := f.Limit
	if limit < 1 || limit > 100 {
		limit = 24
	}
	args = append(args, limit+1)
	columns := strings.Replace(operationsResultColumns, "r.payload", `r.payload||jsonb_build_object('active_round',EXISTS(SELECT 1 FROM operations_pelican_results a WHERE a.task_id=r.task_id AND a.account_id=r.account_id AND a.status IN ('queued','running','uploading') AND a.expires_at>now()))`, 1)
	if f.Scope == "current" || f.Scope == "past" {
		operator := "IN"
		if f.Scope == "past" {
			operator = "NOT IN"
		}
		columns = strings.Replace(columns, "r.group_snapshots", `COALESCE((SELECT jsonb_agg(g) FROM jsonb_array_elements(r.group_snapshots) g WHERE (g->>'id')::bigint `+operator+` (SELECT group_id FROM configured_members WHERE account_id=r.account_id)),'[]'::jsonb)`, 1)
	}
	query := `SELECT ` + columns + ` FROM ` + source + ` r WHERE ` + strings.Join(where, " AND ") + fmt.Sprintf(" ORDER BY r.id DESC LIMIT $%d", len(args))
	if f.History {
		query = `SELECT ` + columns + ` FROM ` + source + ` r WHERE ` + strings.Join(where, " AND ") + fmt.Sprintf(" ORDER BY r.created_at DESC,r.id DESC LIMIT $%d", len(args))
	}
	if f.GroupByAccount {
		// Rank after filters, and page whole accounts rather than individual results.
		// The legacy latest-per-plan query cannot provide four tests across plans.
		query = `WITH ranked AS (SELECT r.*,row_number() OVER (PARTITION BY r.account_id ORDER BY r.created_at DESC,r.id DESC) AS account_rank FROM operations_pelican_results r WHERE ` + strings.Join(where, " AND ") + `), accounts AS (SELECT DISTINCT account_id FROM ranked ORDER BY account_id DESC LIMIT ` + fmt.Sprintf("$%d", len(args)) + `) SELECT ` + columns + ` FROM ranked r WHERE r.account_rank<=5 AND r.account_id IN (SELECT account_id FROM accounts) ORDER BY r.account_id DESC,r.created_at DESC,r.id DESC`
	}
	if f.Scope == "current" || f.Scope == "past" {
		if strings.HasPrefix(query, "WITH ") {
			query = operationsConfiguredGroups + ", " + strings.TrimPrefix(query, "WITH ")
		} else {
			query = operationsConfiguredGroups + " " + query
		}
	}
	rows, e := r.db.QueryContext(ctx, query, args...)
	if e != nil {
		return nil, e
	}
	items, e := collectOperationsResults(rows)
	if e != nil {
		return nil, e
	}
	page := &service.OperationsPage{Items: items, HasMore: len(items) > limit}
	if f.GroupByAccount {
		page.HasMore = false
		page.AccountHasMore = map[int64]bool{}
		page.Items = []*service.OperationsResult{}
		var previous int64
		accounts := 0
		resultCount := 0
		for i, item := range items {
			if i == 0 || item.AccountID != previous {
				accounts++
				previous = item.AccountID
				resultCount = 0
			}
			if accounts > limit {
				page.HasMore = true
				break
			}
			resultCount++
			page.AccountHasMore[item.AccountID] = resultCount > 4
			if resultCount <= 4 {
				page.Items = append(page.Items, item)
			}
		}
	} else if page.HasMore {
		page.Items = items[:limit]
	}
	if len(page.Items) > 0 {
		page.NextID = page.Items[len(page.Items)-1].ID
		if f.GroupByAccount {
			page.NextAccountID = page.Items[len(page.Items)-1].AccountID
		}
	}
	return page, nil
}
func (r *operationsRepository) FilterSnapshots(ctx context.Context) ([]*service.OperationsResult, error) {
	rows, e := r.db.QueryContext(ctx, `SELECT DISTINCT ON(account_id,group_snapshots) `+operationsResultColumns+` FROM operations_pelican_results r WHERE expires_at>now() ORDER BY account_id,group_snapshots,id DESC`)
	if e != nil {
		return nil, e
	}
	return collectOperationsResults(rows)
}
func (r *operationsRepository) RunResults(ctx context.Context, run string) ([]*service.OperationsResult, error) {
	rows, e := r.db.QueryContext(ctx, `SELECT `+operationsResultColumns+` FROM operations_pelican_results r WHERE run_id=$1 AND expires_at>now() ORDER BY id`, run)
	if e != nil {
		return nil, e
	}
	return collectOperationsResults(rows)
}
func (r *operationsRepository) Status(ctx context.Context) (*service.OperationsStatus, error) {
	s := &service.OperationsStatus{MaxWorkers: 4}
	e := r.db.QueryRowContext(ctx, `SELECT count(*) FILTER(WHERE status IN ('running','uploading')),count(*) FILTER(WHERE status='queued'),count(*) FILTER(WHERE cleanup_status='failed') FROM operations_pelican_results`).Scan(&s.Running, &s.Queued, &s.CleanupFailed)
	return s, e
}
func (r *operationsRepository) Recover(ctx context.Context) error {
	// A one-minute grace protects in-flight PUTs after the ten-minute hard deadline.
	_, e := r.db.ExecContext(ctx, `UPDATE operations_pelican_results SET status='failed',pool_slot=NULL,worker_token='',payload=payload||jsonb_build_object('error_code','WORKER_INTERRUPTED','error_stage','recovery','error_message','Execution interrupted; generation was not replayed','finished_at',now()),expires_at=now()+interval '7 days',updated_at=now() WHERE status IN ('running','uploading') AND execution_deadline_at<now()-interval '1 minute'`)
	if e != nil {
		return e
	}
	_, e = r.db.ExecContext(ctx, `UPDATE operations_pelican_results SET status='cancelled',payload=payload||jsonb_build_object('error_code','QUEUE_EXPIRED','finished_at',now()) WHERE status='queued' AND expires_at<=now()`)
	if e != nil {
		return e
	}
	_, e = r.db.ExecContext(ctx, `DELETE FROM operations_pelican_tasks t WHERE deleted_at IS NOT NULL AND NOT EXISTS(SELECT 1 FROM operations_pelican_results r WHERE r.task_id=t.id)`)
	return e
}
func (r *operationsRepository) ClaimCleanup(ctx context.Context, limit int) ([]*service.OperationsResult, error) {
	rows, e := r.db.QueryContext(ctx, `WITH candidates AS(SELECT id FROM operations_pelican_results WHERE expires_at<=now() AND status NOT IN ('running','uploading','queued') AND (cleanup_next_at IS NULL OR cleanup_next_at<=now()) ORDER BY expires_at FOR UPDATE SKIP LOCKED LIMIT $1), claimed AS(UPDATE operations_pelican_results SET cleanup_status='pending',cleanup_next_at=now()+interval '10 minutes' WHERE id IN(SELECT id FROM candidates) RETURNING *) SELECT `+operationsResultColumns+` FROM claimed r`, limit)
	if e != nil {
		return nil, e
	}
	return collectOperationsResults(rows)
}
func (r *operationsRepository) CompleteCleanup(ctx context.Context, id int64) error {
	_, e := r.db.ExecContext(ctx, `DELETE FROM operations_pelican_results WHERE id=$1 AND expires_at<=now() AND pool_slot IS NULL`, id)
	if e != nil {
		return e
	}
	_, e = r.db.ExecContext(ctx, `DELETE FROM operations_pelican_tasks t WHERE deleted_at IS NOT NULL AND NOT EXISTS(SELECT 1 FROM operations_pelican_results r WHERE r.task_id=t.id)`)
	return e
}
func (r *operationsRepository) FailCleanup(ctx context.Context, id int64, message string) error {
	_, e := r.db.ExecContext(ctx, `UPDATE operations_pelican_results SET cleanup_status='failed',cleanup_attempts=cleanup_attempts+1,cleanup_error=$2,cleanup_next_at=now()+make_interval(mins=>LEAST(60,5*(cleanup_attempts+1))),updated_at=now() WHERE id=$1`, id, message)
	return e
}
