package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// activityWorker polls usage and payment changes. Refund recovery serializes
// against billing on the user row; ordinary reward processing uses activity tables.
type activityWorker struct {
	cancel context.CancelFunc
	done   chan struct{}
	once   sync.Once
}

func (s *ActivityService) Start() {
	if s.worker != nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.worker = &activityWorker{cancel: cancel, done: make(chan struct{})}
	go func() {
		defer close(s.worker.done)
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		var tick uint64
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				run, cancelRun := context.WithTimeout(ctx, 5*time.Second)
				if tick%15 == 0 {
					if err := s.SyncRefundReviews(run); err != nil {
						slog.Error("activity refund review sync failed", "error", err)
					}
					if err := s.SyncLotteryOrders(run); err != nil {
						slog.Error("activity lottery order sync failed", "error", err)
					}
				}
				tick++
				for i := 0; i < 10; i++ {
					n, err := s.IngestUsageEvents(run)
					if err != nil {
						slog.Error("activity usage ingest failed", "error", err)
						break
					}
					if n == 0 {
						break
					}
				}
				for i := 0; i < 100; i++ {
					processed, err := s.ProcessConsumptionEvent(run)
					if err != nil {
						slog.Error("activity reward worker failed", "error", err)
						break
					}
					if !processed {
						break
					}
				}
				cancelRun()
			}
		}
	}()
}

func (s *ActivityService) Stop() {
	if s == nil || s.worker == nil {
		return
	}
	s.worker.once.Do(func() { s.worker.cancel(); <-s.worker.done })
}

// IngestUsageEvents reads new rows from usage_logs via a primary-key cursor,
// deducts any unabsorbed withdrawn gift balance on activity_wallets, and
// enqueues eligible consumption events into activity_consumption_outbox.
func (s *ActivityService) IngestUsageEvents(ctx context.Context) (int, error) {
	tx, err := s.db.BeginTx(ctx, nil)
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

	var cursorInit bool
	var lastID int64
	err = tx.QueryRowContext(ctx, `SELECT cursor_initialized, last_usage_log_id FROM activity_worker_status WHERE id=1 FOR UPDATE NOWAIT`).Scan(&cursorInit, &lastID)
	if err != nil {
		// Another replica holds the ingestion cursor lock right now.
		return 0, nil
	}

	if !cursorInit {
		var earliest sql.NullTime
		if err := tx.QueryRowContext(ctx, `SELECT MIN(starts_at) FROM activity_consumption_campaigns WHERE published_at IS NOT NULL`).Scan(&earliest); err != nil {
			return 0, err
		}
		if earliest.Valid {
			if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(id), 0) FROM usage_logs WHERE created_at < $1`, earliest.Time).Scan(&lastID); err != nil {
				return 0, err
			}
		} else {
			if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(id), 0) FROM usage_logs`).Scan(&lastID); err != nil {
				return 0, err
			}
		}
		if _, err := tx.ExecContext(ctx, `UPDATE activity_worker_status SET cursor_initialized=TRUE, last_usage_log_id=$1 WHERE id=1`, lastID); err != nil {
			return 0, err
		}
	}

	rows, err := tx.QueryContext(ctx, `
		SELECT id, user_id, COALESCE(request_id, ''), COALESCE(api_key_id, 0),
		       actual_cost::text, COALESCE(billing_type, 0), COALESCE(request_type, 0), created_at
		FROM usage_logs
		WHERE id > $1
		ORDER BY id ASC
		LIMIT 200
	`, lastID)
	if err != nil {
		return 0, err
	}
	type rawUsage struct {
		id          int64
		userID      int64
		requestID   string
		apiKeyID    int64
		actualCost  string
		billingType int
		requestType int
		createdAt   time.Time
	}
	batch := make([]rawUsage, 0, 200)
	for rows.Next() {
		var u rawUsage
		if err := rows.Scan(&u.id, &u.userID, &u.requestID, &u.apiKeyID, &u.actualCost, &u.billingType, &u.requestType, &u.createdAt); err != nil {
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

	if len(batch) == 0 {
		return 0, tx.Commit()
	}

	maxID := lastID
	for _, u := range batch {
		if u.id > maxID {
			maxID = u.id
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
				SET unabsorbed_gift = GREATEST(unabsorbed_gift - $2::numeric, 0),
				    updated_at = NOW()
				WHERE user_id = $1 AND unabsorbed_gift > 0
			`, u.userID, u.actualCost); err != nil {
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
				SELECT unabsorbed_gift
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

	if _, err := tx.ExecContext(ctx, `UPDATE activity_worker_status SET last_usage_log_id=$1 WHERE id=1`, maxID); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return len(batch), nil
}

// SyncRefundReviews scans payment_orders read-only for balance orders in refund
// states and suspends pending consumption entitlements until reviewed.
func (s *ActivityService) SyncRefundReviews(ctx context.Context) error {
	// Enqueue changes and advance the scan checkpoint atomically. A bad user stays
	// in a durable retry queue while later users and newly changed orders progress.
	scan, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer scan.Rollback()
	var lastScan sql.NullTime
	if err := scan.QueryRowContext(ctx, `SELECT last_refund_scan_at FROM activity_worker_status WHERE id=1 FOR UPDATE`).Scan(&lastScan); err != nil {
		return err
	}
	scanStarted := time.Now()
	since := time.Time{}
	if lastScan.Valid {
		since = lastScan.Time.Add(-10 * time.Second)
	}
	if _, err := scan.ExecContext(ctx, `INSERT INTO activity_refund_sync_queue(user_id,updated_at,next_attempt_at)
 SELECT DISTINCT user_id,$2::timestamptz,$2::timestamptz FROM payment_orders WHERE order_type='balance' AND status IN ('REFUND_REQUESTED','REFUNDING','REFUND_PENDING','PARTIALLY_REFUNDED','REFUNDED') AND user_id IS NOT NULL AND COALESCE(updated_at,NOW()) >= $1
 ON CONFLICT(user_id) DO UPDATE SET updated_at=EXCLUDED.updated_at`, since, scanStarted); err != nil {
		return err
	}
	if _, err := scan.ExecContext(ctx, `UPDATE activity_worker_status SET last_refund_scan_at=$1 WHERE id=1`, scanStarted); err != nil {
		return err
	}
	if err := scan.Commit(); err != nil {
		return err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT user_id FROM activity_refund_sync_queue WHERE next_attempt_at<=NOW() ORDER BY next_attempt_at,user_id LIMIT 200`)
	if err != nil {
		return err
	}
	var users []int64
	for rows.Next() {
		var uid int64
		if err := rows.Scan(&uid); err != nil {
			rows.Close()
			return err
		}
		users = append(users, uid)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	var syncErrors []error
	for _, uid := range users {
		if err := ctx.Err(); err != nil {
			return errors.Join(append(syncErrors, err)...)
		}
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		err = lockActivityWalletTx(ctx, tx, uid)
		if err == nil {
			err = syncUserRefundReviewsTx(ctx, tx, uid)
		}
		if err == nil {
			err = settleUserRefundRewardsTx(ctx, tx, uid)
		}
		if err == nil {
			_, err = tx.ExecContext(ctx, `DELETE FROM activity_refund_sync_queue WHERE user_id=$1 AND updated_at<=$2`, uid, scanStarted)
		}
		if err == nil {
			err = tx.Commit()
		} else {
			_ = tx.Rollback()
		}
		if err != nil {
			syncErrors = append(syncErrors, fmt.Errorf("user %d: %w", uid, err))
			message := err.Error()
			if len(message) > 1000 {
				message = message[:1000]
			}
			if _, queueErr := s.db.ExecContext(ctx, `UPDATE activity_refund_sync_queue SET last_error=$2,next_attempt_at=NOW()+interval '30 seconds' WHERE user_id=$1 AND updated_at<=$3`, uid, message, scanStarted); queueErr != nil {
				return errors.Join(append(syncErrors, queueErr)...)
			}
			continue
		}
		s.invalidateBalance(ctx, uid)
	}
	return errors.Join(syncErrors...)
}

func (s *ActivityService) ProcessConsumptionEvent(ctx context.Context) (bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()

	var enabled bool
	if err = tx.QueryRowContext(ctx, `SELECT processing_enabled FROM activity_runtime WHERE id=1`).Scan(&enabled); err != nil {
		return false, err
	}
	if !enabled {
		return false, nil
	}
	var id int64
	err = tx.QueryRowContext(ctx, `SELECT event_id FROM activity_consumption_outbox WHERE processed_at IS NULL AND next_attempt_at<=NOW() ORDER BY next_attempt_at,event_id FOR UPDATE SKIP LOCKED LIMIT 1`).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if _, err = tx.ExecContext(ctx, `SET LOCAL lock_timeout='500ms'`); err != nil {
		return false, err
	}
	if _, err = tx.ExecContext(ctx, `SAVEPOINT reward_event`); err != nil {
		return false, err
	}
	var complete bool
	processErr := tx.QueryRowContext(ctx, `SELECT activity_process_consumption_event($1)`, id).Scan(&complete)
	message := ""
	if processErr != nil {
		if _, err = tx.ExecContext(ctx, `ROLLBACK TO SAVEPOINT reward_event`); err != nil {
			return false, err
		}
		message = processErr.Error()
		if len(message) > 1000 {
			message = message[:1000]
		}
	} else if !complete {
		message = "waiting for reward budget or refund review"
	}
	if _, err = tx.ExecContext(ctx, `UPDATE activity_consumption_outbox SET attempts=attempts+1,last_error=$2,processed_at=CASE WHEN $3 THEN clock_timestamp() ELSE NULL END,next_attempt_at=clock_timestamp()+LEAST(300,5*(attempts+1))*interval '1 second' WHERE event_id=$1`, id, message, complete && processErr == nil); err != nil {
		return false, err
	}
	if processErr != nil {
		_, err = tx.ExecContext(ctx, `UPDATE activity_worker_status SET last_error_at=clock_timestamp(),last_error=$1 WHERE id=1`, message)
	} else {
		_, err = tx.ExecContext(ctx, `UPDATE activity_worker_status SET last_success_at=clock_timestamp() WHERE id=1`)
	}
	if err != nil {
		return false, err
	}
	if err = tx.Commit(); err != nil {
		return false, err
	}
	if processErr != nil {
		slog.Warn("activity reward event deferred", "event_id", id, "error", processErr)
	}
	return true, nil
}

func activityAwardsAllowed(ctx context.Context, tx *sql.Tx) error {
	var allowed bool
	if err := tx.QueryRowContext(ctx, `SELECT awards_enabled FROM activity_runtime WHERE id=1 FOR SHARE`).Scan(&allowed); err != nil {
		return err
	}
	if !allowed {
		return ErrActivityDisabled
	}
	return nil
}

type ActivityHealth struct {
	ProcessingEnabled bool       `json:"processing_enabled"`
	AwardsEnabled     bool       `json:"awards_enabled"`
	LastUsageLogID    int64      `json:"last_usage_log_id"`
	Pending           int64      `json:"pending_events"`
	Failed            int64      `json:"failed_events"`
	Oldest            *time.Time `json:"oldest_pending_at"`
	LastSuccess       *time.Time `json:"last_success_at"`
	LastError         string     `json:"last_error"`
}

func (s *ActivityService) Health(ctx context.Context) (*ActivityHealth, error) {
	out := &ActivityHealth{}
	err := s.db.QueryRowContext(ctx, `SELECT r.processing_enabled,r.awards_enabled,w.last_usage_log_id,(SELECT COUNT(*) FROM activity_consumption_outbox WHERE processed_at IS NULL),(SELECT COUNT(*) FROM activity_consumption_outbox WHERE processed_at IS NULL AND last_error<>''),(SELECT MIN(created_at) FROM activity_consumption_outbox WHERE processed_at IS NULL),w.last_success_at,w.last_error FROM activity_runtime r CROSS JOIN activity_worker_status w WHERE r.id=1 AND w.id=1`).Scan(&out.ProcessingEnabled, &out.AwardsEnabled, &out.LastUsageLogID, &out.Pending, &out.Failed, &out.Oldest, &out.LastSuccess, &out.LastError)
	return out, err
}

func (s *ActivityService) UpdateRuntime(ctx context.Context, actor int64, processingEnabled, awardsEnabled bool) (*ActivityHealth, error) {
	if awardsEnabled && !processingEnabled {
		return nil, errors.New("enable reward processing before enabling reward payouts")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `UPDATE activity_runtime SET processing_enabled=$1, awards_enabled=$2, updated_at=NOW() WHERE id=1`, processingEnabled, awardsEnabled); err != nil {
		return nil, err
	}
	if err := consumptionAudit(ctx, tx, actor, "runtime:update", 1, map[string]bool{
		"processing_enabled": processingEnabled,
		"awards_enabled":     awardsEnabled,
	}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return s.Health(ctx)
}
