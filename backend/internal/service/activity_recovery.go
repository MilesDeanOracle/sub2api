package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

var ErrActivityRecoveryInsufficient = errors.New("余额不足以扣回退款对应的活动奖励，请补足余额后重试或联系管理员审核")
var ErrActivityRecoveryReview = errors.New("活动奖励来源需要人工核对，退款尚未执行")

// Both the activity SQL transaction and the payment ent transaction implement this.
// All callers acquire users -> activity_wallets -> payment_orders, in that order.
type activityAccountingDB interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func activityScan(ctx context.Context, db activityAccountingDB, query string, args []any, dest ...any) error {
	rows, err := db.QueryContext(ctx, query, args...)
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

func lockActivityWalletTx(ctx context.Context, tx activityAccountingDB, uid int64) error {
	var user int64
	if err := activityScan(ctx, tx, `SELECT id FROM users WHERE id=$1 FOR UPDATE`, []any{uid}, &user); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO activity_wallets(user_id) VALUES($1) ON CONFLICT DO NOTHING`, uid); err != nil {
		return err
	}
	var id int64
	return activityScan(ctx, tx, `SELECT user_id FROM activity_wallets WHERE user_id=$1 FOR UPDATE`, []any{uid}, &id)
}

// recoverActivityReward holds a refund's reward before the gateway is called.
// A normal refund must still have its principal available after reward recovery.
// Explicit forced refunds and rejected consumption reviews retain any shortfall
// as visible debt. Gross earning/budget ledgers are never rewritten.
func recoverActivityReward(ctx context.Context, tx activityAccountingDB, uid int64, kind, source string, amount int64, principal decimal.Decimal, force bool, state string) error {
	if amount <= 0 {
		return nil
	}
	var previous string
	err := activityScan(ctx, tx, `SELECT state FROM activity_reward_recoveries WHERE kind=$1 AND source_id=$2`, []any{kind, source}, &previous)
	if err == nil && previous != "released" {
		return nil
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	var wallet int64
	var balanceRaw, frozenRaw, giftRaw string
	if err := activityScan(ctx, tx, `SELECT w.available_cents,u.balance::text,COALESCE(u.frozen_balance,0)::text,w.unabsorbed_gift::text FROM activity_wallets w JOIN users u ON u.id=w.user_id WHERE w.user_id=$1`, []any{uid}, &wallet, &balanceRaw, &frozenRaw, &giftRaw); err != nil {
		return err
	}
	balance, err := decimal.NewFromString(balanceRaw)
	if err != nil {
		return err
	}
	frozen, err := decimal.NewFromString(frozenRaw)
	if err != nil {
		return err
	}
	gift, err := decimal.NewFromString(giftRaw)
	if err != nil {
		return err
	}
	fromWallet := min(wallet, amount)
	needed := amount - fromWallet
	available := balance.Sub(frozen).Sub(principal).Mul(decimal.NewFromInt(100)).Floor().IntPart()
	if available < 0 {
		available = 0
	}
	if needed > available && !force {
		return ErrActivityRecoveryInsufficient
	}
	fromBalance := min(needed, available)
	debt := needed - fromBalance
	absorbed := decimal.Min(gift, decimal.NewFromInt(fromBalance).Div(decimal.NewFromInt(100)))
	if fromBalance > 0 {
		if _, err := tx.ExecContext(ctx, `UPDATE users SET balance=balance-($2::numeric/100),updated_at=NOW() WHERE id=$1`, uid, fromBalance); err != nil {
			return err
		}
	}
	var after int64
	if err := activityScan(ctx, tx, `UPDATE activity_wallets SET available_cents=available_cents-$2,recovery_debt_cents=recovery_debt_cents+$3,unabsorbed_gift=unabsorbed_gift-$4::numeric,updated_at=NOW() WHERE user_id=$1 RETURNING available_cents`, []any{uid, fromWallet, debt, absorbed.String()}, &after); err != nil {
		return err
	}
	// A released refund may be retried; the recovery row remains the durable key.
	if _, err := tx.ExecContext(ctx, `INSERT INTO activity_reward_recoveries(kind,source_id,user_id,amount_cents,wallet_cents,balance_cents,debt_cents,absorbed_gift,state) VALUES($1,$2,$3,$4,$5,$6,$7,$8::numeric,$9) ON CONFLICT(kind,source_id) DO UPDATE SET amount_cents=EXCLUDED.amount_cents,wallet_cents=EXCLUDED.wallet_cents,balance_cents=EXCLUDED.balance_cents,debt_cents=EXCLUDED.debt_cents,absorbed_gift=EXCLUDED.absorbed_gift,state=EXCLUDED.state,updated_at=NOW()`, kind, source, uid, amount, fromWallet, fromBalance, debt, absorbed.String(), state); err != nil {
		return err
	}
	// One aggregate adjustment, with the exact split retained above for rollback.
	_, err = tx.ExecContext(ctx, `INSERT INTO activity_wallet_ledger(user_id,direction,kind,source_id,amount_cents,wallet_after_cents) VALUES($1,'revoke',$2,$3,$4,$5) ON CONFLICT(direction,kind,source_id) DO UPDATE SET amount_cents=EXCLUDED.amount_cents,wallet_after_cents=EXCLUDED.wallet_after_cents,created_at=NOW()`, uid, kind, fmt.Sprintf("%s:%d", source, time.Now().UnixNano()), amount, after)
	return err
}

func releaseLotteryRecovery(ctx context.Context, tx activityAccountingDB, uid, oid int64) error {
	rows, err := tx.QueryContext(ctx, `SELECT source_id FROM activity_reward_recoveries WHERE kind='lottery_refund' AND source_id LIKE $1 AND user_id=$2 AND state='held' ORDER BY source_id`, fmt.Sprintf("%d:%%", oid), uid)
	if err != nil {
		return err
	}
	var sources []string
	for rows.Next() {
		var source string
		if err := rows.Scan(&source); err != nil {
			rows.Close()
			return err
		}
		sources = append(sources, source)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, source := range sources {
		if err := releaseLotteryRecoverySource(ctx, tx, uid, source); err != nil {
			return err
		}
	}
	return nil
}

func releaseLotteryRecoverySource(ctx context.Context, tx activityAccountingDB, uid int64, source string) error {
	var amount, wallet, balance, debt int64
	var gift string
	err := activityScan(ctx, tx, `UPDATE activity_reward_recoveries SET state='released',updated_at=NOW() WHERE kind='lottery_refund' AND source_id=$1 AND state='held' RETURNING amount_cents,wallet_cents,balance_cents,debt_cents,absorbed_gift::text`, []any{source}, &amount, &wallet, &balance, &debt, &gift)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if balance > 0 {
		if _, err := tx.ExecContext(ctx, `UPDATE users SET balance=balance+($2::numeric/100),updated_at=NOW() WHERE id=$1`, uid, balance); err != nil {
			return err
		}
	}
	var after int64
	if err := activityScan(ctx, tx, `UPDATE activity_wallets SET available_cents=available_cents+$2,recovery_debt_cents=recovery_debt_cents-$3,unabsorbed_gift=unabsorbed_gift+$4::numeric,updated_at=NOW() WHERE user_id=$1 RETURNING available_cents`, []any{uid, wallet, debt, gift}, &after); err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO activity_wallet_ledger(user_id,direction,kind,source_id,amount_cents,wallet_after_cents) VALUES($1,'credit','refund_restore',$2,$3,$4) ON CONFLICT(direction,kind,source_id) DO UPDATE SET amount_cents=EXCLUDED.amount_cents,wallet_after_cents=EXCLUDED.wallet_after_cents,created_at=NOW()`, uid, fmt.Sprintf("%s:%d", source, time.Now().UnixNano()), amount, after)
	return err
}

// Replay the original ticket ledger to attribute existing draws without inventing
// an order association. Lots are ranges, so large recharges do not allocate one
// object per ticket. Ambiguous/inconsistent legacy data fails closed.
func attributeLotteryDraws(ctx context.Context, tx activityAccountingDB, uid int64) error {
	var missing bool
	if err := activityScan(ctx, tx, `SELECT EXISTS(SELECT 1 FROM activity_lottery_draws WHERE user_id=$1 AND order_id IS NULL)`, []any{uid}, &missing); err != nil {
		return err
	}
	if !missing {
		return nil
	}
	type lot struct {
		id          int64
		next, limit int
	}
	type event struct {
		kind, source string
		order        sql.NullInt64
		delta        int
	}
	rows, err := tx.QueryContext(ctx, `SELECT source_type,source_id,order_id,delta_tickets FROM activity_lottery_ticket_ledger WHERE user_id=$1 ORDER BY id`, uid)
	if err != nil {
		return err
	}
	var events []event
	for rows.Next() {
		var e event
		if err := rows.Scan(&e.kind, &e.source, &e.order, &e.delta); err != nil {
			rows.Close()
			return err
		}
		events = append(events, e)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	lots := []*lot{}
	byOrder := map[int64]*lot{}
	debt := 0
	for _, e := range events {
		switch e.kind {
		case "order_grant":
			if !e.order.Valid {
				return ErrActivityRecoveryReview
			}
			skip := min(debt, e.delta)
			debt -= skip
			l := &lot{id: e.order.Int64, next: skip + 1, limit: e.delta}
			lots = append(lots, l)
			byOrder[l.id] = l
		case "order_refund":
			l := byOrder[e.order.Int64]
			if l == nil {
				return ErrActivityRecoveryReview
			}
			old := l.limit
			l.limit += e.delta
			debt += max(0, l.next-1-l.limit) - max(0, l.next-1-old)
		case "refund_recovery":
			debt = max(0, debt-e.delta)
		case "draw":
			batch := strings.TrimPrefix(e.source, fmt.Sprintf("%d:", uid))
			drawRows, err := tx.QueryContext(ctx, `SELECT id FROM activity_lottery_draws WHERE user_id=$1 AND batch_key=$2 ORDER BY draw_index`, uid, batch)
			if err != nil {
				return err
			}
			var ids []int64
			for drawRows.Next() {
				var id int64
				if err := drawRows.Scan(&id); err != nil {
					drawRows.Close()
					return err
				}
				ids = append(ids, id)
			}
			err = drawRows.Err()
			drawRows.Close()
			if err != nil {
				return err
			}
			if len(ids) != -e.delta {
				return ErrActivityRecoveryReview
			}
			for _, id := range ids {
				var source *lot
				for _, l := range lots {
					if l.next <= l.limit {
						source = l
						break
					}
				}
				if source == nil {
					return ErrActivityRecoveryReview
				}
				if _, err := tx.ExecContext(ctx, `UPDATE activity_lottery_draws SET order_id=$2,order_ticket_index=$3 WHERE id=$1 AND order_id IS NULL`, id, source.id, source.next); err != nil {
					return err
				}
				source.next++
			}
		}
	}
	if err := activityScan(ctx, tx, `SELECT EXISTS(SELECT 1 FROM activity_lottery_draws WHERE user_id=$1 AND order_id IS NULL)`, []any{uid}, &missing); err != nil {
		return err
	}
	if missing {
		return ErrActivityRecoveryReview
	}
	return nil
}

func reserveLotteryRefund(ctx context.Context, tx activityAccountingDB, uid, oid int64, refund, principal float64, force bool) error {
	if err := attributeLotteryDraws(ctx, tx, uid); err != nil {
		return err
	}
	var unit, effective int
	var amount, pay float64
	var orderStatus string
	err := activityScan(ctx, tx, `SELECT lo.unit_pay_cents,lo.effective_tickets,COALESCE(o.amount,0),COALESCE(o.pay_amount,0),o.status FROM activity_lottery_orders lo JOIN payment_orders o ON o.id=lo.order_id WHERE lo.order_id=$1 AND lo.user_id=$2`, []any{oid, uid}, &unit, &effective, &amount, &pay, &orderStatus)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	status := "PARTIALLY_REFUNDED"
	if refund >= amount || orderStatus == OrderStatusRefunded {
		status = "REFUNDED"
	}
	allowed, _ := calculateLotteryRefundRemainingTickets(status, amount, pay, refund, unit, effective)
	var recovery int64
	if err := activityScan(ctx, tx, `SELECT COALESCE(SUM(reward_cents),0) FROM activity_lottery_draws WHERE user_id=$1 AND order_id=$2 AND order_ticket_index>$3 AND revoked_at IS NULL`, []any{uid, oid, allowed}, &recovery); err != nil {
		return err
	}
	return recoverActivityReward(ctx, tx, uid, "lottery_refund", fmt.Sprintf("%d:%d", oid, allowed), recovery, decimal.NewFromFloat(principal), force, "held")
}

func settleLotteryRefund(ctx context.Context, tx activityAccountingDB, uid, oid int64) error {
	var state string
	err := activityScan(ctx, tx, `UPDATE activity_reward_recoveries SET state='settled',updated_at=NOW() WHERE kind='lottery_refund' AND source_id LIKE $1 AND user_id=$2 AND state='held' RETURNING state`, []any{fmt.Sprintf("%d:%%", oid), uid}, &state)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	var unit, effective int
	var amount, pay, refund float64
	var status string
	if err := activityScan(ctx, tx, `SELECT lo.unit_pay_cents,lo.effective_tickets,COALESCE(o.amount,0),COALESCE(o.pay_amount,0),COALESCE(o.refund_amount,0),o.status FROM activity_lottery_orders lo JOIN payment_orders o ON o.id=lo.order_id WHERE lo.order_id=$1`, []any{oid}, &unit, &effective, &amount, &pay, &refund, &status); err != nil {
		return err
	}
	allowed, _ := calculateLotteryRefundRemainingTickets(status, amount, pay, refund, unit, effective)
	result, err := tx.ExecContext(ctx, `UPDATE activity_lottery_draws SET revoked_at=NOW() WHERE user_id=$1 AND order_id=$2 AND order_ticket_index>$3 AND revoked_at IS NULL`, uid, oid, allowed)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count > 0 {
		_, err = tx.ExecContext(ctx, `INSERT INTO activity_lottery_ticket_ledger(user_id,source_type,source_id,order_id,delta_tickets) VALUES($1,'refund_recovery',$2,$3,$4) ON CONFLICT(source_type,source_id) DO NOTHING`, uid, fmt.Sprintf("%d:%d", oid, allowed), oid, count)
	}
	return err
}

// Also handles provider-side refunds observed by the worker. A shortage is
// retained as debt rather than silently treating an already paid reward as valid.
func settleUserRefundRewardsTx(ctx context.Context, tx activityAccountingDB, uid int64) error {
	rows, err := tx.QueryContext(ctx, `SELECT id,COALESCE(refund_amount,0)::float8 FROM payment_orders WHERE user_id=$1 AND order_type='balance' AND status IN ('REFUNDED','PARTIALLY_REFUNDED') ORDER BY id`, uid)
	if err != nil {
		return err
	}
	type order struct {
		id     int64
		amount float64
	}
	var orders []order
	for rows.Next() {
		var o order
		if err := rows.Scan(&o.id, &o.amount); err != nil {
			rows.Close()
			return err
		}
		orders = append(orders, o)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, o := range orders {
		if err := reserveLotteryRefund(ctx, tx, uid, o.id, o.amount, 0, true); err != nil {
			return err
		}
		if err := settleLotteryRefund(ctx, tx, uid, o.id); err != nil {
			return err
		}
	}
	return nil
}

func collectActivityRecoveryDebt(ctx context.Context, tx activityAccountingDB, uid int64) error {
	var debt, wallet int64
	var balanceRaw, frozenRaw string
	if err := activityScan(ctx, tx, `SELECT w.recovery_debt_cents,w.available_cents,u.balance::text,COALESCE(u.frozen_balance,0)::text FROM activity_wallets w JOIN users u ON u.id=w.user_id WHERE w.user_id=$1`, []any{uid}, &debt, &wallet, &balanceRaw, &frozenRaw); err != nil {
		return err
	}
	if debt <= 0 {
		return nil
	}
	balance, err := decimal.NewFromString(balanceRaw)
	if err != nil {
		return err
	}
	frozen, err := decimal.NewFromString(frozenRaw)
	if err != nil {
		return err
	}
	fromWallet := min(wallet, debt)
	fromBalance := debt - fromWallet
	if balance.Sub(frozen).Mul(decimal.NewFromInt(100)).Floor().IntPart() < fromBalance {
		return ErrActivityRecoveryInsufficient
	}
	var after int64
	if err := activityScan(ctx, tx, `UPDATE activity_wallets SET available_cents=available_cents-$2,recovery_debt_cents=0,unabsorbed_gift=GREATEST(0,unabsorbed_gift-$3::numeric/100),updated_at=NOW() WHERE user_id=$1 RETURNING available_cents`, []any{uid, fromWallet, fromBalance}, &after); err != nil {
		return err
	}
	if fromBalance > 0 {
		if _, err := tx.ExecContext(ctx, `UPDATE users SET balance=balance-$2::numeric/100,updated_at=NOW() WHERE id=$1`, uid, fromBalance); err != nil {
			return err
		}
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO activity_wallet_ledger(user_id,direction,kind,source_id,amount_cents,wallet_after_cents) VALUES($1,'revoke','debt_repayment',$2,$3,$4)`, uid, fmt.Sprintf("%d:%d", uid, time.Now().UnixNano()), debt, after)
	return err
}
