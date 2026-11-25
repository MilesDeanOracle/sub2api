package service

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"sync"
	"time"
)

// One bounded batch per tick, across all replicas with SKIP LOCKED. Reservations
// and acknowledgements commit together; crashes release locks and leave events
// retryable. No worker goroutine is created by constructors used in tests.
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
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				run, cancel := context.WithTimeout(ctx, 5*time.Second)
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
				cancel()
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

func (s *ActivityService) ProcessConsumptionEvent(ctx context.Context) (bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	// Do not lock the runtime row: the worker's user/config lock order stays fixed.
	var enabled bool
	if err = tx.QueryRowContext(ctx, `SELECT processing_enabled FROM activity_runtime WHERE id=1`).Scan(&enabled); err != nil {
		return false, err
	}
	if !enabled {
		return false, nil
	}
	var id int64
	err = tx.QueryRowContext(ctx, `SELECT ledger_id FROM activity_consumption_outbox WHERE processed_at IS NULL AND next_attempt_at<=NOW() ORDER BY next_attempt_at,ledger_id FOR UPDATE SKIP LOCKED LIMIT 1`).Scan(&id)
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
	if _, err = tx.ExecContext(ctx, `UPDATE activity_consumption_outbox SET attempts=attempts+1,last_error=$2,processed_at=CASE WHEN $3 THEN clock_timestamp() ELSE NULL END,next_attempt_at=clock_timestamp()+LEAST(300,5*(attempts+1))*interval '1 second' WHERE ledger_id=$1`, id, message, complete && processErr == nil); err != nil {
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
		slog.Warn("activity reward event deferred", "ledger_id", id, "error", processErr)
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
	Pending           int64      `json:"pending_events"`
	Failed            int64      `json:"failed_events"`
	Oldest            *time.Time `json:"oldest_pending_at"`
	LastSuccess       *time.Time `json:"last_success_at"`
	LastError         string     `json:"last_error"`
}

func (s *ActivityService) Health(ctx context.Context) (*ActivityHealth, error) {
	out := &ActivityHealth{}
	err := s.db.QueryRowContext(ctx, `SELECT r.processing_enabled,r.awards_enabled,(SELECT COUNT(*) FROM activity_consumption_outbox WHERE processed_at IS NULL),(SELECT COUNT(*) FROM activity_consumption_outbox WHERE processed_at IS NULL AND last_error<>''),(SELECT MIN(created_at) FROM activity_consumption_outbox WHERE processed_at IS NULL),w.last_success_at,w.last_error FROM activity_runtime r CROSS JOIN activity_worker_status w WHERE r.id=1 AND w.id=1`).Scan(&out.ProcessingEnabled, &out.AwardsEnabled, &out.Pending, &out.Failed, &out.Oldest, &out.LastSuccess, &out.LastError)
	return out, err
}
