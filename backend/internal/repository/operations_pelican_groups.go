package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// Configuration, rather than account availability or schedule state, defines
// current detection. These memberships are shared by catalog, queries and cleanup.
// detect_paused accounts keep their configured membership (cleanup protection)
// while the wall files them under past via the scope predicates below.
const operationsConfiguredGroups = `WITH configured_accounts AS (
 SELECT DISTINCT (c->>'account_id')::bigint AS account_id
 FROM operations_pelican_tasks t CROSS JOIN LATERAL jsonb_array_elements(t.account_configs) c
 WHERE t.deleted_at IS NULL
), configured_members AS (
 SELECT a.id AS account_id,a.name AS account_name,a.platform,
 COALESCE(f.detect_paused,false) AS detect_paused,
 COALESCE(g.id,-1) AS group_id,COALESCE(g.name,'') AS group_name
 FROM configured_accounts c JOIN accounts a ON a.id=c.account_id AND a.deleted_at IS NULL
 LEFT JOIN operations_pelican_account_flags f ON f.account_id=a.id
 LEFT JOIN (account_groups ag JOIN groups g ON g.id=ag.group_id AND g.deleted_at IS NULL) ON ag.account_id=a.id
), configured_groups AS (SELECT DISTINCT group_id FROM configured_members)`

// The wall files paused accounts under past results: membership counts as
// current only while its detection is not paused.
func operationsScopePredicate(scope string) string {
	membership := `EXISTS (SELECT 1 FROM configured_members cg WHERE cg.account_id=r.account_id AND NOT cg.detect_paused AND (
 (cg.group_id=-1 AND r.group_snapshots='[]'::jsonb) OR
 r.group_snapshots @> jsonb_build_array(jsonb_build_object('id',cg.group_id))))`
	if scope == "past" {
		// A shared result may belong to both tabs, but only through its respective groups.
		return `((r.group_snapshots='[]'::jsonb AND NOT EXISTS(SELECT 1 FROM configured_members WHERE account_id=r.account_id AND group_id=-1 AND NOT detect_paused)) OR EXISTS(SELECT 1 FROM jsonb_array_elements(r.group_snapshots) g WHERE NOT EXISTS(SELECT 1 FROM configured_members WHERE account_id=r.account_id AND group_id=(g->>'id')::bigint AND NOT detect_paused)))`
	}
	return membership
}

// Cleanup keeps the original membership rule: results of accounts that are
// still configured — including paused ones — must survive group cleanups.
const operationsCleanupPastScope = `((r.group_snapshots='[]'::jsonb AND NOT EXISTS(SELECT 1 FROM configured_members WHERE account_id=r.account_id AND group_id=-1)) OR EXISTS(SELECT 1 FROM jsonb_array_elements(r.group_snapshots) g WHERE NOT EXISTS(SELECT 1 FROM configured_members WHERE account_id=r.account_id AND group_id=(g->>'id')::bigint)))`

func (r *operationsRepository) WallGroups(ctx context.Context) (*service.OperationsWallGroups, error) {
	rows, err := r.db.QueryContext(ctx, operationsConfiguredGroups+`, members AS (
 SELECT account_id,account_name,platform,group_id,group_name,0 AS source_rank,0::bigint AS result_order FROM configured_members
 UNION ALL
 SELECT r.account_id,COALESCE(r.payload->>'account_name',''),COALESCE(r.payload->>'protocol',''),
 (g->>'id')::bigint,g->>'name',1,r.id
 FROM operations_pelican_results r CROSS JOIN LATERAL jsonb_array_elements(
 CASE WHEN r.group_snapshots='[]'::jsonb THEN '[{"id":-1,"name":""}]'::jsonb ELSE r.group_snapshots END) g
 WHERE r.expires_at>now()
	 ) SELECT m.group_id,m.group_name,m.account_id,m.account_name,m.platform,
	 EXISTS(SELECT 1 FROM configured_members c WHERE c.group_id=m.group_id AND c.account_id=m.account_id AND NOT c.detect_paused),a.priority,
	 COALESCE(f.detect_paused,false)
	 FROM members m LEFT JOIN accounts a ON a.id=m.account_id AND a.deleted_at IS NULL
	 LEFT JOIN operations_pelican_account_flags f ON f.account_id=m.account_id
	 ORDER BY m.group_id,m.source_rank,m.result_order DESC,m.account_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := &service.OperationsWallGroups{Current: []service.OperationsWallGroup{}, Past: []service.OperationsWallGroup{}}
	type groupKey struct {
		id      int64
		current bool
	}
	groups := map[groupKey]*service.OperationsWallGroup{}
	for rows.Next() {
		var groupID int64
		var name string
		var a service.OperationsAccount
		var live bool
		if err := rows.Scan(&groupID, &name, &a.ID, &a.Name, &a.Platform, &live, &a.Priority, &a.DetectPaused); err != nil {
			return nil, err
		}
		key := groupKey{groupID, live}
		g := groups[key]
		if g == nil {
			g = &service.OperationsWallGroup{ID: groupID, Name: name, Accounts: []service.OperationsAccount{}}
			groups[key] = g
		}
		found := false
		for _, old := range g.Accounts {
			if old.ID == a.ID {
				found = true
				break
			}
		}
		if !found {
			a.Groups = []service.OperationsGroup{}
			a.HasUngrouped = groupID == -1
			if groupID > 0 {
				a.Groups = append(a.Groups, service.OperationsGroup{ID: groupID, Name: name})
			}
			g.Accounts = append(g.Accounts, a)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// Rows are sorted, keep group ordering deterministic without relying on maps.
	keys := []groupKey{}
	for key := range groups {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i].id < keys[j].id })
	for _, key := range keys {
		if key.current {
			out.Current = append(out.Current, *groups[key])
		} else {
			out.Past = append(out.Past, *groups[key])
		}
	}
	return out, nil
}

func (r *operationsRepository) PrepareGroupCleanup(ctx context.Context, groupID int64) (*service.OperationsGroupCleanupBatch, error) {
	return r.prepareCleanup(ctx, groupID, 0)
}

func (r *operationsRepository) PrepareAccountCleanup(ctx context.Context, accountID, groupID int64) (*service.OperationsGroupCleanupBatch, error) {
	return r.prepareCleanup(ctx, groupID, accountID)
}

func (r *operationsRepository) prepareCleanup(ctx context.Context, groupID, accountID int64) (*service.OperationsGroupCleanupBatch, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	// Serialize against task configuration and membership edits for classification.
	if _, err = tx.ExecContext(ctx, `LOCK TABLE operations_pelican_tasks,accounts,account_groups,groups IN SHARE MODE`); err != nil {
		return nil, err
	}
	var current bool
	if err = tx.QueryRowContext(ctx, operationsConfiguredGroups+` SELECT EXISTS(SELECT 1 FROM configured_members WHERE ($1::bigint=0 OR group_id=$1) AND ($2::bigint=0 OR account_id=$2))`, groupID, accountID).Scan(&current); err != nil {
		return nil, err
	}
	condition := `($2::bigint=0 OR r.group_snapshots @> $1::jsonb)`
	arg := fmt.Sprintf(`[{"id":%d}]`, groupID)
	if groupID == -1 {
		condition = `r.group_snapshots=$1::jsonb`
		arg = `[]`
	}
	// A group may contain both configured and removed accounts. Only the past
	// membership is eligible, so cleanup cannot touch its current accounts.
	rows, err := tx.QueryContext(ctx, operationsConfiguredGroups+` SELECT `+operationsResultColumns+` FROM operations_pelican_results r WHERE `+condition+` AND ($3::bigint=0 OR r.account_id=$3) AND `+operationsCleanupPastScope+` AND ($2::bigint=0 OR NOT EXISTS(SELECT 1 FROM configured_members c WHERE c.account_id=r.account_id AND c.group_id=$2)) AND r.expires_at>now() ORDER BY r.id FOR UPDATE OF r`, arg, groupID, accountID)
	if err != nil {
		return nil, err
	}
	items, err := collectOperationsResults(rows)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 && current {
		return nil, service.ErrOperationsConflict
	}
	configured := map[int64]bool{}
	if groupID == 0 {
		members, err := tx.QueryContext(ctx, operationsConfiguredGroups+` SELECT group_id FROM configured_members WHERE account_id=$1`, accountID)
		if err != nil {
			return nil, err
		}
		for members.Next() {
			var id int64
			if err := members.Scan(&id); err != nil {
				members.Close()
				return nil, err
			}
			configured[id] = true
		}
		err = members.Err()
		members.Close()
		if err != nil {
			return nil, err
		}
	}
	for _, item := range items {
		if item.Status == "running" || item.Status == "uploading" {
			return nil, service.ErrOperationsConflict
		}
	}
	out := &service.OperationsGroupCleanupBatch{Items: []*service.OperationsResult{}}
	for _, item := range items {
		kept := []service.OperationsGroup{}
		for _, g := range item.Groups {
			if (groupID != 0 && g.ID != groupID) || (groupID == 0 && configured[g.ID]) {
				kept = append(kept, g)
			}
		}
		if len(kept) > 0 {
			raw, _ := json.Marshal(kept)
			if _, err = tx.ExecContext(ctx, `UPDATE operations_pelican_results SET group_snapshots=$2,updated_at=now() WHERE id=$1`, item.ID, string(raw)); err != nil {
				return nil, err
			}
			out.Detached++
			continue
		}
		if _, err = tx.ExecContext(ctx, `UPDATE operations_pelican_results SET status=CASE WHEN status='queued' THEN 'cancelled' ELSE status END,expires_at=now(),cleanup_status='pending',cleanup_next_at=now()+interval '10 minutes',updated_at=now() WHERE id=$1`, item.ID); err != nil {
			return nil, err
		}
		out.Items = append(out.Items, item)
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return out, nil
}
