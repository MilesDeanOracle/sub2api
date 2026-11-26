package service

import (
	"context"
	"database/sql"
	"encoding/json"
)

func allocateLotteryDraw(ctx context.Context, tx *sql.Tx, uid, drawID int64) error {
	var order int64
	var index int
	err := tx.QueryRowContext(ctx, `SELECT lo.order_id,COALESCE(MAX(d.order_ticket_index),0)+1 FROM activity_lottery_orders lo JOIN activity_lottery_ticket_ledger grant_entry ON grant_entry.order_id=lo.order_id AND grant_entry.source_type='order_grant' LEFT JOIN activity_lottery_draws d ON d.order_id=lo.order_id WHERE lo.user_id=$1 GROUP BY lo.order_id,grant_entry.id,lo.effective_tickets HAVING lo.effective_tickets>COALESCE(MAX(d.order_ticket_index),0) ORDER BY grant_entry.id LIMIT 1`, uid).Scan(&order, &index)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `UPDATE activity_lottery_draws SET order_id=$2,order_ticket_index=$3 WHERE id=$1`, drawID, order, index)
	return err
}

// Close the issuance interval at the moment a payout exhausts usable budget.
// Historical orders retain the rules of their original settlement interval.
func pauseDepletedLotteryTx(ctx context.Context, tx *sql.Tx) error {
	var enabled bool
	var budget int64
	var raw []byte
	if err := tx.QueryRowContext(ctx, `SELECT enabled,budget_cents,tiers FROM activity_lottery_config WHERE id=1 FOR UPDATE`).Scan(&enabled, &budget, &raw); err != nil {
		return err
	}
	if !enabled {
		return nil
	}
	var tiers []LotteryTier
	if err := json.Unmarshal(raw, &tiers); err != nil {
		return err
	}
	_, maxReward, _, _, err := parseAndValidateLotteryTiers(tiers)
	if err != nil {
		return err
	}
	var spent int64
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(SUM(amount_cents),0) FROM activity_reward_ledger WHERE kind='lottery'`).Scan(&spent); err != nil {
		return err
	}
	if budget-spent >= int64(maxReward) {
		return nil
	}
	if _, err := tx.ExecContext(ctx, `UPDATE activity_lottery_windows SET closed_at=clock_timestamp() WHERE closed_at IS NULL`); err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `UPDATE activity_lottery_config SET enabled=false,updated_at=NOW() WHERE id=1`)
	return err
}

func lotteryTiersLocked(ctx context.Context, db activityAccountingDB) (bool, error) {
	var locked bool
	err := activityScan(ctx, db, `SELECT EXISTS(SELECT 1 FROM activity_lottery_windows WHERE closed_at IS NULL)
 OR EXISTS(SELECT 1 FROM activity_lottery_ticket_ledger GROUP BY user_id HAVING SUM(delta_tickets)>0)
 OR EXISTS(SELECT 1 FROM payment_orders o JOIN activity_lottery_windows w ON COALESCE(o.completed_at,o.paid_at)>=w.opened_at AND (w.closed_at IS NULL OR COALESCE(o.completed_at,o.paid_at)<w.closed_at) WHERE o.order_type='balance' AND o.status IN ('COMPLETED','PARTIALLY_REFUNDED') AND o.pay_amount*100>=w.unit_pay_cents AND COALESCE(NULLIF(UPPER(TRIM(o.provider_snapshot->>'currency')),''),'CNY')='CNY' AND NOT EXISTS(SELECT 1 FROM activity_lottery_orders lo WHERE lo.order_id=o.id))`, nil, &locked)
	return locked, err
}
