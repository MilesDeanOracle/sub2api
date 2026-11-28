package service

import (
	"context"
	"errors"
	"github.com/Wei-Shaw/sub2api/internal/repository/activitystore"

	"database/sql"
	"encoding/json"
	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"fmt"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

type activityProductionFixture struct {
	db      *sql.DB
	service *ActivityService
	ctx     context.Context
	t       *testing.T
}

func newActivityProductionFixture(t *testing.T) *activityProductionFixture {
	t.Helper()
	dsn := os.Getenv("ACTIVITY_TEST_DSN")
	if dsn == "" {
		t.Skip("set ACTIVITY_TEST_DSN for isolated production accounting tests")
	}
	root, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	schema := fmt.Sprintf("activity_production_%d", time.Now().UnixNano())
	_, err = root.Exec(`CREATE SCHEMA ` + schema)
	require.NoError(t, err)
	u, err := url.Parse(dsn)
	require.NoError(t, err)
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	db, err := sql.Open("postgres", u.String())
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, db.Close())
		_, err := root.Exec(`DROP SCHEMA ` + schema + ` CASCADE`)
		require.NoError(t, err)
		require.NoError(t, root.Close())
	})
	f := &activityProductionFixture{db: db, service: NewActivityService(db, nil, nil), ctx: context.Background(), t: t}
	f.exec(`CREATE TABLE settings(key TEXT PRIMARY KEY,value TEXT NOT NULL);
 CREATE TABLE users(id BIGINT PRIMARY KEY,balance NUMERIC(20,8) NOT NULL DEFAULT 0,frozen_balance NUMERIC(20,8) NOT NULL DEFAULT 0,status TEXT NOT NULL DEFAULT 'active',deleted_at TIMESTAMPTZ,created_at TIMESTAMPTZ DEFAULT NOW(),updated_at TIMESTAMPTZ DEFAULT NOW());
 CREATE TABLE usage_logs(id BIGSERIAL PRIMARY KEY,user_id BIGINT,api_key_id BIGINT DEFAULT 1,request_id TEXT,actual_cost NUMERIC(20,10),billing_type INT DEFAULT 0,request_type INT DEFAULT 0,created_at TIMESTAMPTZ DEFAULT NOW());
 INSERT INTO users(id,balance) VALUES(1,100);`)
	rawOrder, err := os.ReadFile(filepath.Join("..", "..", "migrations", "092_payment_orders.sql"))
	require.NoError(t, err)
	f.exec(string(rawOrder))
	f.exec(`ALTER TABLE payment_orders ADD COLUMN bonus_amount NUMERIC DEFAULT 0, ADD COLUMN out_trade_no TEXT NOT NULL DEFAULT '', ADD COLUMN provider_key TEXT, ADD COLUMN provider_snapshot JSONB;
 CREATE TABLE payment_audit_logs(id BIGSERIAL PRIMARY KEY,order_id TEXT NOT NULL,action TEXT NOT NULL,detail TEXT NOT NULL DEFAULT '',operator TEXT NOT NULL DEFAULT '',created_at TIMESTAMPTZ NOT NULL DEFAULT NOW());`)
	for _, file := range []string{"258_activity_rewards.sql", "259_activity_lottery.sql", "260_activity_independent_budgets.sql", "264_activity_production_safety.sql", "265_activity_lottery_nine_grid.sql", "266_activity_consumption_master_switch.sql", "268_activity_atomic_billing.sql"} {
		raw, err := os.ReadFile(filepath.Join("..", "..", "migrations", file))
		require.NoError(t, err)
		f.exec(string(raw))
	}
	// Legacy-log fixtures explicitly keep the reconciliation interval open.
	f.exec(`UPDATE activity_billing_config SET cutover_at='infinity'; UPDATE activity_audiences SET mode='all'; UPDATE activity_config SET checkin_enabled=true,checkin_require_recharge=false,checkin_rollout_percent=100,checkin_min_spend_cents=0,checkin_daily_cents=25,checkin_monthly_budget_cents=10000,user_lifetime_cap_cents=10000;
 UPDATE activity_lottery_config SET enabled=true,budget_cents=1000000;
 INSERT INTO activity_lottery_windows(unit_pay_cents,opened_at) VALUES(1000,NOW()-interval '1 day');
 INSERT INTO payment_orders(id,user_id,amount,pay_amount,status,order_type,paid_at,completed_at,expires_at) VALUES(100,1,100,100,'COMPLETED','balance',NOW(),NOW(),NOW());`)
	tiers := defaultLotteryTiers()
	for i := range tiers {
		tiers[i].RewardCents = 100
	}
	raw, err := json.Marshal(tiers)
	require.NoError(t, err)
	f.exec(`UPDATE activity_lottery_config SET tiers=$1::jsonb`, string(raw))
	return f
}
func (f *activityProductionFixture) exec(q string, args ...any) {
	f.t.Helper()
	_, err := f.db.Exec(q, args...)
	require.NoError(f.t, err)
}
func (f *activityProductionFixture) scalar(q string, args ...any) string {
	f.t.Helper()
	var value string
	require.NoError(f.t, f.db.QueryRow(q, args...).Scan(&value))
	return value
}
func (f *activityProductionFixture) transaction(fn func(*sql.Tx) error) error {
	tx, err := f.db.BeginTx(f.ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := activitystore.LockWallet(f.ctx, tx, 1); err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}

func TestLotteryNineGridMigrationPostgres(t *testing.T) {
	f := newActivityProductionFixture(t)
	require.Equal(t, "8", f.scalar(`SELECT COUNT(*)::text FROM activity_lottery_config, jsonb_array_elements(tiers) tier WHERE (tier->>'weight')::int>0`))
	legacy := legacySixLotteryTiers()
	// Preserve operator-customized amounts and weights, not just defaults.
	legacy[0].RewardCents, legacy[0].Weight = 75, 4321
	raw, err := json.Marshal(legacy)
	require.NoError(t, err)
	f.exec(`UPDATE activity_lottery_config SET tiers=$1::jsonb`, string(raw))
	f.exec(`INSERT INTO activity_lottery_draws(user_id,batch_key,draw_count,draw_index,tier_position,reward_cents) VALUES(1,'old-six',1,1,6,5000)`)
	migration, err := os.ReadFile(filepath.Join("..", "..", "migrations", "265_activity_lottery_nine_grid.sql"))
	require.NoError(t, err)
	f.exec(string(migration))
	f.exec(string(migration))
	config, err := f.service.AdminLotteryConfig(f.ctx)
	require.NoError(t, err)
	require.Len(t, config.Tiers, 8)
	for i := range legacy {
		require.Equal(t, legacy[i].Position, config.Tiers[i].Position)
		require.Equal(t, legacy[i].RewardCents, config.Tiers[i].RewardCents)
		require.Equal(t, legacy[i].Weight, config.Tiers[i].Weight)
	}
	require.Zero(t, config.Tiers[6].Weight)
	require.Zero(t, config.Tiers[7].Weight)
	require.True(t, config.TiersLocked)
	require.Equal(t, "6:5000", f.scalar(`SELECT tier_position::text||':'||reward_cents::text FROM activity_lottery_draws WHERE batch_key='old-six'`))
	config.Tiers[6].Weight = 1
	_, err = f.service.UpdateAdminLotteryConfig(f.ctx, 1, *config)
	require.ErrorContains(t, err, "请先停止新增抽奖资格")

	// Schema and service must support both new positions, including real draws.
	f.exec(`DELETE FROM activity_lottery_draws WHERE batch_key='old-six'`)
	for position := 7; position <= 8; position++ {
		tiers := defaultLotteryTiers()
		for i := range tiers {
			tiers[i].Weight = 0
		}
		tiers[position-1].Weight = 1
		raw, err := json.Marshal(tiers)
		require.NoError(t, err)
		f.exec(`UPDATE activity_lottery_config SET tiers=$1::jsonb`, string(raw))
		result, err := f.service.DrawLottery(f.ctx, 1, 1, fmt.Sprintf("new-slot-%d", position))
		require.NoError(t, err)
		require.Len(t, result.Items, 1)
		require.Equal(t, position, result.Items[0].TierPosition)
	}
}

func TestLotteryEightTierConfigurationPostgres(t *testing.T) {
	f := newActivityProductionFixture(t)
	_, err := f.service.DrawLottery(f.ctx, 1, 10, "settle-before-eight-tier-edit")
	require.NoError(t, err)
	config, err := f.service.AdminLotteryConfig(f.ctx)
	require.NoError(t, err)
	config.Enabled = false
	config, err = f.service.UpdateAdminLotteryConfig(f.ctx, 1, *config)
	require.NoError(t, err)
	require.False(t, config.TiersLocked)
	config.Tiers = defaultLotteryTiers()
	config.Tiers[6].Weight = 0
	_, err = f.service.UpdateAdminLotteryConfig(f.ctx, 1, *config)
	require.ErrorContains(t, err, "八档奖项必须全部配置正整数权重")
	config.Tiers = defaultLotteryTiers()
	for i := range config.Tiers {
		config.Tiers[i].RewardCents += 1
		config.Tiers[i].Weight += 1
	}
	updated, err := f.service.UpdateAdminLotteryConfig(f.ctx, 1, *config)
	require.NoError(t, err)
	for i := range updated.Tiers {
		require.Equal(t, config.Tiers[i].RewardCents, updated.Tiers[i].RewardCents)
		require.Equal(t, config.Tiers[i].Weight, updated.Tiers[i].Weight)
	}
}

func TestActivityProductionPostgres(t *testing.T) {
	t.Run("zero reward never consumes today's checkin", func(t *testing.T) {
		f := newActivityProductionFixture(t)
		f.exec(`UPDATE activity_config SET checkin_monthly_budget_cents=0`)
		_, err := f.service.Checkin(f.ctx, 1)
		require.ErrorIs(t, err, ErrActivityNoReward)
		require.Equal(t, "0", f.scalar(`SELECT COUNT(*)::text FROM activity_checkins`))
		status, err := f.service.Status(f.ctx, 1)
		require.NoError(t, err)
		require.Equal(t, "budget_exhausted", status.CheckinBlockReason)
		require.False(t, status.CheckinEligible)
		f.exec(`UPDATE activity_config SET checkin_monthly_budget_cents=10000`)
		status, err = f.service.Checkin(f.ctx, 1)
		require.NoError(t, err)
		require.Equal(t, 25, status.TodayRewardCents)
		require.True(t, status.CheckedToday)
	})
	t.Run("partial refund before first sync retains unrefunded tickets", func(t *testing.T) {
		f := newActivityProductionFixture(t)
		f.exec(`UPDATE payment_orders SET status='PARTIALLY_REFUNDED',refund_amount=30 WHERE id=100`)
		status, err := f.service.LotteryStatus(f.ctx, 1)
		require.NoError(t, err)
		require.Equal(t, 7, status.RemainingDraws)
	})
	t.Run("refund hold rollback and settlement are idempotent and order specific", func(t *testing.T) {
		f := newActivityProductionFixture(t)
		_, err := f.service.LotteryStatus(f.ctx, 1)
		require.NoError(t, err)
		draw, err := f.service.DrawLottery(f.ctx, 1, 10, "refund-accounting")
		require.NoError(t, err)
		require.EqualValues(t, 1000, draw.TotalRewardCents)
		reserve := func(tx *sql.Tx) error { return reserveLotteryRefund(f.ctx, tx, 1, 100, 50, 50, false) }
		require.NoError(t, f.transaction(reserve))
		require.NoError(t, f.transaction(reserve))
		require.Equal(t, "500", f.scalar(`SELECT available_cents::text FROM activity_wallets WHERE user_id=1`))
		require.NoError(t, f.transaction(func(tx *sql.Tx) error { return releaseLotteryRecovery(f.ctx, tx, 1, 100) }))
		require.NoError(t, f.transaction(func(tx *sql.Tx) error { return releaseLotteryRecovery(f.ctx, tx, 1, 100) }))
		require.Equal(t, "1000", f.scalar(`SELECT available_cents::text FROM activity_wallets WHERE user_id=1`))
		require.NoError(t, f.transaction(reserve))
		f.exec(`UPDATE payment_orders SET status='PARTIALLY_REFUNDED',refund_amount=50 WHERE id=100`)
		require.NoError(t, f.transaction(func(tx *sql.Tx) error { return settleLotteryRefund(f.ctx, tx, 1, 100) }))
		require.NoError(t, f.transaction(func(tx *sql.Tx) error { return settleLotteryRefund(f.ctx, tx, 1, 100) }))
		require.Equal(t, "5", f.scalar(`SELECT COUNT(*)::text FROM activity_lottery_draws WHERE revoked_at IS NOT NULL`))
		status, err := f.service.LotteryStatus(f.ctx, 1)
		require.NoError(t, err)
		require.Equal(t, 0, status.RemainingDraws)
		require.Equal(t, 0, status.TicketDebt)
		require.Equal(t, "500", f.scalar(`SELECT available_cents::text FROM activity_wallets WHERE user_id=1`))
	})
	t.Run("transferred or spent bonuses cannot subsidize a full principal refund", func(t *testing.T) {
		f := newActivityProductionFixture(t)
		_, err := f.service.LotteryStatus(f.ctx, 1)
		require.NoError(t, err)
		_, err = f.service.DrawLottery(f.ctx, 1, 10, "transferred-refund")
		require.NoError(t, err)
		_, err = f.service.WithdrawWallet(f.ctx, 1)
		require.NoError(t, err)
		f.exec(`UPDATE users SET balance=107 WHERE id=1`)
		f.exec(`UPDATE activity_wallets SET unabsorbed_gift=7 WHERE user_id=1`)
		err = f.transaction(func(tx *sql.Tx) error { return reserveLotteryRefund(f.ctx, tx, 1, 100, 100, 100, false) })
		require.ErrorIs(t, err, ErrActivityRecoveryInsufficient)
		require.Equal(t, "107.00000000", f.scalar(`SELECT balance::text FROM users WHERE id=1`))
		f.exec(`UPDATE users SET balance=110 WHERE id=1`)
		require.NoError(t, f.transaction(func(tx *sql.Tx) error { return reserveLotteryRefund(f.ctx, tx, 1, 100, 100, 100, false) }))
		require.Equal(t, "100.00000000", f.scalar(`SELECT balance::text FROM users WHERE id=1`))
		require.NoError(t, f.transaction(func(tx *sql.Tx) error { return releaseLotteryRecovery(f.ctx, tx, 1, 100) }))
		require.Equal(t, "110.00000000", f.scalar(`SELECT balance::text FROM users WHERE id=1`))
		require.Equal(t, "7.00000000", f.scalar(`SELECT unabsorbed_gift::text FROM activity_wallets WHERE user_id=1`))
	})
	t.Run("simultaneous draws cannot overspend tickets or budget", func(t *testing.T) {
		f := newActivityProductionFixture(t)
		_, err := f.service.LotteryStatus(f.ctx, 1)
		require.NoError(t, err)
		f.exec(`UPDATE activity_lottery_config SET budget_cents=1000`)
		var wg sync.WaitGroup
		errs := make(chan error, 2)
		for _, key := range []string{"concurrent-a", "concurrent-b"} {
			wg.Add(1)
			go func(key string) {
				defer wg.Done()
				ctx, cancel := context.WithTimeout(f.ctx, 5*time.Second)
				defer cancel()
				_, err := f.service.DrawLottery(ctx, 1, 10, key)
				errs <- err
			}(key)
		}
		wg.Wait()
		close(errs)
		success := 0
		for err := range errs {
			if err == nil {
				success++
			}
		}
		require.Equal(t, 1, success)
		require.Equal(t, "1000", f.scalar(`SELECT SUM(amount_cents)::text FROM activity_reward_ledger WHERE kind='lottery'`))
		require.Equal(t, "false", f.scalar(`SELECT enabled::text FROM activity_lottery_config`))
		require.Equal(t, "0", f.scalar(`SELECT COUNT(*)::text FROM activity_lottery_windows WHERE closed_at IS NULL`))
	})
	t.Run("rejected claimed reward is recovered once and shortages remain blocked", func(t *testing.T) {
		f := newActivityProductionFixture(t)
		f.exec(`INSERT INTO activity_consumption_campaigns(id,name,starts_at,ends_at,claim_until,state,published_at,budget_cents) VALUES(1,'review',NOW()-interval '1 day',NOW()+interval '1 day',NOW()+interval '3 days','active',NOW()-interval '1 day',10000);
  INSERT INTO activity_consumption_tiers(campaign_id,position,threshold_cents,reward_cents) VALUES(1,1,100,500);
  INSERT INTO activity_consumption_entitlements(campaign_id,user_id,position,reward_cents,state,claimed_at) VALUES(1,1,1,500,'review',NOW());`)
		require.NoError(t, f.transaction(func(tx *sql.Tx) error { return creditActivityWalletTx(f.ctx, tx, 1, "consumption", "1:1:1", 500) }))
		f.exec(`UPDATE activity_wallets SET available_cents=0 WHERE user_id=1`)
		f.exec(`UPDATE users SET balance=0 WHERE id=1`)
		require.NoError(t, f.service.ResolveConsumptionReview(f.ctx, 99, 1, false, "invalid usage reversed"))
		require.Equal(t, "revoked", f.scalar(`SELECT state FROM activity_consumption_entitlements WHERE user_id=1`))
		require.Equal(t, "500", f.scalar(`SELECT recovery_debt_cents::text FROM activity_wallets WHERE user_id=1`))
		require.NoError(t, f.service.ResolveConsumptionReview(f.ctx, 99, 1, false, "retry"))
		require.Equal(t, "1", f.scalar(`SELECT COUNT(*)::text FROM activity_reward_recoveries`))
		wallet, err := f.service.WalletStatus(f.ctx, 1)
		require.NoError(t, err)
		require.True(t, wallet.WithdrawBlocked)
		require.Equal(t, "recovery_debt", wallet.WithdrawBlockReason)
	})
	t.Run("budget shortage is distinguished from processing", func(t *testing.T) {
		f := newActivityProductionFixture(t)
		f.exec(`INSERT INTO activity_consumption_campaigns(id,name,starts_at,ends_at,claim_until,state,published_at,budget_cents,rollout_percent) VALUES(1,'budget',NOW()-interval '1 day',NOW()+interval '1 day',NOW()+interval '3 days','active',NOW()-interval '1 day',0,100);
  INSERT INTO activity_consumption_tiers(campaign_id,position,threshold_cents,reward_cents) VALUES(1,1,100,500);
  INSERT INTO activity_usage_events(usage_log_id,user_id,actual_cost,eligible_cost,occurred_at) VALUES(1,1,1,1,NOW());
  INSERT INTO activity_consumption_outbox(event_id,user_id) VALUES(1,1);`)
		require.NoError(t, f.transaction(func(tx *sql.Tx) error {
			var done bool
			return tx.QueryRow(`SELECT activity_process_consumption_event(1)`).Scan(&done)
		}))
		summary, err := f.service.ConsumptionStatus(f.ctx, 1)
		require.NoError(t, err)
		require.True(t, summary.ProcessingPending)
		require.True(t, summary.BudgetBlocked)
		f.exec(`UPDATE activity_config SET user_lifetime_cap_cents=0`)
		summary, err = f.service.ConsumptionStatus(f.ctx, 1)
		require.NoError(t, err)
		require.Equal(t, "user_cap", summary.BlockReason)
	})

	t.Run("historical unrelated refunds do not freeze new rewards", func(t *testing.T) {
		f := newActivityProductionFixture(t)
		f.exec(`UPDATE payment_orders SET status='REFUNDED',refund_amount=100,updated_at=NOW()-interval '1 year',paid_at=NOW()-interval '1 year',completed_at=NOW()-interval '1 year' WHERE id=100`)
		require.NoError(t, f.service.SyncRefundReviews(f.ctx))
		require.Equal(t, "0", f.scalar(`SELECT COUNT(*)::text FROM activity_refund_reviews`))
		_, err := f.service.Checkin(f.ctx, 1)
		require.NoError(t, err)
	})
	t.Run("odds cannot change while existing tickets are outstanding", func(t *testing.T) {
		f := newActivityProductionFixture(t)
		_, err := f.service.LotteryStatus(f.ctx, 1)
		require.NoError(t, err)
		config, err := f.service.AdminLotteryConfig(f.ctx)
		require.NoError(t, err)
		require.True(t, config.TiersLocked)
		require.EqualValues(t, 10, config.OutstandingTickets)
		config.Enabled = false
		_, err = f.service.UpdateAdminLotteryConfig(f.ctx, 99, *config)
		require.NoError(t, err)
		config.Tiers[0].Weight++
		_, err = f.service.UpdateAdminLotteryConfig(f.ctx, 99, *config)
		require.Error(t, err)
	})
	t.Run("legacy draws retain order attribution for refund recovery", func(t *testing.T) {
		f := newActivityProductionFixture(t)
		_, err := f.service.LotteryStatus(f.ctx, 1)
		require.NoError(t, err)
		_, err = f.service.DrawLottery(f.ctx, 1, 10, "legacy")
		require.NoError(t, err)
		f.exec(`UPDATE activity_lottery_draws SET order_id=NULL,order_ticket_index=NULL`)
		require.NoError(t, f.transaction(func(tx *sql.Tx) error { return reserveLotteryRefund(f.ctx, tx, 1, 100, 50, 50, false) }))
		require.Equal(t, "10", f.scalar(`SELECT COUNT(*)::text FROM activity_lottery_draws WHERE order_id=100`))
		require.Equal(t, "500", f.scalar(`SELECT available_cents::text FROM activity_wallets WHERE user_id=1`))
	})
	t.Run("recovery debt must be repaid before review can release rewards", func(t *testing.T) {
		f := newActivityProductionFixture(t)
		f.exec(`UPDATE users SET balance=0 WHERE id=1`)
		require.NoError(t, f.transaction(func(tx *sql.Tx) error {
			return recoverActivityReward(f.ctx, tx, 1, "consumption_revoke", "review-debt", 500, decimal.Zero, true, "settled")
		}))
		require.ErrorIs(t, f.transaction(func(tx *sql.Tx) error { return collectActivityRecoveryDebt(f.ctx, tx, 1) }), ErrActivityRecoveryInsufficient)
		f.exec(`UPDATE users SET balance=5 WHERE id=1`)
		require.NoError(t, f.transaction(func(tx *sql.Tx) error { return collectActivityRecoveryDebt(f.ctx, tx, 1) }))
		require.NoError(t, f.transaction(func(tx *sql.Tx) error { return collectActivityRecoveryDebt(f.ctx, tx, 1) }))
		require.Equal(t, "0", f.scalar(`SELECT recovery_debt_cents::text FROM activity_wallets WHERE user_id=1`))
		require.Equal(t, "0.00000000", f.scalar(`SELECT balance::text FROM users WHERE id=1`))
	})

	t.Run("a corrupt legacy user does not starve other refund reviews", func(t *testing.T) {
		f := newActivityProductionFixture(t)
		f.exec(`INSERT INTO users(id) VALUES(2);
   INSERT INTO payment_orders(id,user_id,amount,pay_amount,status,order_type,paid_at,completed_at,expires_at) VALUES(200,2,50,50,'REFUNDED','balance',NOW(),NOW(),NOW());
   INSERT INTO activity_lottery_draws(user_id,batch_key,draw_count,draw_index,tier_position,reward_cents) VALUES(1,'inconsistent-legacy',1,1,1,100);
   UPDATE payment_orders SET status='REFUNDED',refund_amount=100 WHERE id=100`)
		require.ErrorIs(t, f.service.SyncRefundReviews(f.ctx), ErrActivityRecoveryReview)
		require.Equal(t, "pending", f.scalar(`SELECT review_status FROM activity_refund_reviews WHERE order_id=200`))
		require.Equal(t, "1", f.scalar(`SELECT user_id::text FROM activity_refund_sync_queue`))
		require.Equal(t, "true", f.scalar(`SELECT (last_refund_scan_at IS NOT NULL)::text FROM activity_worker_status WHERE id=1`))
	})
	t.Run("safety migration and read-only preflight remain executable", func(t *testing.T) {
		f := newActivityProductionFixture(t)
		migration, err := os.ReadFile(filepath.Join("..", "..", "migrations", "264_activity_production_safety.sql"))
		require.NoError(t, err)
		f.exec(string(migration))
		preflight, err := os.ReadFile(filepath.Join("..", "..", "..", "scripts", "activity-rewards-preflight.sql"))
		require.NoError(t, err)
		f.exec(string(preflight))
	})
	t.Run("frozen balance cannot pay reward recovery", func(t *testing.T) {
		f := newActivityProductionFixture(t)
		f.exec(`UPDATE users SET balance=105,frozen_balance=5 WHERE id=1`)
		err := f.transaction(func(tx *sql.Tx) error {
			return recoverActivityReward(f.ctx, tx, 1, "lottery_refund", "frozen", 100, decimal.NewFromInt(100), false, "held")
		})
		require.ErrorIs(t, err, ErrActivityRecoveryInsufficient)
	})
}

// This repository returns balance through the caller's ent transaction, as the
// production repository does. Embedded methods must never be called in these tests.
type activityPaymentUserRepo struct {
	UserRepository
	db *sql.DB
}

func (r *activityPaymentUserRepo) UpdateBalance(ctx context.Context, id int64, amount float64) error {
	var db activityAccountingDB = r.db
	if tx := dbent.TxFromContext(ctx); tx != nil {
		db = tx.Client()
	}
	_, err := db.ExecContext(ctx, `UPDATE users SET balance=balance+$2::numeric WHERE id=$1`, id, amount)
	return err
}
func (f *activityProductionFixture) paymentService() (*PaymentService, *RefundPlan) {
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, f.db)))
	svc := &PaymentService{entClient: client, activityRefundAccounting: true, userRepo: &activityPaymentUserRepo{db: f.db}}
	order, err := client.PaymentOrder.Get(f.ctx, 100)
	require.NoError(f.t, err)
	return svc, &RefundPlan{OrderID: 100, Order: order, RefundAmount: 100, GatewayAmount: 100, Reason: "production regression", DeductBalance: true, DeductionType: payment.DeductionTypeBalance, BalanceToDeduct: 100}
}
func TestActivityPaymentProductionPostgres(t *testing.T) {
	setup := func(t *testing.T) *activityProductionFixture {
		f := newActivityProductionFixture(t)
		_, err := f.service.LotteryStatus(f.ctx, 1)
		require.NoError(t, err)
		_, err = f.service.DrawLottery(f.ctx, 1, 10, "payment-flow")
		require.NoError(t, err)
		return f
	}
	t.Run("real ent success settles principal and rewards", func(t *testing.T) {
		f := setup(t)
		svc, p := f.paymentService()
		result, err := svc.ExecuteRefund(f.ctx, p)
		require.NoError(t, err)
		require.True(t, result.Success)
		require.Equal(t, "0.00000000", f.scalar(`SELECT balance::text FROM users WHERE id=1`))
		require.Equal(t, "0", f.scalar(`SELECT available_cents::text FROM activity_wallets WHERE user_id=1`))
		require.Equal(t, OrderStatusRefunded, f.scalar(`SELECT status FROM payment_orders WHERE id=100`))
		require.Equal(t, "settled", f.scalar(`SELECT state FROM activity_reward_recoveries`))
		status, err := f.service.LotteryStatus(f.ctx, 1)
		require.NoError(t, err)
		require.Zero(t, status.TicketDebt)
		require.Equal(t, "1", f.scalar(`SELECT COUNT(*)::text FROM payment_audit_logs WHERE action='REFUND_ACTIVITY_RESERVED'`))
	})
	t.Run("gateway failure restores all reservations atomically", func(t *testing.T) {
		f := setup(t)
		svc, p := f.paymentService()
		n, err := svc.claimRefundWithActivityRewards(f.ctx, p)
		require.NoError(t, err)
		require.Equal(t, 1, n)
		_, err = svc.handleGwFail(f.ctx, p, errors.New("provider rejected"))
		require.NoError(t, err)
		require.Equal(t, "100.00000000", f.scalar(`SELECT balance::text FROM users WHERE id=1`))
		require.Equal(t, "1000", f.scalar(`SELECT available_cents::text FROM activity_wallets WHERE user_id=1`))
		require.Equal(t, OrderStatusCompleted, f.scalar(`SELECT status FROM payment_orders WHERE id=100`))
		result, err := svc.ExecuteRefund(f.ctx, p)
		require.NoError(t, err)
		require.True(t, result.Success)
	})
	for _, success := range []bool{true, false} {
		t.Run(fmt.Sprintf("pending terminal success=%v preserves holds", success), func(t *testing.T) {
			f := setup(t)
			svc, p := f.paymentService()
			_, err := svc.claimRefundWithActivityRewards(f.ctx, p)
			require.NoError(t, err)
			_, err = svc.markRefundPending(f.ctx, p, &payment.RefundResponse{Status: payment.ProviderStatusPending, RefundID: "pending"})
			require.NoError(t, err)
			order, err := svc.entClient.PaymentOrder.Get(f.ctx, 100)
			require.NoError(t, err)
			terminal := svc.refundFinalizePlan(order)
			detail, err := svc.latestRefundPendingDetail(f.ctx, 100)
			require.NoError(t, err)
			detail.applyTo(terminal)
			if success {
				_, err = svc.finalizePendingRefundSuccess(f.ctx, terminal)
			} else {
				_, err = svc.finalizeRefundFailed(f.ctx, terminal, errors.New("declined"))
			}
			require.NoError(t, err)
			if success {
				require.Equal(t, "0.00000000", f.scalar(`SELECT balance::text FROM users WHERE id=1`))
				require.Equal(t, "settled", f.scalar(`SELECT state FROM activity_reward_recoveries`))
			} else {
				require.Equal(t, "100.00000000", f.scalar(`SELECT balance::text FROM users WHERE id=1`))
				require.Equal(t, "1000", f.scalar(`SELECT available_cents::text FROM activity_wallets WHERE user_id=1`))
				require.Equal(t, "released", f.scalar(`SELECT state FROM activity_reward_recoveries`))
			}
			_, err = svc.finalizePendingRefundSuccess(f.ctx, terminal)
			require.Error(t, err)
		})
	}

	t.Run("unknown gateway outcomes keep principal and rewards held", func(t *testing.T) {
		f := setup(t)
		svc, p := f.paymentService()
		_, err := svc.claimRefundWithActivityRewards(f.ctx, p)
		require.NoError(t, err)
		response, err := normalizeActivityRefundResult(nil, context.DeadlineExceeded)
		require.NoError(t, err)
		result, err := svc.finishRefund(f.ctx, p, response)
		require.NoError(t, err)
		require.False(t, result.Success)
		require.Equal(t, OrderStatusRefundPending, f.scalar(`SELECT status FROM payment_orders WHERE id=100`))
		require.Equal(t, "0.00000000", f.scalar(`SELECT balance::text FROM users WHERE id=1`))
		require.Equal(t, "held", f.scalar(`SELECT state FROM activity_reward_recoveries`))
		_, err = svc.ExecuteRefund(f.ctx, p)
		require.Error(t, err)
	})
	t.Run("provider full refund with missing amount revokes all rewards", func(t *testing.T) {
		f := setup(t)
		f.exec(`UPDATE payment_orders SET status='REFUNDED',refund_amount=0 WHERE id=100`)
		require.NoError(t, f.transaction(func(tx *sql.Tx) error { return settleUserRefundRewardsTx(f.ctx, tx, 1) }))
		require.Equal(t, "10", f.scalar(`SELECT COUNT(*)::text FROM activity_lottery_draws WHERE revoked_at IS NOT NULL`))
		require.Equal(t, "0", f.scalar(`SELECT available_cents::text FROM activity_wallets WHERE user_id=1`))
	})
	t.Run("successive partial refunds only recover additional invalid draws", func(t *testing.T) {
		f := setup(t)
		for _, amount := range []int{30, 50, 100} {
			f.exec(`UPDATE payment_orders SET status=CASE WHEN $1=100 THEN 'REFUNDED' ELSE 'PARTIALLY_REFUNDED' END,refund_amount=$1 WHERE id=100`, amount)
			require.NoError(t, f.transaction(func(tx *sql.Tx) error { return settleUserRefundRewardsTx(f.ctx, tx, 1) }))
			require.NoError(t, f.transaction(func(tx *sql.Tx) error { return settleUserRefundRewardsTx(f.ctx, tx, 1) }))
			require.Equal(t, fmt.Sprint(1000-amount*10), f.scalar(`SELECT available_cents::text FROM activity_wallets WHERE user_id=1`))
		}
		require.Equal(t, "3", f.scalar(`SELECT COUNT(*)::text FROM activity_reward_recoveries`))
	})
}

func TestActivityRefundUncertainResponses(t *testing.T) {
	for _, tc := range []struct {
		name     string
		response *payment.RefundResponse
		err      error
		want     string
	}{
		{"timeout", nil, context.DeadlineExceeded, payment.ProviderStatusPending},
		{"missing", nil, nil, payment.ProviderStatusPending},
		{"unknown", &payment.RefundResponse{Status: "unrecognized", RefundID: "lookup"}, nil, payment.ProviderStatusPending},
		{"rejected", &payment.RefundResponse{Status: payment.ProviderStatusFailed}, nil, payment.ProviderStatusFailed},
		{"success plus warning", &payment.RefundResponse{Status: payment.ProviderStatusSuccess}, errors.New("warning"), payment.ProviderStatusSuccess},
	} {
		t.Run(tc.name, func(t *testing.T) {
			response, err := normalizeActivityRefundResult(tc.response, tc.err)
			require.NoError(t, err)
			require.Equal(t, tc.want, response.Status)
		})
	}
}

func accountingConsumptionFixture(t *testing.T) *activityProductionFixture {
	f := newActivityProductionFixture(t)
	f.exec(`INSERT INTO activity_consumption_campaigns(id,name,starts_at,ends_at,claim_until,rollout_percent,budget_cents,state,published_at) VALUES(1,'review reproduction',NOW()-interval '1 hour',NOW()+interval '1 day',NOW()+interval '3 days',100,10000,'active',NOW()-interval '1 hour');
 INSERT INTO activity_consumption_tiers(campaign_id,position,threshold_cents,reward_cents) VALUES(1,1,1000,30)`)
	_, err := f.service.IngestUsageEvents(f.ctx)
	require.NoError(t, err)
	return f
}

func TestRegressionOutOfOrderUsageCommitPostgres(t *testing.T) {
	f := accountingConsumptionFixture(t)
	early, err := f.db.BeginTx(f.ctx, nil)
	require.NoError(t, err)
	defer early.Rollback()
	var lowerID int64
	require.NoError(t, early.QueryRow(`INSERT INTO usage_logs(user_id,request_id,actual_cost) VALUES(1,'lower-id-later-commit',10) RETURNING id`).Scan(&lowerID))
	f.exec(`INSERT INTO usage_logs(user_id,request_id,actual_cost) VALUES(1,'higher-id-first-commit',10)`)
	first, err := f.service.IngestUsageEvents(f.ctx)
	require.NoError(t, err)
	require.NoError(t, early.Commit())
	second, err := f.service.IngestUsageEvents(f.ctx)
	require.NoError(t, err)
	count := f.scalar(`SELECT COUNT(*)::text FROM activity_usage_events`)
	t.Logf("lowerID=%d firstIngest=%d secondIngest=%d eventCount=%s cursor=%s spend=%s", lowerID, first, second, count, f.scalar(`SELECT last_usage_log_id::text FROM activity_worker_status`), f.scalar(`SELECT activity_consumption_spend(1,1)::text`))
	require.Equal(t, "2", count, "both committed consumption records must be ingested")
}

func TestRegressionTransferMustNotAbsorbEarlierUsagePostgres(t *testing.T) {
	f := accountingConsumptionFixture(t)
	f.exec(`UPDATE users SET balance=balance-10 WHERE id=1;
 INSERT INTO usage_logs(user_id,request_id,actual_cost) VALUES(1,'principal-before-transfer',10)`)
	require.NoError(t, f.transaction(func(tx *sql.Tx) error { return creditActivityWalletTx(f.ctx, tx, 1, "lottery", "review-gift", 1000) }))
	transferred, err := f.service.WithdrawWallet(f.ctx, 1)
	require.NoError(t, err)
	require.EqualValues(t, 1000, transferred.WithdrawnCents)
	_, err = f.service.IngestUsageEvents(f.ctx)
	require.NoError(t, err)
	spend := f.scalar(`SELECT activity_consumption_spend(1,1)::text`)
	t.Logf("principalConsumptionBeforeTransfer=10 transferredGift=10 spend=%s absorbed=%s giftRemaining=%s", spend, f.scalar(`SELECT absorbed_gift::text FROM activity_usage_events WHERE request_id='principal-before-transfer'`), f.scalar(`SELECT unabsorbed_gift::text FROM activity_wallets WHERE user_id=1`))
	require.Equal(t, "10.00000000", spend, "later gift transfer must not absorb earlier principal consumption")
}

func TestRegressionConsumptionPendingAndSignedEventsPostgres(t *testing.T) {
	f := accountingConsumptionFixture(t)
	// A committed legacy row below the diagnostic cursor still needs processing.
	f.exec(`UPDATE activity_worker_status SET cursor_initialized=true,last_usage_log_id=1000;
 INSERT INTO usage_logs(id,user_id,request_id,actual_cost) VALUES(10,1,'legacy-below-cursor',10)`)
	status, err := f.service.ConsumptionStatus(f.ctx, 1)
	require.NoError(t, err)
	require.True(t, status.ProcessingPending)
	_, err = f.service.IngestUsageEvents(f.ctx)
	require.NoError(t, err)
	f.exec(`UPDATE activity_consumption_outbox SET processed_at=NOW()`)
	status, err = f.service.ConsumptionStatus(f.ctx, 1)
	require.NoError(t, err)
	require.False(t, status.ProcessingPending)
	// A modern log without a transactional bill cannot earn rewards or stay pending.
	f.exec(`UPDATE activity_billing_config SET cutover_at=NOW()-interval '1 second';
 INSERT INTO usage_logs(id,user_id,request_id,actual_cost) VALUES(2000,1,'unbilled-after-cutover',10)`)
	status, err = f.service.ConsumptionStatus(f.ctx, 1)
	require.NoError(t, err)
	require.False(t, status.ProcessingPending)
	// Chronological display and correction support the new decreasing ID sequence.
	f.exec(`INSERT INTO activity_usage_events(usage_log_id,user_id,request_id,actual_cost,eligible_cost,occurred_at,billing_recorded)
 VALUES(-1,1,'older-bill',10,10,NOW()-interval '1 minute',true),(-2,1,'newer-bill',10,10,NOW(),true)`)
	ledger, err := f.service.ConsumptionUsageLedger(f.ctx, 1)
	require.NoError(t, err)
	require.EqualValues(t, -2, ledger[0]["id"])
	require.NoError(t, f.service.CorrectConsumption(f.ctx, 99, -2, "duplicate bill"))
	require.Equal(t, "true", f.scalar(`SELECT (reversed_at IS NOT NULL)::text FROM activity_usage_events WHERE usage_log_id=-2`))
	require.Equal(t, "duplicate bill", f.scalar(`SELECT reverse_note FROM activity_usage_events WHERE usage_log_id=-2`))
}
