package activitystore

import (
	"context"
	"database/sql"
	"fmt"
)

type TicketEvent struct {
	Kind, Source string
	Order        sql.NullInt64
	Delta        int
}

func HasUnattributedDraws(ctx context.Context, tx Tx, uid int64) (bool, error) {
	var missing bool
	err := Scan(ctx, tx, `SELECT EXISTS(SELECT 1 FROM activity_lottery_draws WHERE user_id=$1 AND order_id IS NULL)`, []any{uid}, &missing)
	return missing, err
}
func TicketEvents(ctx context.Context, tx Tx, uid int64) ([]TicketEvent, error) {
	rows, err := tx.QueryContext(ctx, `SELECT source_type,source_id,order_id,delta_tickets FROM activity_lottery_ticket_ledger WHERE user_id=$1 ORDER BY id`, uid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var events []TicketEvent
	for rows.Next() {
		var e TicketEvent
		if err := rows.Scan(&e.Kind, &e.Source, &e.Order, &e.Delta); err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	return events, rows.Err()
}
func DrawIDs(ctx context.Context, tx Tx, uid int64, batch string) ([]int64, error) {
	rows, err := tx.QueryContext(ctx, `SELECT id FROM activity_lottery_draws WHERE user_id=$1 AND batch_key=$2 ORDER BY draw_index`, uid, batch)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
func AttributeDraw(ctx context.Context, tx Tx, draw, order int64, index int) error {
	_, err := tx.ExecContext(ctx, `UPDATE activity_lottery_draws SET order_id=$2,order_ticket_index=$3 WHERE id=$1 AND order_id IS NULL`, draw, order, index)
	return err
}
func AllocateDraw(ctx context.Context, tx Tx, uid, draw int64) error {
	var order int64
	var index int
	if err := Scan(ctx, tx, `SELECT lo.order_id,COALESCE(MAX(d.order_ticket_index),0)+1 FROM activity_lottery_orders lo JOIN activity_lottery_ticket_ledger grant_entry ON grant_entry.order_id=lo.order_id AND grant_entry.source_type='order_grant' LEFT JOIN activity_lottery_draws d ON d.order_id=lo.order_id WHERE lo.user_id=$1 GROUP BY lo.order_id,grant_entry.id,lo.effective_tickets HAVING lo.effective_tickets>COALESCE(MAX(d.order_ticket_index),0) ORDER BY grant_entry.id LIMIT 1`, []any{uid}, &order, &index); err != nil {
		return err
	}
	return AttributeDraw(ctx, tx, draw, order, index)
}
func PendingRecoverySources(ctx context.Context, tx Tx, uid, oid int64) ([]string, error) {
	rows, err := tx.QueryContext(ctx, `SELECT source_id FROM activity_reward_recoveries WHERE kind='lottery_refund' AND source_id LIKE $1 AND user_id=$2 AND state='held' ORDER BY source_id`, fmt.Sprintf("%d:%%", oid), uid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var sources []string
	for rows.Next() {
		var source string
		if err := rows.Scan(&source); err != nil {
			return nil, err
		}
		sources = append(sources, source)
	}
	return sources, rows.Err()
}

type RefundOrder struct {
	ID                  int64
	Unit, Effective     int
	Amount, Pay, Refund float64
	Status              string
}

func LotteryRefundOrder(ctx context.Context, tx Tx, uid, oid int64) (RefundOrder, error) {
	var o RefundOrder
	o.ID = oid
	err := Scan(ctx, tx, `SELECT lo.unit_pay_cents,lo.effective_tickets,COALESCE(o.amount,0),COALESCE(o.pay_amount,0),COALESCE(o.refund_amount,0),o.status FROM activity_lottery_orders lo JOIN payment_orders o ON o.id=lo.order_id WHERE lo.order_id=$1 AND lo.user_id=$2`, []any{oid, uid}, &o.Unit, &o.Effective, &o.Amount, &o.Pay, &o.Refund, &o.Status)
	return o, err
}
func PendingDrawReward(ctx context.Context, tx Tx, uid, oid int64, allowed int) (int64, error) {
	var amount int64
	err := Scan(ctx, tx, `SELECT COALESCE(SUM(reward_cents),0) FROM activity_lottery_draws WHERE user_id=$1 AND order_id=$2 AND order_ticket_index>$3 AND revoked_at IS NULL`, []any{uid, oid, allowed}, &amount)
	return amount, err
}
func MarkRecoverySettled(ctx context.Context, tx Tx, uid, oid int64) (bool, error) {
	res, err := tx.ExecContext(ctx, `UPDATE activity_reward_recoveries SET state='settled',updated_at=NOW() WHERE kind='lottery_refund' AND source_id LIKE $1 AND user_id=$2 AND state='held'`, fmt.Sprintf("%d:%%", oid), uid)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n > 0, err
}
func RevokeDraws(ctx context.Context, tx Tx, uid, oid int64, allowed int) error {
	res, err := tx.ExecContext(ctx, `UPDATE activity_lottery_draws SET revoked_at=NOW() WHERE user_id=$1 AND order_id=$2 AND order_ticket_index>$3 AND revoked_at IS NULL`, uid, oid, allowed)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil || n == 0 {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO activity_lottery_ticket_ledger(user_id,source_type,source_id,order_id,delta_tickets) VALUES($1,'refund_recovery',$2,$3,$4) ON CONFLICT(source_type,source_id) DO NOTHING`, uid, fmt.Sprintf("%d:%d", oid, allowed), oid, n)
	return err
}
func RefundedOrders(ctx context.Context, tx Tx, uid int64) ([]RefundOrder, error) {
	rows, err := tx.QueryContext(ctx, `SELECT id,COALESCE(refund_amount,0)::float8 FROM payment_orders WHERE user_id=$1 AND order_type='balance' AND status IN ('REFUNDED','PARTIALLY_REFUNDED') ORDER BY id`, uid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var orders []RefundOrder
	for rows.Next() {
		var o RefundOrder
		if err := rows.Scan(&o.ID, &o.Refund); err != nil {
			return nil, err
		}
		orders = append(orders, o)
	}
	return orders, rows.Err()
}
