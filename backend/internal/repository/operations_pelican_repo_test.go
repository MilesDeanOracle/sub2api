package repository

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

// This opt-in test uses a new random schema, never the application's tables.
func operationsTestDatabase(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("OPERATIONS_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set OPERATIONS_TEST_DATABASE_URL to test durable pool against PostgreSQL")
	}
	base, e := sql.Open("postgres", dsn)
	require.NoError(t, e)
	require.NoError(t, base.Ping())
	schema := "operations_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	_, e = base.Exec(`CREATE SCHEMA ` + schema)
	require.NoError(t, e)
	u, e := url.Parse(dsn)
	require.NoError(t, e)
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	db, e := sql.Open("postgres", u.String())
	require.NoError(t, e)
	db.SetMaxOpenConns(16)
	t.Cleanup(func() { db.Close(); _, _ = base.Exec(`DROP SCHEMA ` + schema + ` CASCADE`); base.Close() })
	migration, e := os.ReadFile("../../migrations/256_operations_pelican_console.sql")
	require.NoError(t, e)
	_, e = db.Exec(string(migration))
	require.NoError(t, e)
	_, e = db.Exec(`CREATE TABLE accounts(id bigint PRIMARY KEY,name text,platform text,deleted_at timestamptz); CREATE TABLE groups(id bigint PRIMARY KEY,name text,deleted_at timestamptz); CREATE TABLE account_groups(account_id bigint,group_id bigint)`)
	require.NoError(t, e)
	return db
}
func operationsTestTask(t *testing.T, r service.OperationsRepository, n int) *service.OperationsTask {
	t.Helper()
	task := &service.OperationsTask{Name: uuid.NewString(), IntervalHours: 2, Prompt: "HTML", StorageSource: "backup_s3", AccountConfigs: []service.OperationsAccountConfig{}}
	for i := 1; i <= n; i++ {
		task.AccountConfigs = append(task.AccountConfigs, service.OperationsAccountConfig{AccountID: int64(i), ModelID: "model"})
	}
	saved, e := r.SaveTask(context.Background(), task, nil, "")
	require.NoError(t, e)
	return saved
}
func operationsTestRound(t *testing.T, r service.OperationsRepository, task *service.OperationsTask, key string) *service.OperationsRound {
	t.Helper()
	items := []*service.OperationsResult{}
	for _, a := range task.AccountConfigs {
		items = append(items, &service.OperationsResult{AccountID: a.AccountID, AccountName: fmt.Sprint(a.AccountID), Groups: []service.OperationsGroup{{ID: 1, Name: "Group"}}, ModelID: a.ModelID, ConfigSnapshot: *task})
	}
	out, e := r.Enqueue(context.Background(), task.ID, "manual", key, 1, items, nil)
	require.NoError(t, e)
	return out
}
func TestOperationsDurablePoolAndHistory(t *testing.T) {
	db := operationsTestDatabase(t)
	ctx := context.Background()
	repo := NewOperationsRepository(db)
	task := operationsTestTask(t, repo, 10)
	first := operationsTestRound(t, repo, task, "request-one")
	require.Equal(t, 10, first.Queued)
	same := operationsTestRound(t, repo, task, "request-one")
	require.True(t, same.Existing)
	require.Equal(t, first.RunID, same.RunID)
	coalesced := operationsTestRound(t, repo, task, "request-coalesced")
	require.Equal(t, first.RunID, coalesced.RunID)
	secondTask := operationsTestTask(t, repo, 2)
	operationsTestRound(t, repo, secondTask, "request-two")
	var wg sync.WaitGroup
	var mu sync.Mutex
	claimed := []*service.OperationsResult{}
	errs := []error{}
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			otherInstance := NewOperationsRepository(db)
			item, e := otherInstance.Claim(ctx)
			mu.Lock()
			defer mu.Unlock()
			if e != nil {
				errs = append(errs, e)
			}
			if item != nil {
				claimed = append(claimed, item)
			}
		}()
	}
	wg.Wait()
	require.Empty(t, errs)
	require.Len(t, claimed, 4)
	status, e := repo.Status(ctx)
	require.NoError(t, e)
	require.Equal(t, 4, status.Running)
	require.Equal(t, 8, status.Queued)
	finish := func(v *service.OperationsResult, state string) {
		v.Status = state
		now := time.Now().UTC()
		v.FinishedAt = &now
		require.NoError(t, repo.UpdateResult(ctx, v))
	}
	finish(claimed[0], "failed")
	next, e := repo.Claim(ctx)
	require.NoError(t, e)
	require.NotNil(t, next)
	require.NotEqual(t, claimed[0].ID, next.ID)
	require.Equal(t, *claimed[0].PoolSlot, *next.PoolSlot)
	// Stale ownership cannot change a result after its slot was released.
	claimed[0].Status = "succeeded"
	require.ErrorIs(t, repo.UpdateResult(ctx, claimed[0]), service.ErrOperationsConflict)
	for _, v := range claimed[1:] {
		finish(v, "succeeded")
	}
	finish(next, "succeeded")
	for {
		v, e := repo.Claim(ctx)
		require.NoError(t, e)
		if v == nil {
			break
		}
		finish(v, "succeeded")
	}
	coalescedRetry := operationsTestRound(t, repo, task, "request-coalesced")
	require.Equal(t, first.RunID, coalescedRetry.RunID)
	newer := operationsTestRound(t, repo, task, "request-three")
	require.NotEqual(t, first.RunID, newer.RunID)
	v, e := repo.Claim(ctx)
	require.NoError(t, e)
	finish(v, "failed")
	page, e := repo.ListResults(ctx, service.OperationsResultFilter{TaskID: task.ID, AccountID: v.AccountID, GroupID: 1})
	require.NoError(t, e)
	require.Len(t, page.Items, 1)
	require.Equal(t, "failed", page.Items[0].Status)
	history, e := repo.ListResults(ctx, service.OperationsResultFilter{History: true, TaskID: task.ID, AccountID: v.AccountID})
	require.NoError(t, e)
	require.GreaterOrEqual(t, len(history.Items), 2)
	_, e = db.Exec(`UPDATE operations_pelican_results SET expires_at=now()-interval '1 hour' WHERE id=$1`, v.ID)
	require.NoError(t, e)
	pending, e := repo.ClaimCleanup(ctx, 10)
	require.NoError(t, e)
	require.Len(t, pending, 1)
	require.NoError(t, repo.FailCleanup(ctx, v.ID, "storage denied"))
	kept, e := repo.GetResult(ctx, v.ID)
	require.NoError(t, e)
	require.Equal(t, "failed", kept.CleanupStatus)
	require.NoError(t, repo.CompleteCleanup(ctx, v.ID))
	_, e = repo.GetResult(ctx, v.ID)
	require.ErrorIs(t, e, service.ErrOperationsNotFound)
}

func TestOperationsScheduleLifecycleAndRecovery(t *testing.T) {
	db := operationsTestDatabase(t)
	ctx := context.Background()
	repo := NewOperationsRepository(db)
	task := operationsTestTask(t, repo, 3)
	items := func(t *service.OperationsTask) []*service.OperationsResult {
		out := []*service.OperationsResult{}
		for _, a := range t.AccountConfigs {
			out = append(out, &service.OperationsResult{AccountID: a.AccountID, Groups: []service.OperationsGroup{}, ConfigSnapshot: *t})
		}
		return out
	}
	// Enabling is atomic with the first round; editing an enabled plan does not
	// create a second round or reset its cadence.
	task.Enabled = true
	task, e := repo.SaveTask(ctx, task, items(task), "enable-first")
	require.NoError(t, e)
	require.NotNil(t, task.NextRunAt)
	nextTime := *task.NextRunAt
	status, e := repo.Status(ctx)
	require.NoError(t, e)
	require.Equal(t, 3, status.Queued)
	stale := *task
	task.Name = "renamed"
	task, e = repo.SaveTask(ctx, task, items(task), "edit-enabled")
	require.NoError(t, e)
	require.True(t, nextTime.Equal(*task.NextRunAt))
	_, e = repo.SaveTask(ctx, &stale, nil, "stale-edit")
	require.ErrorIs(t, e, service.ErrOperationsConflict)
	active, e := repo.Claim(ctx)
	require.NoError(t, e)
	require.NotNil(t, active)
	task.Enabled = false
	task, e = repo.SaveTask(ctx, task, nil, "disable")
	require.NoError(t, e)
	require.Nil(t, task.NextRunAt)
	status, e = repo.Status(ctx)
	require.NoError(t, e)
	require.Equal(t, 1, status.Running)
	require.Zero(t, status.Queued)
	// Deadline recovery fences a stale worker and retains object location for
	// cleanup, without generating another queued unit.
	_, e = db.Exec(`UPDATE operations_pelican_results SET execution_deadline_at=now()-interval '2 minutes',object_key='operations/pelican/uncertain.html' WHERE id=$1`, active.ID)
	require.NoError(t, e)
	require.NoError(t, repo.Recover(ctx))
	recovered, e := repo.GetResult(ctx, active.ID)
	require.NoError(t, e)
	require.Equal(t, "failed", recovered.Status)
	require.Equal(t, "operations/pelican/uncertain.html", recovered.ObjectKey)
	require.Nil(t, recovered.PoolSlot)
	ok, e := repo.Heartbeat(ctx, active.ID, active.WorkerToken)
	require.NoError(t, e)
	require.False(t, ok)
	// A manual run works while scheduling is disabled, without setting next time.
	operationsTestRound(t, repo, task, "manual-while-disabled")
	current, e := repo.GetTask(ctx, task.ID)
	require.NoError(t, e)
	require.Nil(t, current.NextRunAt)
	require.NoError(t, repo.DeleteTask(ctx, task.ID))
	_, e = repo.GetTask(ctx, task.ID)
	require.ErrorIs(t, e, service.ErrOperationsNotFound)
	// Restart after several missed intervals queues one round and advances to
	// the next future interval instead of replaying every missed round.
	other := operationsTestTask(t, repo, 2)
	_, e = db.Exec(`UPDATE operations_pelican_tasks SET enabled=true,next_run_at=now()-interval '9 hours' WHERE id=$1`, other.ID)
	require.NoError(t, e)
	other, e = repo.GetTask(ctx, other.ID)
	require.NoError(t, e)
	due := *other.NextRunAt
	round, e := repo.Enqueue(ctx, other.ID, "scheduled", "missed-schedule", 0, items(other), &due)
	require.NoError(t, e)
	require.Equal(t, 2, round.Queued)
	other, e = repo.GetTask(ctx, other.ID)
	require.NoError(t, e)
	require.True(t, other.NextRunAt.After(time.Now()))
	require.True(t, other.NextRunAt.Before(time.Now().Add(2*time.Hour)))
	_, e = repo.Enqueue(ctx, other.ID, "scheduled", "missed-schedule", 0, items(other), &due)
	require.ErrorIs(t, e, service.ErrOperationsConflict)
}

func TestOperationsAccountRowsPaginationAndFilters(t *testing.T) {
	db := operationsTestDatabase(t)
	ctx := context.Background()
	repo := NewOperationsRepository(db)
	first := operationsTestTask(t, repo, 3)
	second := operationsTestTask(t, repo, 3)
	insert := func(task, account int64, status string, group int, at time.Time) int64 {
		t.Helper()
		var id int64
		err := db.QueryRowContext(ctx, `INSERT INTO operations_pelican_results(task_id,run_id,account_id,trigger_key,status,group_snapshots,payload,created_at,expires_at) VALUES($1,$2,$3,'test',$4,$5,'{}',$6,now()+interval '7 days') RETURNING id`, task, uuid.NewString(), account, status, fmt.Sprintf(`[{"id":%d,"name":"Group"}]`, group), at).Scan(&id)
		require.NoError(t, err)
		return id
	}
	now := time.Now().UTC()
	// A quiet account must not disappear behind another account's many tests.
	quietID := insert(first.ID, 3, "succeeded", 1, now)
	ids := []int64{}
	for i := 0; i < 6; i++ {
		task, status := first.ID, "succeeded"
		if i%2 == 1 {
			task, status = second.ID, "failed"
		}
		ids = append(ids, insert(task, 2, status, 1, now.Add(time.Duration(i-10)*time.Hour)))
	}
	activeID := insert(first.ID, 2, "queued", 2, now)
	expiredID := insert(first.ID, 2, "failed", 1, now)
	_, err := db.ExecContext(ctx, `UPDATE operations_pelican_results SET expires_at=now()-interval '1 hour' WHERE id=$1`, expiredID)
	require.NoError(t, err)
	lastID := insert(first.ID, 1, "succeeded", 1, now)

	filter := service.OperationsResultFilter{GroupByAccount: true, Limit: 2}
	page, err := repo.ListResults(ctx, filter)
	require.NoError(t, err)
	require.True(t, page.HasMore)
	require.EqualValues(t, 2, page.NextAccountID)
	require.Len(t, page.Items, 5)
	require.Equal(t, []int64{quietID, activeID, ids[5], ids[4], ids[3]}, operationsResultIDs(page.Items))
	require.Equal(t, map[int64]bool{3: false, 2: true}, page.AccountHasMore)
	// Per-row history continues immediately after the fourth displayed test.
	older, err := repo.ListResults(ctx, service.OperationsResultFilter{History: true, AccountID: 2, BeforeID: ids[3], Limit: 3})
	require.NoError(t, err)
	require.Equal(t, []int64{ids[2], ids[1], ids[0]}, operationsResultIDs(older.Items))
	require.False(t, older.HasMore)
	require.Equal(t, ids[0], older.NextID)
	// An ID inserted later with an older timestamp must still paginate correctly.
	backdated := insert(second.ID, 2, "failed", 2, now.Add(-24*time.Hour))
	older, err = repo.ListResults(ctx, service.OperationsResultFilter{History: true, AccountID: 2, BeforeID: ids[3], Limit: 3})
	require.NoError(t, err)
	require.True(t, older.HasMore)
	require.Equal(t, []int64{ids[2], ids[1], ids[0]}, operationsResultIDs(older.Items))
	tail, err := repo.ListResults(ctx, service.OperationsResultFilter{History: true, AccountID: 2, BeforeID: older.NextID, Limit: 3})
	require.NoError(t, err)
	require.Equal(t, []int64{backdated}, operationsResultIDs(tail.Items))
	require.False(t, tail.HasMore)
	_, err = db.ExecContext(ctx, `DELETE FROM operations_pelican_results WHERE id=$1`, backdated)
	require.NoError(t, err)
	filter.BeforeAccountID = page.NextAccountID
	page, err = repo.ListResults(ctx, filter)
	require.NoError(t, err)
	require.False(t, page.HasMore)
	require.Equal(t, map[int64]bool{1: false}, page.AccountHasMore)
	require.Equal(t, []int64{lastID}, operationsResultIDs(page.Items))

	// Filters apply before ranking, so older matching tests fill the row.
	page, err = repo.ListResults(ctx, service.OperationsResultFilter{GroupByAccount: true, AccountID: 2, GroupID: 1, TaskID: first.ID})
	require.NoError(t, err)
	require.Equal(t, []int64{ids[4], ids[2], ids[0]}, operationsResultIDs(page.Items))
	page, err = repo.ListResults(ctx, service.OperationsResultFilter{GroupByAccount: true, AccountID: 2, Status: "failed"})
	require.NoError(t, err)
	require.Equal(t, []int64{ids[5], ids[3], ids[1]}, operationsResultIDs(page.Items))
	until := now.Add(-7 * time.Hour)
	page, err = repo.ListResults(ctx, service.OperationsResultFilter{GroupByAccount: true, History: true, AccountID: 2, To: &until})
	require.NoError(t, err)
	require.Equal(t, []int64{ids[3], ids[2], ids[1], ids[0]}, operationsResultIDs(page.Items))
}

func operationsResultIDs(items []*service.OperationsResult) []int64 {
	ids := make([]int64, len(items))
	for i, item := range items {
		ids[i] = item.ID
	}
	return ids
}

func TestOperationsGroupLifecycleAndCleanup(t *testing.T) {
	db := operationsTestDatabase(t)
	ctx := context.Background()
	repo := NewOperationsRepository(db)
	task := operationsTestTask(t, repo, 1) // Disabled schedules still configure detection.
	_, err := db.Exec(`INSERT INTO accounts VALUES(1,'Selected','openai',NULL),(2,'Past','openai',NULL),(3,'Unused','openai',NULL); INSERT INTO groups VALUES(1,'Current',NULL),(2,'Past',NULL),(3,'Never configured',NULL); INSERT INTO account_groups VALUES(1,1),(2,2),(3,3)`)
	require.NoError(t, err)
	insert := func(account int64, groups, status string) int64 {
		var id int64
		err := db.QueryRow(`INSERT INTO operations_pelican_results(task_id,run_id,account_id,trigger_key,status,group_snapshots,payload,expires_at) VALUES($1,$2,$3,'test',$4,$5,'{"account_name":"Snapshot"}',now()+interval '7 days') RETURNING id`, task.ID, uuid.NewString(), account, status, groups).Scan(&id)
		require.NoError(t, err)
		return id
	}
	current := insert(1, `[{"id":1,"name":"Current"}]`, "succeeded")
	shared := insert(1, `[{"id":1,"name":"Current"},{"id":2,"name":"Past"}]`, "succeeded")
	past := insert(2, `[{"id":2,"name":"Past"}]`, "queued")
	catalog, err := repo.WallGroups(ctx)
	require.NoError(t, err)
	require.Len(t, catalog.Current, 1)
	require.EqualValues(t, 1, catalog.Current[0].ID)
	require.Len(t, catalog.Past, 1)
	require.EqualValues(t, 2, catalog.Past[0].ID)
	live, err := repo.ListResults(ctx, service.OperationsResultFilter{History: true, GroupByAccount: true, Scope: "current"})
	require.NoError(t, err)
	require.ElementsMatch(t, []int64{current, shared}, operationsResultIDs(live.Items))
	for _, item := range live.Items {
		require.Equal(t, []service.OperationsGroup{{ID: 1, Name: "Current"}}, item.Groups)
	}
	older, err := repo.ListResults(ctx, service.OperationsResultFilter{History: true, GroupByAccount: true, Scope: "past"})
	require.NoError(t, err)
	require.Equal(t, []int64{past, shared}, operationsResultIDs(older.Items))
	require.EqualValues(t, 2, older.Items[1].Groups[0].ID)
	wrong, err := repo.ListResults(ctx, service.OperationsResultFilter{History: true, Scope: "past", GroupID: 1})
	require.NoError(t, err)
	require.Empty(t, wrong.Items)
	_, err = repo.PrepareGroupCleanup(ctx, 1)
	require.ErrorIs(t, err, service.ErrOperationsConflict)
	batch, err := repo.PrepareGroupCleanup(ctx, 2)
	require.NoError(t, err)
	require.Equal(t, 1, batch.Detached)
	require.Len(t, batch.Items, 1)
	require.Equal(t, past, batch.Items[0].ID)
	kept, err := repo.GetResult(ctx, shared)
	require.NoError(t, err)
	require.Equal(t, []service.OperationsGroup{{ID: 1, Name: "Current"}}, kept.Groups)
	queued, err := repo.GetResult(ctx, past)
	require.NoError(t, err)
	require.Equal(t, "cancelled", queued.Status)
	require.True(t, queued.ExpiresAt.Before(time.Now()))
	require.NoError(t, repo.CompleteCleanup(ctx, past))
	_, err = repo.GetResult(ctx, past)
	require.ErrorIs(t, err, service.ErrOperationsNotFound)
	catalog, err = repo.WallGroups(ctx)
	require.NoError(t, err)
	require.Empty(t, catalog.Past)
	// Removing configuration moves snapshots to the past tab.
	require.NoError(t, repo.DeleteTask(ctx, task.ID))
	catalog, err = repo.WallGroups(ctx)
	require.NoError(t, err)
	require.Empty(t, catalog.Current)
	require.Len(t, catalog.Past, 1)
	// Running work blocks cleanup, and its row remains intact.
	_, err = db.Exec(`UPDATE operations_pelican_results SET status='running',pool_slot=1 WHERE id=$1`, current)
	require.NoError(t, err)
	_, err = repo.PrepareGroupCleanup(ctx, 1)
	require.ErrorIs(t, err, service.ErrOperationsConflict)
	kept, err = repo.GetResult(ctx, current)
	require.NoError(t, err)
	require.True(t, kept.ExpiresAt.After(time.Now()))
}

func TestOperationsUngroupedLifecycle(t *testing.T) {
	db := operationsTestDatabase(t)
	repo := NewOperationsRepository(db)
	ctx := context.Background()
	task := operationsTestTask(t, repo, 1)
	_, err := db.Exec(`INSERT INTO accounts VALUES(1,'No group','openai',NULL)`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO operations_pelican_results(task_id,run_id,account_id,trigger_key,status,group_snapshots,payload,expires_at) VALUES($1,$2,1,'test','succeeded','[]','{}',now()+interval '7 days')`, task.ID, uuid.NewString())
	require.NoError(t, err)
	catalog, err := repo.WallGroups(ctx)
	require.NoError(t, err)
	require.Len(t, catalog.Current, 1)
	require.EqualValues(t, -1, catalog.Current[0].ID)
	require.Empty(t, catalog.Past)
	page, err := repo.ListResults(ctx, service.OperationsResultFilter{Scope: "current", History: true, GroupByAccount: true, GroupID: -1})
	require.NoError(t, err)
	require.Len(t, page.Items, 1)
	_, err = repo.PrepareGroupCleanup(ctx, -1)
	require.ErrorIs(t, err, service.ErrOperationsConflict)
	_, err = db.Exec(`UPDATE operations_pelican_tasks SET account_configs='[]' WHERE id=$1`, task.ID)
	require.NoError(t, err)
	catalog, err = repo.WallGroups(ctx)
	require.NoError(t, err)
	require.Empty(t, catalog.Current)
	require.Len(t, catalog.Past, 1)
	page, err = repo.ListResults(ctx, service.OperationsResultFilter{Scope: "past", History: true, GroupByAccount: true, GroupID: -1})
	require.NoError(t, err)
	require.Len(t, page.Items, 1)
	batch, err := repo.PrepareGroupCleanup(ctx, -1)
	require.NoError(t, err)
	require.Len(t, batch.Items, 1)
}

func TestOperationsMixedGroupAccountConfiguration(t *testing.T) {
	db := operationsTestDatabase(t)
	repo := NewOperationsRepository(db)
	ctx := context.Background()
	task := operationsTestTask(t, repo, 1) // Account 1 stays selected in a disabled plan.
	_, err := db.Exec(`INSERT INTO accounts VALUES(1,'Configured','openai',NULL),(2,'Removed','openai',NULL),(3,'Never tested','openai',NULL); INSERT INTO groups VALUES(1,'Mixed',NULL); INSERT INTO account_groups VALUES(1,1),(2,1),(3,1)`)
	require.NoError(t, err)
	insert := func(account int64) int64 {
		var id int64
		err := db.QueryRow(`INSERT INTO operations_pelican_results(task_id,run_id,account_id,trigger_key,status,group_snapshots,payload,expires_at) VALUES($1,$2,$3,'test','succeeded','[{"id":1,"name":"Mixed"}]','{}',now()+interval '7 days') RETURNING id`, task.ID, uuid.NewString(), account).Scan(&id)
		require.NoError(t, err)
		return id
	}
	currentID := insert(1)
	var pastIDs []int64
	for i := 0; i < 7; i++ {
		pastIDs = append(pastIDs, insert(2))
	}
	catalog, err := repo.WallGroups(ctx)
	require.NoError(t, err)
	require.Len(t, catalog.Current, 1)
	require.Len(t, catalog.Past, 1)
	require.EqualValues(t, 1, catalog.Current[0].ID)
	require.EqualValues(t, 1, catalog.Past[0].ID)
	require.Len(t, catalog.Current[0].Accounts, 1)
	require.Len(t, catalog.Past[0].Accounts, 1)
	require.EqualValues(t, 1, catalog.Current[0].Accounts[0].ID)
	require.EqualValues(t, 2, catalog.Past[0].Accounts[0].ID)
	live, err := repo.ListResults(ctx, service.OperationsResultFilter{Scope: "current", History: true, GroupByAccount: true, GroupID: 1})
	require.NoError(t, err)
	require.Equal(t, []int64{currentID}, operationsResultIDs(live.Items))
	past, err := repo.ListResults(ctx, service.OperationsResultFilter{Scope: "past", History: true, GroupByAccount: true, GroupID: 1, Limit: 1})
	require.NoError(t, err)
	require.Equal(t, []int64{pastIDs[6], pastIDs[5], pastIDs[4], pastIDs[3]}, operationsResultIDs(past.Items))
	require.True(t, past.AccountHasMore[2])
	older, err := repo.ListResults(ctx, service.OperationsResultFilter{Scope: "past", History: true, GroupID: 1, AccountID: 2, BeforeID: pastIDs[3], Limit: 3})
	require.NoError(t, err)
	require.Equal(t, []int64{pastIDs[2], pastIDs[1], pastIDs[0]}, operationsResultIDs(older.Items))
	require.False(t, older.HasMore)
	wrong, err := repo.ListResults(ctx, service.OperationsResultFilter{Scope: "past", History: true, GroupID: 1, AccountID: 1})
	require.NoError(t, err)
	require.Empty(t, wrong.Items)
	// Re-selecting an account moves all of its results back to current.
	_, err = db.Exec(`UPDATE operations_pelican_tasks SET account_configs=account_configs||'[{"account_id":2,"model_id":"model"}]'::jsonb WHERE id=$1`, task.ID)
	require.NoError(t, err)
	catalog, err = repo.WallGroups(ctx)
	require.NoError(t, err)
	require.Empty(t, catalog.Past)
	_, err = repo.PrepareGroupCleanup(ctx, 1)
	require.ErrorIs(t, err, service.ErrOperationsConflict)
	_, err = db.Exec(`UPDATE operations_pelican_tasks SET account_configs='[{"account_id":1,"model_id":"model"}]' WHERE id=$1`, task.ID)
	require.NoError(t, err)
	// Cleanup of the mixed group must never expire the configured account's result.
	batch, err := repo.PrepareGroupCleanup(ctx, 1)
	require.NoError(t, err)
	require.Len(t, batch.Items, 7)
	for _, item := range batch.Items {
		require.EqualValues(t, 2, item.AccountID)
		require.NoError(t, repo.CompleteCleanup(ctx, item.ID))
	}
	kept, err := repo.GetResult(ctx, currentID)
	require.NoError(t, err)
	require.True(t, kept.ExpiresAt.After(time.Now()))
	catalog, err = repo.WallGroups(ctx)
	require.NoError(t, err)
	require.Empty(t, catalog.Past)
	require.Len(t, catalog.Current, 1)
}
