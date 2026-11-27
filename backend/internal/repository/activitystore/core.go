package activitystore

import (
	"context"
	"time"

	"github.com/lib/pq"
)

// This file is the activity module's read adapter for core Sub2API tables.
// Core schema changes should be absorbed here instead of spreading through the
// activity services. Financial writes remain in the transaction-scoped helpers.

func UserActive(ctx context.Context, tx Tx, userID int64) (bool, error) {
	var active bool
	err := Scan(ctx, tx, `SELECT status='active' FROM users WHERE id=$1 AND deleted_at IS NULL`, []any{userID}, &active)
	return active, err
}

func UserExists(ctx context.Context, tx Tx, userID int64) (bool, error) {
	var exists bool
	err := Scan(ctx, tx, `SELECT EXISTS(SELECT 1 FROM users WHERE id=$1)`, []any{userID}, &exists)
	return exists, err
}

func UsersExistCount(ctx context.Context, tx Tx, userIDs []int64) (int, error) {
	var count int
	err := Scan(ctx, tx, `SELECT COUNT(*) FROM users WHERE id=ANY($1) AND deleted_at IS NULL`, []any{pq.Array(userIDs)}, &count)
	return count, err
}

func HasPendingBalanceRefund(ctx context.Context, tx Tx, userID int64) (bool, error) {
	var pending bool
	err := Scan(ctx, tx, `SELECT EXISTS(SELECT 1 FROM payment_orders WHERE user_id=$1 AND status IN ('REFUND_REQUESTED','REFUNDING','REFUND_PENDING'))`, []any{userID}, &pending)
	return pending, err
}

func HasCompletedRecharge(ctx context.Context, tx Tx, userID int64) (bool, error) {
	var completed bool
	err := Scan(ctx, tx, `SELECT EXISTS(SELECT 1 FROM payment_orders WHERE user_id=$1 AND order_type='balance' AND status='COMPLETED' AND COALESCE(pay_amount,0)>0)`, []any{userID}, &completed)
	return completed, err
}

func EligibleSpendAtLeast(ctx context.Context, tx Tx, userID, minimumCents int64) (bool, error) {
	var eligible bool
	err := Scan(ctx, tx, `SELECT COALESCE(SUM(actual_cost),0) >= $2::numeric/100 FROM usage_logs WHERE user_id=$1 AND COALESCE(billing_type,0)=0 AND actual_cost>0 AND COALESCE(request_type,0)<>4`, []any{userID, minimumCents}, &eligible)
	return eligible, err
}

func FirstEligibleUsageAt(ctx context.Context, tx Tx, userID int64) (time.Time, error) {
	var first time.Time
	err := Scan(ctx, tx, `SELECT created_at FROM usage_logs WHERE user_id=$1 AND COALESCE(billing_type,0)=0 AND actual_cost>0 AND COALESCE(request_type,0) <> 4 ORDER BY created_at LIMIT 1`, []any{userID}, &first)
	return first, err
}

func EligibleUsageBetween(ctx context.Context, tx Tx, userID int64, after, before time.Time) (bool, error) {
	var used bool
	err := Scan(ctx, tx, `SELECT EXISTS(SELECT 1 FROM usage_logs WHERE user_id=$1 AND COALESCE(billing_type,0)=0 AND actual_cost>0 AND COALESCE(request_type,0) <> 4 AND created_at>$2 AND created_at<$3)`, []any{userID, after, before}, &used)
	return used, err
}

func EligibleUsageAfter(ctx context.Context, tx Tx, userID int64, after time.Time) (bool, error) {
	var used bool
	err := Scan(ctx, tx, `SELECT EXISTS(SELECT 1 FROM usage_logs WHERE user_id=$1 AND COALESCE(billing_type,0)=0 AND actual_cost>0 AND COALESCE(request_type,0)<>4 AND created_at>$2)`, []any{userID, after}, &used)
	return used, err
}

type PaidOrder struct {
	ID       int64
	PayCents int
	PaidAt   time.Time
	Status   string
	Currency string
}

func PaidBalanceOrders(ctx context.Context, tx Tx, userID int64, limit int) ([]PaidOrder, error) {
	rows, err := tx.QueryContext(ctx, `SELECT id,ROUND(pay_amount*100)::int,COALESCE(paid_at,completed_at),status,
		COALESCE(NULLIF(provider_snapshot->>'currency',''),'CNY')
		FROM payment_orders WHERE user_id=$1 AND order_type='balance' AND paid_at IS NOT NULL
		ORDER BY paid_at,id LIMIT $2`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]PaidOrder, 0, limit)
	for rows.Next() {
		var order PaidOrder
		if err := rows.Scan(&order.ID, &order.PayCents, &order.PaidAt, &order.Status, &order.Currency); err != nil {
			return nil, err
		}
		out = append(out, order)
	}
	return out, rows.Err()
}

type RefundReviewOrder struct {
	ID           int64
	Status       string
	RefundAmount string
}

func RefundOrdersForReview(ctx context.Context, tx Tx, userID int64) ([]RefundReviewOrder, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT id, status, COALESCE(refund_amount,0)::text
		FROM payment_orders
		WHERE user_id = $1
		  AND order_type = 'balance'
		  AND status IN ('REFUND_REQUESTED','REFUNDING','REFUND_PENDING','PARTIALLY_REFUNDED','REFUNDED')
          AND (status IN ('REFUND_REQUESTED','REFUNDING','REFUND_PENDING')
               OR COALESCE(updated_at,paid_at) >= (SELECT first_call_tracking_started_at FROM activity_config WHERE id=1)
               OR EXISTS(SELECT 1 FROM activity_lottery_orders lo WHERE lo.order_id=payment_orders.id))
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []RefundReviewOrder
	for rows.Next() {
		var order RefundReviewOrder
		if err := rows.Scan(&order.ID, &order.Status, &order.RefundAmount); err != nil {
			return nil, err
		}
		out = append(out, order)
	}
	return out, rows.Err()
}

func EnqueueRefundReviewUsers(ctx context.Context, tx Tx, since, scannedAt time.Time) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO activity_refund_sync_queue(user_id,updated_at,next_attempt_at)
 SELECT DISTINCT user_id,$2::timestamptz,$2::timestamptz FROM payment_orders WHERE order_type='balance' AND status IN ('REFUND_REQUESTED','REFUNDING','REFUND_PENDING','PARTIALLY_REFUNDED','REFUNDED') AND user_id IS NOT NULL AND COALESCE(updated_at,NOW()) >= $1
 ON CONFLICT(user_id) DO UPDATE SET updated_at=EXCLUDED.updated_at`, since, scannedAt)
	return err
}

type RewardMomentRecord struct {
	LedgerID     int64
	OwnerID      int64
	Kind         string
	CampaignID   *int64
	CampaignName string
	AmountCents  int64
	CreatedAt    time.Time
	AliasSecret  string
}

func RewardMomentRecords(ctx context.Context, tx Tx, viewerID int64, limit int) ([]RewardMomentRecord, error) {
	rows, err := tx.QueryContext(ctx, `SELECT m.ledger_id,m.user_id,m.kind,m.campaign_id,COALESCE(c.name,''),m.amount_cents,m.created_at,cfg.alias_secret
 FROM activity_reward_moments m
 JOIN activity_reward_moments_config cfg ON cfg.id=1 AND cfg.enabled AND $1=ANY(cfg.user_ids)
 JOIN activity_reward_sharing p ON p.user_id=m.user_id AND p.allowed AND m.created_at>=p.granted_at
 JOIN users u ON u.id=m.user_id AND u.status='active' AND u.deleted_at IS NULL
 LEFT JOIN activity_consumption_campaigns c ON c.id=m.campaign_id
 WHERE m.created_at>=NOW()-interval '7 days' AND m.created_at<=NOW() AND m.user_id<>$1
 AND CASE m.kind
 WHEN 'checkin' THEN activity_rollout_allowed('checkin',(SELECT checkin_rollout_percent FROM activity_config WHERE id=1),$1) AND activity_audience_allowed('checkin',m.user_id)
 WHEN 'lottery' THEN activity_audience_allowed('lottery',$1) AND activity_audience_allowed('lottery',m.user_id)
 WHEN 'consumption' THEN c.published_at IS NOT NULL AND activity_rollout_allowed('consumption',c.rollout_percent,$1) AND activity_audience_allowed('consumption',m.user_id)
 ELSE FALSE END
 ORDER BY m.created_at DESC,m.ledger_id DESC LIMIT $2`, viewerID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]RewardMomentRecord, 0, limit)
	for rows.Next() {
		var item RewardMomentRecord
		if err := rows.Scan(&item.LedgerID, &item.OwnerID, &item.Kind, &item.CampaignID, &item.CampaignName, &item.AmountCents, &item.CreatedAt, &item.AliasSecret); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func GlobalRewardReviewBlocked(ctx context.Context, tx Tx, userID int64) (bool, error) {
	var blocked bool
	err := Scan(ctx, tx, `SELECT EXISTS(SELECT 1 FROM activity_consumption_entitlements WHERE user_id=$1 AND state='review') OR EXISTS(SELECT 1 FROM activity_refund_reviews WHERE user_id=$1 AND review_status='pending') OR EXISTS(SELECT 1 FROM activity_wallets WHERE user_id=$1 AND recovery_debt_cents>0) OR EXISTS(SELECT 1 FROM payment_orders WHERE user_id=$1 AND status IN ('REFUND_REQUESTED','REFUNDING','REFUND_PENDING'))`, []any{userID}, &blocked)
	return blocked, err
}

func WalletReviewBlocked(ctx context.Context, tx Tx, userID int64) (bool, error) {
	var blocked bool
	err := Scan(ctx, tx, `SELECT EXISTS(SELECT 1 FROM activity_consumption_entitlements WHERE user_id=$1 AND state='review') OR EXISTS(SELECT 1 FROM activity_refund_reviews WHERE user_id=$1 AND review_status='pending') OR EXISTS(SELECT 1 FROM payment_orders WHERE user_id=$1 AND order_type='balance' AND status IN ('REFUND_REQUESTED','REFUNDING','REFUND_PENDING')) OR COALESCE((SELECT SUM(delta_tickets) FROM activity_lottery_ticket_ledger WHERE user_id=$1),0)<0`, []any{userID}, &blocked)
	return blocked, err
}

func CheckinReviewBlocked(ctx context.Context, tx Tx, userID int64) (bool, error) {
	var blocked bool
	err := Scan(ctx, tx, `SELECT EXISTS(SELECT 1 FROM activity_refund_reviews WHERE user_id=$1 AND review_status='pending') OR EXISTS(SELECT 1 FROM activity_wallets WHERE user_id=$1 AND recovery_debt_cents>0) OR EXISTS(SELECT 1 FROM payment_orders WHERE user_id=$1 AND status IN ('REFUND_REQUESTED','REFUNDING','REFUND_PENDING'))`, []any{userID}, &blocked)
	return blocked, err
}

func DrawReviewBlocked(ctx context.Context, tx Tx, userID int64) (bool, error) {
	var blocked bool
	err := Scan(ctx, tx, `
		SELECT EXISTS(SELECT 1 FROM activity_consumption_entitlements WHERE user_id=$1 AND state='review')
		    OR EXISTS(SELECT 1 FROM activity_refund_reviews WHERE user_id=$1 AND review_status='pending')
		    OR EXISTS(SELECT 1 FROM activity_wallets WHERE user_id=$1 AND recovery_debt_cents>0)
		OR EXISTS(SELECT 1 FROM payment_orders WHERE user_id=$1 AND order_type='balance' AND status IN ('REFUND_REQUESTED','REFUNDING','REFUND_PENDING'))
	`, []any{userID}, &blocked)
	return blocked, err
}

type LotteryGrantCandidate struct {
	OrderID   int64
	PayCents  int64
	SettledAt time.Time
}

func LotteryGrantCandidates(ctx context.Context, tx Tx, userID int64, earliest time.Time) ([]LotteryGrantCandidate, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT o.id, ROUND(o.pay_amount * 100)::bigint, COALESCE(o.completed_at, o.paid_at)
		FROM payment_orders o
		WHERE o.user_id = $1
		  AND o.order_type = 'balance'
		  AND o.status IN ('COMPLETED','PARTIALLY_REFUNDED')
		  AND COALESCE(o.completed_at, o.paid_at) >= $2
		  AND COALESCE(NULLIF(UPPER(TRIM(o.provider_snapshot->>'currency')), ''), 'CNY') = 'CNY'
		  AND NOT EXISTS (SELECT 1 FROM activity_lottery_orders lo WHERE lo.order_id = o.id)
		ORDER BY o.id ASC
	`, userID, earliest)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []LotteryGrantCandidate
	for rows.Next() {
		var candidate LotteryGrantCandidate
		if err := rows.Scan(&candidate.OrderID, &candidate.PayCents, &candidate.SettledAt); err != nil {
			return nil, err
		}
		out = append(out, candidate)
	}
	return out, rows.Err()
}

type LotteryRefundCandidate struct {
	OrderID          int64
	UnitPayCents     int
	EffectiveTickets int
	Status           string
	OrderAmount      float64
	PayAmount        float64
	RefundAmount     float64
}

func LotteryRefundCandidates(ctx context.Context, tx Tx, userID int64) ([]LotteryRefundCandidate, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT lo.order_id, lo.unit_pay_cents, lo.effective_tickets,
		       o.status, COALESCE(o.amount, 0)::float8, COALESCE(o.pay_amount, 0)::float8, COALESCE(o.refund_amount, 0)::float8
		FROM activity_lottery_orders lo
		JOIN payment_orders o ON o.id = lo.order_id
		WHERE lo.user_id = $1
		  AND lo.effective_tickets > 0
		  AND o.status IN ('PARTIALLY_REFUNDED', 'REFUNDED')
		FOR UPDATE OF lo
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []LotteryRefundCandidate
	for rows.Next() {
		var candidate LotteryRefundCandidate
		if err := rows.Scan(&candidate.OrderID, &candidate.UnitPayCents, &candidate.EffectiveTickets, &candidate.Status, &candidate.OrderAmount, &candidate.PayAmount, &candidate.RefundAmount); err != nil {
			return nil, err
		}
		out = append(out, candidate)
	}
	return out, rows.Err()
}

func LotterySyncUserIDs(ctx context.Context, tx Tx, earliest time.Time, limit int) ([]int64, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT DISTINCT user_id FROM (
			SELECT o.user_id
			FROM payment_orders o
			WHERE o.order_type = 'balance'
			  AND o.status IN ('COMPLETED','PARTIALLY_REFUNDED')
			  AND COALESCE(o.completed_at, o.paid_at) >= $1
			  AND COALESCE(NULLIF(UPPER(TRIM(o.provider_snapshot->>'currency')), ''), 'CNY') = 'CNY'
			  AND activity_audience_allowed('lottery', o.user_id)
			  AND NOT EXISTS (SELECT 1 FROM activity_lottery_orders lo WHERE lo.order_id = o.id)
			  AND EXISTS (
			      SELECT 1 FROM activity_lottery_windows w
			      WHERE COALESCE(o.completed_at, o.paid_at) >= w.opened_at
			        AND (w.closed_at IS NULL OR COALESCE(o.completed_at, o.paid_at) < w.closed_at)
			  )
			LIMIT $2
		) pending_grants
		UNION
		SELECT DISTINCT lo.user_id
		FROM activity_lottery_orders lo
		JOIN payment_orders o ON o.id = lo.order_id
		WHERE lo.effective_tickets > 0
		  AND o.status IN ('PARTIALLY_REFUNDED', 'REFUNDED')
		LIMIT $2
	`, earliest, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []int64
	for rows.Next() {
		var userID int64
		if err := rows.Scan(&userID); err != nil {
			return nil, err
		}
		out = append(out, userID)
	}
	return out, rows.Err()
}

func LotteryTiersLocked(ctx context.Context, tx Tx) (bool, error) {
	var locked bool
	err := Scan(ctx, tx, `SELECT EXISTS(SELECT 1 FROM activity_lottery_windows WHERE closed_at IS NULL)
 OR EXISTS(SELECT 1 FROM activity_lottery_ticket_ledger GROUP BY user_id HAVING SUM(delta_tickets)>0)
 OR EXISTS(SELECT 1 FROM payment_orders o JOIN activity_lottery_windows w ON COALESCE(o.completed_at,o.paid_at)>=w.opened_at AND (w.closed_at IS NULL OR COALESCE(o.completed_at,o.paid_at)<w.closed_at) WHERE o.order_type='balance' AND o.status IN ('COMPLETED','PARTIALLY_REFUNDED') AND o.pay_amount*100>=w.unit_pay_cents AND COALESCE(NULLIF(UPPER(TRIM(o.provider_snapshot->>'currency')),''),'CNY')='CNY' AND NOT EXISTS(SELECT 1 FROM activity_lottery_orders lo WHERE lo.order_id=o.id))`, nil, &locked)
	return locked, err
}
