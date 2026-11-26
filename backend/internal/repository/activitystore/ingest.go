package activitystore

import (
	"context"
	"database/sql"
	"errors"
	"github.com/lib/pq"
	"time"
)

// IngestUsageEvents reconciles legacy usage logs by durable per-row acknowledgement.
// Modern bills already have atomic events. IDs are never used as commit order.
func IngestUsageEvents(ctx context.Context, db *sql.DB) (int, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	var enabled bool
	if err := tx.QueryRowContext(ctx, `SELECT processing_enabled FROM activity_runtime WHERE id=1`).Scan(&enabled); err != nil {
		return 0, err
	}
	if !enabled {
		return 0, nil
	}

	// This lock limits legacy reconciliation to one replica. The ID is only a
	// diagnostic high-water mark; it never determines which rows are eligible.
	var lastID, reconcileID int64
	err = tx.QueryRowContext(ctx, `SELECT last_usage_log_id,usage_reconcile_id FROM activity_worker_status WHERE id=1 FOR UPDATE NOWAIT`).Scan(&lastID, &reconcileID)
	if err != nil {
		var pg *pq.Error
		if errors.As(err, &pg) && pg.Code == "55P03" {
			return 0, nil
		}
		return 0, err
	}

	// One forward page and one rotating historical page bound each pass's
	// work. Reconciliation has no time window: a late commit below the high-water
	// mark is eventually visited, however long its transaction remained open.
	rows, err := tx.QueryContext(ctx, `
 WITH ids AS (
  (SELECT id FROM usage_logs WHERE id>$1 ORDER BY id LIMIT 200)
  UNION
  (SELECT id FROM usage_logs WHERE id>$2 AND id<=$1 ORDER BY id LIMIT 200)
 )
 SELECT u.id,u.user_id,COALESCE(u.request_id,''),COALESCE(u.api_key_id,0),u.actual_cost::text,COALESCE(u.billing_type,0),COALESCE(u.request_type,0),u.created_at,
 done.usage_log_id IS NOT NULL,
 u.created_at<(SELECT cutover_at FROM activity_billing_config WHERE id=1) AND u.created_at>=LEAST((SELECT first_call_tracking_started_at FROM activity_config WHERE id=1),COALESCE((SELECT MIN(starts_at) FROM activity_consumption_campaigns WHERE published_at IS NOT NULL),'infinity'::timestamptz))
 FROM ids JOIN usage_logs u ON u.id=ids.id LEFT JOIN activity_usage_ingested done ON done.usage_log_id=u.id ORDER BY u.id
 `, lastID, reconcileID)
	if err != nil {
		return 0, err
	}
	type rawUsage struct {
		id                    int64
		userID                int64
		requestID             string
		apiKeyID              int64
		actualCost            string
		billingType           int
		requestType           int
		createdAt             time.Time
		acknowledged, inScope bool
	}
	batch := make([]rawUsage, 0, 200)
	for rows.Next() {
		var u rawUsage
		if err := rows.Scan(&u.id, &u.userID, &u.requestID, &u.apiKeyID, &u.actualCost, &u.billingType, &u.requestType, &u.createdAt, &u.acknowledged, &u.inScope); err != nil {
			rows.Close()
			return 0, err
		}
		batch = append(batch, u)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return 0, err
	}
	rows.Close()

	maxReconcile := int64(0)
	processed := 0
	maxID := lastID
	for _, u := range batch {
		if u.id <= lastID && u.id > maxReconcile {
			maxReconcile = u.id
		}
		if u.id > maxID {
			maxID = u.id
		}
		if u.acknowledged {
			continue
		}
		processed++
		if _, err := tx.ExecContext(ctx, `INSERT INTO activity_usage_ingested(usage_log_id) VALUES($1) ON CONFLICT DO NOTHING`, u.id); err != nil {
			return 0, err
		}
		if !u.inScope {
			continue
		}
		var billed bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM activity_usage_events WHERE request_id=$1 AND api_key_id=$2 AND billing_recorded)`, u.requestID, u.apiKeyID).Scan(&billed); err != nil {
			return 0, err
		}
		if billed {
			continue
		}
		if u.billingType != 0 || u.requestType == 4 {
			continue
		}
		var positive bool
		if err := tx.QueryRowContext(ctx, `SELECT $1::numeric > 0`, u.actualCost).Scan(&positive); err != nil {
			return 0, err
		}
		if !positive {
			continue
		}

		if err := LockWallet(ctx, tx, u.userID); err != nil {
			return 0, err
		}
		// Record post-launch first billed API call idempotently without triggers.
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO activity_first_calls (user_id, request_id, occurred_at)
			SELECT $1, $2, $3
			WHERE EXISTS (
				SELECT 1 FROM users usr CROSS JOIN activity_config cfg
				WHERE usr.id = $1 AND usr.created_at >= cfg.first_call_tracking_started_at
			)
			ON CONFLICT (user_id) DO NOTHING
		`, u.userID, u.requestID, u.createdAt); err != nil {
			return 0, err
		}

		// Serialize reward eligibility with master-switch changes without stopping
		// billing ingestion or gift absorption for the other activities.
		var consumptionLock int
		if err := tx.QueryRowContext(ctx, `SELECT id FROM activity_consumption_config WHERE id=1 FOR SHARE`).Scan(&consumptionLock); err != nil {
			return 0, err
		}
		// Avoid persisting activity_usage_events when no published consumption campaign
		// covers this usage event, while still burning down any withdrawn gift balance.
		var inCampaign bool
		if err := tx.QueryRowContext(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM activity_consumption_campaigns c
				WHERE c.published_at IS NOT NULL
				  AND c.state IN ('active', 'paused', 'closed')
				  AND c.published_at <= $1
				  AND c.starts_at <= $1
				  AND $1 < LEAST(c.ends_at, COALESCE(c.stopped_at, c.ends_at))
				  AND c.claim_until > NOW()
				  AND activity_rollout_allowed('consumption', c.rollout_percent, $2)
                  AND activity_consumption_enabled_at($1)
			)
		`, u.createdAt, u.userID).Scan(&inCampaign); err != nil {
			return 0, err
		}
		if !inCampaign {
			if _, err := tx.ExecContext(ctx, `
				UPDATE activity_wallets
				SET unabsorbed_gift = GREATEST(unabsorbed_gift - LEAST($2::numeric,GREATEST(0,unabsorbed_gift-COALESCE((SELECT SUM(amount_cents)::numeric/100 FROM activity_wallet_ledger WHERE user_id=$1 AND direction='withdraw' AND created_at>$3),0))), 0),
				    updated_at = NOW()
				WHERE user_id = $1 AND unabsorbed_gift > 0
			`, u.userID, u.actualCost, u.createdAt); err != nil {
				return 0, err
			}
			continue
		}

		if _, err := tx.ExecContext(ctx, `INSERT INTO activity_wallets(user_id) VALUES($1) ON CONFLICT DO NOTHING`, u.userID); err != nil {
			return 0, err
		}

		var absorbed, eligible string
		var inserted bool
		err := tx.QueryRowContext(ctx, `
			WITH w AS (
				SELECT GREATEST(0,unabsorbed_gift-COALESCE((SELECT SUM(amount_cents)::numeric/100 FROM activity_wallet_ledger WHERE user_id=$2 AND direction='withdraw' AND created_at>$6),0)) AS unabsorbed_gift
    FROM activity_wallets
				WHERE user_id = $2
				FOR UPDATE
			),
			calc AS (
				SELECT LEAST($5::numeric, w.unabsorbed_gift) AS absorbed,
				       GREATEST($5::numeric - LEAST($5::numeric, w.unabsorbed_gift), 0) AS eligible
				FROM w
			),
			ins AS (
				INSERT INTO activity_usage_events (
					usage_log_id, user_id, request_id, api_key_id,
					actual_cost, absorbed_gift, eligible_cost, occurred_at
				)
				SELECT $1, $2, $3, $4, $5::numeric, calc.absorbed, calc.eligible, $6
				FROM calc
				ON CONFLICT (usage_log_id) DO NOTHING
				RETURNING absorbed_gift::text, eligible_cost::text
			)
			SELECT absorbed_gift, eligible_cost, TRUE FROM ins
		`, u.id, u.userID, u.requestID, u.apiKeyID, u.actualCost, u.createdAt).Scan(&absorbed, &eligible, &inserted)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return 0, err
		}
		if inserted {
			if _, err := tx.ExecContext(ctx, `
				UPDATE activity_wallets
				SET unabsorbed_gift = GREATEST(unabsorbed_gift - $2::numeric, 0),
				    updated_at = NOW()
				WHERE user_id = $1 AND $2::numeric > 0
			`, u.userID, absorbed); err != nil {
				return 0, err
			}
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO activity_consumption_outbox (event_id, user_id, created_at)
				SELECT $1, $2, $3
				WHERE $4::numeric > 0
				ON CONFLICT (event_id) DO NOTHING
			`, u.id, u.userID, u.createdAt, eligible); err != nil {
				return 0, err
			}
		}
	}

	if _, err := tx.ExecContext(ctx, `UPDATE activity_worker_status SET cursor_initialized=TRUE,last_usage_log_id=$1,usage_reconcile_id=$2 WHERE id=1`, maxID, maxReconcile); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return processed, nil
}
