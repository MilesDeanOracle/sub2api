//go:build unit

package service

import (
	"context"
	"errors"
	"testing"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/stretchr/testify/require"
)

func TestRefundRejectsUnverifiedOrPartialPrincipalBeforeGateway(t *testing.T) {
	for _, available := range []float64{0, 25} {
		t.Run(map[bool]string{true: "unverified historical balance", false: "concurrent partial principal"}[available == 0], func(t *testing.T) {
			ctx := context.Background()
			client := newPaymentConfigServiceTestClient(t)
			order := createCompletedRefundableOrderForTest(t, ctx, client, "principal-safety")
			_, err := client.User.UpdateOneID(order.UserID).SetBalance(100).Save(ctx)
			require.NoError(t, err)
			prov := &pendingRefundProviderTestDouble{queryStatus: payment.ProviderStatusSuccess}
			restore := replacePaymentProviderFactoryForTest(t, prov)
			defer restore()
			repo := &mockUserRepo{deductAvailableBalanceFn: func(ctx context.Context, id int64, amount float64) (float64, error) {
				tx := dbent.TxFromContext(ctx)
				require.NotNil(t, tx)
				_, err := tx.Client().User.UpdateOneID(id).AddBalance(-available).Save(ctx)
				return available, err
			}}
			svc := &PaymentService{entClient: client, loadBalancer: &captureLoadBalancer{}, userRepo: repo}
			plan := &RefundPlan{OrderID: order.ID, Order: order, RefundAmount: 100, GatewayAmount: 100, DeductionType: payment.DeductionTypeBalance, BalanceToDeduct: 100}
			_, err = svc.ExecuteRefund(ctx, plan)
			require.ErrorContains(t, err, "principal is insufficient")
			require.Zero(t, prov.refundCalls)
			user, err := client.User.Get(ctx, order.UserID)
			require.NoError(t, err)
			require.Equal(t, 100.0, user.Balance)
			saved, err := client.PaymentOrder.Get(ctx, order.ID)
			require.NoError(t, err)
			require.Equal(t, OrderStatusCompleted, saved.Status)
		})
	}
}

type uncertainRefundProvider struct {
	pendingRefundProviderTestDouble
	fail bool
}

func (p *uncertainRefundProvider) Refund(context.Context, payment.RefundRequest) (*payment.RefundResponse, error) {
	p.refundCalls++
	if p.fail {
		return nil, errors.New("connection lost after submission")
	}
	return &payment.RefundResponse{Status: payment.ProviderStatusPending}, nil
}
func TestRefundUncertainSubmissionKeepsDurableHoldAndNeverResubmits(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	order := createCompletedRefundableOrderForTest(t, ctx, client, "uncertain-hold")
	wallet := &refundHoldWallet{balance: 100}
	prov := &uncertainRefundProvider{fail: true, pendingRefundProviderTestDouble: pendingRefundProviderTestDouble{queryStatus: payment.ProviderStatusSuccess}}
	restore := replacePaymentProviderFactoryForTest(t, prov)
	defer restore()
	svc := &PaymentService{entClient: client, loadBalancer: &captureLoadBalancer{}, userRepo: wallet.repo(order.UserID)}
	plan := &RefundPlan{OrderID: order.ID, Order: order, RefundAmount: 100, GatewayAmount: 100, DeductionType: payment.DeductionTypeBalance, BalanceToDeduct: 100}
	result, err := svc.ExecuteRefund(ctx, plan)
	require.NoError(t, err)
	require.False(t, result.Success)
	require.Contains(t, result.Warning, "uncertain")
	require.Zero(t, wallet.balance)
	detail, err := svc.latestRefundPendingDetail(ctx, order.ID)
	require.NoError(t, err)
	require.True(t, detail.DeductionHeld)
	require.Equal(t, 100.0, detail.BalanceDeducted)
	// A retry after a process restart cannot claim the pending order or deduct twice.
	_, err = svc.ExecuteRefund(ctx, plan)
	require.Error(t, err)
	require.Equal(t, 1, prov.refundCalls)
	require.Equal(t, 1, wallet.deductCalls)
	result, err = svc.QueryAndFinalizeRefund(ctx, order.ID)
	require.NoError(t, err)
	require.True(t, result.Success)
	require.Equal(t, 1, wallet.deductCalls)
}
