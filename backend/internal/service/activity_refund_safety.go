package service

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/paymentauditlog"
	"github.com/Wei-Shaw/sub2api/ent/paymentorder"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// Persist the order claim, principal hold and recovery evidence atomically before
// contacting a provider. An uncertain network outcome must retain the hold;
// querying the original provider, never resubmitting, resolves it after a crash.
func (s *PaymentService) executeHeldBalanceRefund(ctx context.Context, p *RefundPlan) (*RefundResult, error) {
	if !p.Force && p.BalanceToDeduct < p.RefundAmount-1e-8 {
		return nil, infraerrors.BadRequest("REFUND_FUNDS_UNVERIFIED", "confirmed principal is insufficient; reconcile historical funds before refunding")
	}
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	txCtx := dbent.NewTxContext(ctx, tx)
	claimed, err := tx.PaymentOrder.Update().Where(paymentorder.IDEQ(p.OrderID), paymentorder.StatusIn(OrderStatusCompleted, OrderStatusRefundRequested, OrderStatusRefundFailed)).
		SetStatus(OrderStatusRefundPending).SetRefundAmount(p.RefundAmount).SetRefundReason(p.Reason).SetForceRefund(p.Force).Save(txCtx)
	if err != nil {
		return nil, err
	}
	if claimed == 0 {
		return nil, infraerrors.Conflict("CONFLICT", "order status changed")
	}
	hasFailedHold, err := tx.PaymentAuditLog.Query().Where(paymentauditlog.OrderIDEQ(strconv.FormatInt(p.OrderID, 10)), paymentauditlog.ActionEQ("REFUND_ROLLBACK_FAILED")).Exist(txCtx)
	if err != nil {
		return nil, err
	}
	if hasFailedHold {
		// Legacy failed holds cannot be safely guessed or silently waived.
		return nil, infraerrors.Conflict("REFUND_MANUAL_REVIEW", "previous refund hold needs manual reconciliation before retry")
	}
	deducted := 0.0
	if p.BalanceToDeduct > 0 {
		deducted, err = s.deductAvailableBalance(txCtx, p.Order.UserID, p.BalanceToDeduct)
		if err != nil {
			return nil, fmt.Errorf("reserve refund principal: %w", err)
		}
	}
	if math.IsNaN(deducted) || math.IsInf(deducted, 0) || deducted < 0 || deducted > p.BalanceToDeduct+1e-8 {
		return nil, fmt.Errorf("invalid refund deduction result")
	}
	if !p.Force && deducted < p.RefundAmount-1e-8 {
		// Repository uses this same transaction. Rollback restores any partial hold.
		return nil, infraerrors.BadRequest("REFUND_FUNDS_UNVERIFIED", "confirmed principal is insufficient; reconcile historical funds before refunding")
	}
	if err = createPaymentAuditLogTx(txCtx, tx.Client(), p.OrderID, "REFUND_PENDING", map[string]any{
		"refundAmount": p.RefundAmount, "reason": p.Reason, "force": p.Force, "deductionHeld": true,
		"deductionType": payment.DeductionTypeBalance, "balanceDeducted": deducted, "providerSubmission": "uncertain_until_confirmed",
	}); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	p.BalanceToDeduct = deducted
	p.deductionHeld = true
	s.invalidateRefundBalanceCache(ctx, p.Order.UserID)
	// Detach finalization from a client disconnect, but retain a bounded deadline.
	finalizeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
	defer cancel()
	resp, err := s.gwRefund(finalizeCtx, p)
	if err != nil {
		if resp != nil && resp.Status == payment.ProviderStatusFailed {
			return s.finalizeRefundFailed(finalizeCtx, p, err)
		}
		s.writeAuditLog(finalizeCtx, p.OrderID, "REFUND_SUBMISSION_UNCERTAIN", "admin", map[string]any{"detail": psErrMsg(err)})
		return &RefundResult{Success: false, Warning: "refund outcome is uncertain; funds remain held; query the original provider before any retry"}, nil
	}
	switch resp.Status {
	case payment.ProviderStatusSuccess, payment.ProviderStatusRefunded:
		return s.finalizePendingRefundSuccess(finalizeCtx, p)
	case payment.ProviderStatusPending:
		return s.markRefundPending(finalizeCtx, p, resp)
	default:
		return &RefundResult{Success: false, Warning: "refund requires provider reconciliation; funds remain held"}, nil
	}
}
