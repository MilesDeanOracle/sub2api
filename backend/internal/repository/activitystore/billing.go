// Package activitystore owns transactional activity accounting persistence.
// It accepts the caller's transaction so core debits and reward accounting commit
// or roll back together; it does not depend on the service package.
package activitystore

import (
	"context"
	"database/sql"
	"errors"

	"github.com/shopspring/decimal"
)

type Tx interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func Scan(ctx context.Context, tx Tx, query string, args []any, dest ...any) error {
	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return err
		}
		return sql.ErrNoRows
	}
	return rows.Scan(dest...)
}

// LockWallet is also the lock order used by billing and wallet transfers.
func LockWallet(ctx context.Context, tx Tx, uid int64) error {
	var id int64
	if err := Scan(ctx, tx, `SELECT id FROM users WHERE id=$1 FOR UPDATE`, []any{uid}, &id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO activity_wallets(user_id) VALUES($1) ON CONFLICT DO NOTHING`, uid); err != nil {
		return err
	}
	return Scan(ctx, tx, `SELECT user_id FROM activity_wallets WHERE user_id=$1 FOR UPDATE`, []any{uid}, &id)
}

// RecordUsage runs after the user debit, while its row lock is still held.
// A committed bill always has a durable event, even if the usage log is delayed,
// lost, or assigned a lower ID than a transaction that committed first.
func RecordUsage(ctx context.Context, tx Tx, uid, key int64, request string, cost float64, eligible bool) error {
	if cost <= 0 {
		return nil
	}
	if err := LockWallet(ctx, tx, uid); err != nil {
		return err
	}
	var gift string
	if err := Scan(ctx, tx, `SELECT unabsorbed_gift::text FROM activity_wallets WHERE user_id=$1`, []any{uid}, &gift); err != nil {
		return err
	}
	available, err := decimal.NewFromString(gift)
	if err != nil {
		return err
	}
	absorbed := decimal.Min(available, decimal.NewFromFloat(cost))
	return recordUsage(ctx, tx, uid, key, request, cost, absorbed, eligible, true)
}

func recordUsage(ctx context.Context, tx Tx, uid, key int64, request string, cost float64, absorbed decimal.Decimal, eligible, burn bool) error {
	progress := decimal.Zero
	if eligible {
		progress = decimal.NewFromFloat(cost).Sub(absorbed)
	}
	var id int64
	err := Scan(ctx, tx, `INSERT INTO activity_usage_events(usage_log_id,user_id,request_id,api_key_id,actual_cost,absorbed_gift,eligible_cost,occurred_at,billing_recorded)
 VALUES(nextval('activity_billing_event_seq'),$1,$2,$3,$4,$5::numeric,$6::numeric,clock_timestamp(),TRUE)
 ON CONFLICT(request_id,api_key_id) WHERE billing_recorded DO NOTHING RETURNING usage_log_id`, []any{uid, request, key, cost, absorbed.String(), progress.String()}, &id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if burn {
		if _, err := tx.ExecContext(ctx, `UPDATE activity_wallets SET unabsorbed_gift=unabsorbed_gift-$2::numeric,updated_at=NOW() WHERE user_id=$1`, uid, absorbed.String()); err != nil {
			return err
		}
	}
	if eligible {
		if _, err := tx.ExecContext(ctx, `INSERT INTO activity_first_calls(user_id,request_id,occurred_at)
 SELECT $1,$2,clock_timestamp() WHERE EXISTS(SELECT 1 FROM users u CROSS JOIN activity_config c WHERE u.id=$1 AND u.created_at>=c.first_call_tracking_started_at)
 ON CONFLICT(user_id) DO NOTHING`, uid, request); err != nil {
			return err
		}
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO activity_consumption_outbox(event_id,user_id)
 SELECT e.usage_log_id,e.user_id FROM activity_usage_events e
 WHERE e.usage_log_id=$1 AND e.eligible_cost>0 AND EXISTS(
 SELECT 1 FROM activity_consumption_campaigns c WHERE c.published_at IS NOT NULL
 AND c.state IN ('active','paused','closed') AND c.published_at<=e.occurred_at
 AND c.starts_at<=e.occurred_at AND e.occurred_at<LEAST(c.ends_at,COALESCE(c.stopped_at,c.ends_at)) AND c.claim_until>clock_timestamp())`, id)
	return err
}

// Reserve gifts with the corresponding balance hold. Later wallet transfers
// cannot change the source composition of an already reserved image job.
func ReserveHold(ctx context.Context, tx Tx, uid int64, batch string, amount float64) error {
	if err := LockWallet(ctx, tx, uid); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, `WITH held AS (
 INSERT INTO activity_billing_holds(user_id,batch_id,gift)
 SELECT user_id,$2,LEAST(unabsorbed_gift,$3::numeric) FROM activity_wallets WHERE user_id=$1
 ON CONFLICT DO NOTHING RETURNING gift)
 UPDATE activity_wallets SET unabsorbed_gift=unabsorbed_gift-held.gift FROM held WHERE user_id=$1`, uid, batch, amount)
	return err
}

func FinishHold(ctx context.Context, tx Tx, uid, key int64, batch, request string, cost float64, capture bool) error {
	if err := LockWallet(ctx, tx, uid); err != nil {
		return err
	}
	var raw string
	err := Scan(ctx, tx, `DELETE FROM activity_billing_holds WHERE user_id=$1 AND batch_id=$2 RETURNING gift::text`, []any{uid, batch}, &raw)
	if errors.Is(err, sql.ErrNoRows) {
		// A hold created before this migration has no verified source split. Exclude
		// it from reward progress rather than consuming a later gift or guessing.
		if capture && cost > 0 {
			return recordUsage(ctx, tx, uid, key, request, cost, decimal.NewFromFloat(cost), false, false)
		}
		return nil
	}
	if err != nil {
		return err
	}
	gift, err := decimal.NewFromString(raw)
	if err != nil {
		return err
	}
	absorbed := decimal.Zero
	if capture {
		absorbed = decimal.Min(gift, decimal.NewFromFloat(cost))
	}
	if _, err := tx.ExecContext(ctx, `UPDATE activity_wallets SET unabsorbed_gift=unabsorbed_gift+$2::numeric WHERE user_id=$1`, uid, gift.Sub(absorbed).String()); err != nil {
		return err
	}
	if capture && cost > 0 {
		return recordUsage(ctx, tx, uid, key, request, cost, absorbed, true, false)
	}
	return nil
}
