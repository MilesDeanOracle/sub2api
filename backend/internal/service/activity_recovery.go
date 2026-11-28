package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/Wei-Shaw/sub2api/internal/repository/activitystore"
	"strings"

	"github.com/shopspring/decimal"
)

var ErrActivityRecoveryInsufficient = errors.New("余额不足以扣回退款对应的活动奖励，请补足余额后重试或联系管理员审核")
var ErrActivityRecoveryReview = errors.New("活动奖励来源需要人工核对，退款尚未执行")

// Both SQL and Ent transactions implement this persistence boundary.
type activityAccountingDB = activitystore.Tx

// recoverActivityReward holds a refund's reward before the gateway is called.
// A normal refund must still have its principal available after reward recovery.
// Explicit forced refunds and rejected consumption reviews retain any shortfall
// as visible debt. Gross earning/budget ledgers are never rewritten.
func recoverActivityReward(ctx context.Context, tx activityAccountingDB, uid int64, kind, source string, amount int64, principal decimal.Decimal, force bool, state string) error {
	if amount <= 0 {
		return nil
	}
	previous, err := activitystore.RecoveryState(ctx, tx, kind, source)
	if err == nil && previous != "released" {
		return nil
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	funds, err := activitystore.LoadFunds(ctx, tx, uid)
	if err != nil {
		return err
	}
	wallet, balanceRaw, frozenRaw, giftRaw := funds.Wallet, funds.Balance, funds.Frozen, funds.Gift
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
		if err := activitystore.AdjustBalance(ctx, tx, uid, decimal.NewFromInt(fromBalance).Div(decimal.NewFromInt(100)).Neg().String()); err != nil {
			return err
		}
	}
	return activitystore.ApplyRecovery(ctx, tx, activitystore.Recovery{Kind: kind, Source: source, UserID: uid, Amount: amount, Wallet: fromWallet, Balance: fromBalance, Debt: debt, Gift: absorbed.String(), State: state})
}

func releaseLotteryRecovery(ctx context.Context, tx activityAccountingDB, uid, oid int64) error {
	sources, err := activitystore.PendingRecoverySources(ctx, tx, uid, oid)
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
	return activitystore.ReleaseRecovery(ctx, tx, uid, source)
}

// Replay the original ticket ledger to attribute existing draws without inventing
// an order association. Lots are ranges, so large recharges do not allocate one
// object per ticket. Ambiguous/inconsistent legacy data fails closed.
func attributeLotteryDraws(ctx context.Context, tx activityAccountingDB, uid int64) error {
	missing, err := activitystore.HasUnattributedDraws(ctx, tx, uid)
	if err != nil {
		return err
	}
	if !missing {
		return nil
	}
	type lot struct {
		id          int64
		next, limit int
	}
	events, err := activitystore.TicketEvents(ctx, tx, uid)
	if err != nil {
		return err
	}
	lots := []*lot{}
	byOrder := map[int64]*lot{}
	debt := 0
	for _, e := range events {
		switch e.Kind {
		case "order_grant":
			if !e.Order.Valid {
				return ErrActivityRecoveryReview
			}
			skip := min(debt, e.Delta)
			debt -= skip
			l := &lot{id: e.Order.Int64, next: skip + 1, limit: e.Delta}
			lots = append(lots, l)
			byOrder[l.id] = l
		case "order_refund":
			l := byOrder[e.Order.Int64]
			if l == nil {
				return ErrActivityRecoveryReview
			}
			old := l.limit
			l.limit += e.Delta
			debt += max(0, l.next-1-l.limit) - max(0, l.next-1-old)
		case "refund_recovery":
			debt = max(0, debt-e.Delta)
		case "draw":
			batch := strings.TrimPrefix(e.Source, fmt.Sprintf("%d:", uid))
			ids, err := activitystore.DrawIDs(ctx, tx, uid, batch)
			if err != nil {
				return err
			}
			if len(ids) != -e.Delta {
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
				if err := activitystore.AttributeDraw(ctx, tx, id, source.id, source.next); err != nil {
					return err
				}
				source.next++
			}
		}
	}
	missing, err = activitystore.HasUnattributedDraws(ctx, tx, uid)
	if err != nil {
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
	order, err := activitystore.LotteryRefundOrder(ctx, tx, uid, oid)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	unit, effective, amount, pay, orderStatus := order.Unit, order.Effective, order.Amount, order.Pay, order.Status
	status := "PARTIALLY_REFUNDED"
	if refund >= amount || orderStatus == OrderStatusRefunded {
		status = "REFUNDED"
	}
	allowed, _ := calculateLotteryRefundRemainingTickets(status, amount, pay, refund, unit, effective)
	recovery, err := activitystore.PendingDrawReward(ctx, tx, uid, oid, allowed)
	if err != nil {
		return err
	}
	return recoverActivityReward(ctx, tx, uid, "lottery_refund", fmt.Sprintf("%d:%d", oid, allowed), recovery, decimal.NewFromFloat(principal), force, "held")
}

func settleLotteryRefund(ctx context.Context, tx activityAccountingDB, uid, oid int64) error {
	settled, err := activitystore.MarkRecoverySettled(ctx, tx, uid, oid)
	if err != nil || !settled {
		return err
	}
	order, err := activitystore.LotteryRefundOrder(ctx, tx, uid, oid)
	if err != nil {
		return err
	}
	allowed, _ := calculateLotteryRefundRemainingTickets(order.Status, order.Amount, order.Pay, order.Refund, order.Unit, order.Effective)
	return activitystore.RevokeDraws(ctx, tx, uid, oid, allowed)
}

// Also handles provider-side refunds observed by the worker. A shortage is
// retained as debt rather than silently treating an already paid reward as valid.
func settleUserRefundRewardsTx(ctx context.Context, tx activityAccountingDB, uid int64) error {
	orders, err := activitystore.RefundedOrders(ctx, tx, uid)
	if err != nil {
		return err
	}
	for _, o := range orders {
		if err := reserveLotteryRefund(ctx, tx, uid, o.ID, o.Refund, 0, true); err != nil {
			return err
		}
		if err := settleLotteryRefund(ctx, tx, uid, o.ID); err != nil {
			return err
		}
	}
	return nil
}

func collectActivityRecoveryDebt(ctx context.Context, tx activityAccountingDB, uid int64) error {
	funds, err := activitystore.LoadFunds(ctx, tx, uid)
	if err != nil {
		return err
	}
	debt, wallet, balanceRaw, frozenRaw := funds.Debt, funds.Wallet, funds.Balance, funds.Frozen
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
	return activitystore.RepayDebt(ctx, tx, uid, fromWallet, fromBalance, debt)
}
