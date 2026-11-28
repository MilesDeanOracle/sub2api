package service

import (
	"context"
	"fmt"
	"github.com/Wei-Shaw/sub2api/internal/repository/activitystore"
	"strings"

	"github.com/Wei-Shaw/sub2api/ent/paymentorder"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/shopspring/decimal"
)

func (s *PaymentService) claimRefundWithActivityRewards(ctx context.Context, p *RefundPlan) (int, error) {
	if !s.activityRefundAccounting || p.Order.OrderType != payment.OrderTypeBalance {
		return s.entClient.PaymentOrder.Update().Where(paymentorder.IDEQ(p.OrderID), paymentorder.StatusIn(OrderStatusCompleted, OrderStatusRefundRequested, OrderStatusRefundFailed)).SetStatus(OrderStatusRefunding).Save(ctx)
	}
	hadRollbackFailure := s.hasAuditLog(ctx, p.OrderID, "REFUND_ROLLBACK_FAILED")
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	if err := activitystore.LockWallet(ctx, tx.Client(), p.Order.UserID); err != nil {
		return 0, err
	}
	count, err := tx.PaymentOrder.Update().Where(paymentorder.IDEQ(p.OrderID), paymentorder.StatusIn(OrderStatusCompleted, OrderStatusRefundRequested, OrderStatusRefundFailed)).SetStatus(OrderStatusRefunding).Save(ctx)
	if err != nil || count == 0 {
		return count, err
	}
	principal := 0.0
	if p.DeductBalance {
		principal = p.RefundAmount
	}
	if err := reserveLotteryRefund(ctx, tx.Client(), p.Order.UserID, p.OrderID, p.RefundAmount, principal, p.Force); err != nil {
		return 0, err
	}
	// Reserve principal under the same user lock as reward recovery. Otherwise a
	// concurrent API charge can spend the principal between checking and deducting.
	principalHeld := false
	principalDeducted := decimal.Zero
	if p.DeductionType == payment.DeductionTypeBalance && p.DeductBalance && !hadRollbackFailure {
		funds, err := activitystore.LoadFunds(ctx, tx.Client(), p.Order.UserID)
		if err != nil {
			return 0, err
		}
		available, err := decimal.NewFromString(funds.Balance)
		if err != nil {
			return 0, err
		}
		frozen, err := decimal.NewFromString(funds.Frozen)
		if err != nil {
			return 0, err
		}
		available = decimal.Max(decimal.Zero, available.Sub(frozen))
		requested := decimal.NewFromFloat(p.RefundAmount)
		if available.LessThan(requested) && !p.Force {
			return 0, ErrActivityRecoveryInsufficient
		}
		principalDeducted = decimal.Min(available, requested)
		if err := activitystore.AdjustBalance(ctx, tx.Client(), p.Order.UserID, principalDeducted.Neg().String()); err != nil {
			return 0, err
		}
		principalHeld = true
	}
	if err := createPaymentAuditLogTx(ctx, tx.Client(), p.OrderID, "REFUND_ACTIVITY_RESERVED", map[string]any{"principalDeducted": principalDeducted.String(), "refundAmount": p.RefundAmount, "force": p.Force}); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	p.activityPrincipalHeld = principalHeld
	if principalHeld {
		p.BalanceToDeduct, _ = principalDeducted.Float64()
	}
	s.invalidateRefundBalanceCache(ctx, p.Order.UserID)
	return count, nil
}

// Immediate gateway failures restore principal, reward holds, and order status
// atomically. A crash cannot return only one side of the refund reservation.
func (s *PaymentService) rollbackActivityRefund(ctx context.Context, p *RefundPlan, gatewayErr error) error {
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := activitystore.LockWallet(ctx, tx.Client(), p.Order.UserID); err != nil {
		return err
	}
	changed, err := tx.PaymentOrder.Update().Where(paymentorder.IDEQ(p.OrderID), paymentorder.StatusEQ(OrderStatusRefunding)).SetStatus(p.Order.Status).Save(ctx)
	if err != nil {
		return err
	}
	if changed == 0 {
		return fmt.Errorf("refund status changed during rollback")
	}
	if p.BalanceToDeduct > 0 {
		if err := activitystore.AdjustBalance(ctx, tx.Client(), p.Order.UserID, decimal.NewFromFloat(p.BalanceToDeduct).String()); err != nil {
			return err
		}
	}
	if err := releaseLotteryRecovery(ctx, tx.Client(), p.Order.UserID, p.OrderID); err != nil {
		return err
	}
	if err := createPaymentAuditLogTx(ctx, tx.Client(), p.OrderID, "REFUND_GATEWAY_FAILED", map[string]any{"detail": psErrMsg(gatewayErr), "balanceRestored": p.BalanceToDeduct}); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	s.invalidateRefundBalanceCache(ctx, p.Order.UserID)
	return nil
}

// The provider may have accepted a refund before a timeout, missing response, or
// unknown status. Preserve held funds until a query confirms a terminal outcome.
func normalizeActivityRefundResult(resp *payment.RefundResponse, err error) (*payment.RefundResponse, error) {
	if resp != nil {
		switch strings.TrimSpace(resp.Status) {
		case payment.ProviderStatusSuccess, payment.ProviderStatusRefunded, payment.ProviderStatusPending:
			return resp, nil
		case payment.ProviderStatusFailed:
			return resp, err
		}
	}
	return &payment.RefundResponse{Status: payment.ProviderStatusPending, RefundID: refundResponseID(resp)}, nil
}
