package service

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

// Opt in with ACTIVITY_TEST_DSN. Each run owns an isolated schema and verifies
// that activity rewards never attach triggers or foreign keys to sub2api tables,
// accumulate in activity_wallets first, and only credit users.balance on manual withdraw.
func TestConsumptionPostgres(t *testing.T) {
	dsn := os.Getenv("ACTIVITY_TEST_DSN")
	if dsn == "" {
		t.Skip("set ACTIVITY_TEST_DSN for isolated PostgreSQL accounting tests")
	}
	root, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	defer root.Close()
	schema := fmt.Sprintf("activity_test_%d", time.Now().UnixNano())
	_, err = root.Exec(`CREATE SCHEMA ` + schema)
	require.NoError(t, err)
	defer func() { _, err := root.Exec(`DROP SCHEMA ` + schema + ` CASCADE`); require.NoError(t, err) }()
	u, err := url.Parse(dsn)
	require.NoError(t, err)
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	db, err := sql.Open("postgres", u.String())
	require.NoError(t, err)
	defer db.Close()
	exec := func(query string, args ...any) {
		t.Helper()
		_, err := db.Exec(query, args...)
		require.NoError(t, err)
	}
	scalar := func(query string, args ...any) string {
		t.Helper()
		var v string
		require.NoError(t, db.QueryRow(query, args...).Scan(&v))
		return v
	}

	exec(`
		CREATE TABLE settings(key TEXT PRIMARY KEY, value TEXT NOT NULL);
		CREATE TABLE users(id BIGINT PRIMARY KEY, balance NUMERIC(20,8) NOT NULL DEFAULT 0, frozen_balance NUMERIC(20,8) NOT NULL DEFAULT 0, status TEXT NOT NULL DEFAULT 'active', deleted_at TIMESTAMPTZ, created_at TIMESTAMPTZ DEFAULT NOW(), updated_at TIMESTAMPTZ DEFAULT NOW());
		CREATE TABLE payment_orders(id BIGINT PRIMARY KEY, user_id BIGINT, amount NUMERIC(20,8), pay_amount NUMERIC(20,8), bonus_amount NUMERIC(20,8), refund_amount NUMERIC(20,8), status TEXT, order_type TEXT, paid_at TIMESTAMPTZ, completed_at TIMESTAMPTZ, provider_snapshot JSONB, recharge_code TEXT, updated_at TIMESTAMPTZ DEFAULT NOW());
		CREATE TABLE usage_logs(id BIGSERIAL PRIMARY KEY, user_id BIGINT, api_key_id BIGINT DEFAULT 1, request_id TEXT, actual_cost NUMERIC(20,10), billing_type INT DEFAULT 0, request_type INT DEFAULT 0, created_at TIMESTAMPTZ DEFAULT clock_timestamp());
		INSERT INTO users(id, balance) VALUES(1, 120);
	`)
	for _, migration := range []string{"258_activity_rewards.sql", "259_activity_lottery.sql", "260_activity_independent_budgets.sql", "264_activity_production_safety.sql", "266_activity_consumption_master_switch.sql"} {
		b, err := os.ReadFile(filepath.Join("..", "..", "migrations", migration))
		require.NoError(t, err)
		exec(string(b))
	}

	// Hard invariant: ZERO triggers and ZERO incoming foreign keys on sub2api core tables.
	require.Equal(t, "0", scalar(`
		SELECT COUNT(*)::text
		FROM pg_trigger tg
		JOIN pg_class cls ON cls.oid = tg.tgrelid
		JOIN pg_namespace ns ON ns.oid = cls.relnamespace
		WHERE ns.nspname = $1
		  AND NOT tg.tgisinternal
		  AND cls.relname IN ('users', 'usage_logs', 'payment_orders', 'settings')
	`, schema))
	require.Equal(t, "0", scalar(`
		SELECT COUNT(*)::text
		FROM pg_constraint con
		JOIN pg_class ref_cls ON ref_cls.oid = con.confrelid
		JOIN pg_namespace ns ON ns.oid = ref_cls.relnamespace
		WHERE ns.nspname = $1
		  AND con.contype = 'f'
		  AND ref_cls.relname IN ('users', 'usage_logs', 'payment_orders', 'settings')
	`, schema))

	exec(`UPDATE activity_audiences SET mode='all'`)
	s := NewActivityService(db, nil, nil)
	ctx := context.Background()
	_, err = s.IngestUsageEvents(ctx)
	require.NoError(t, err)

	drain := func() {
		t.Helper()
		require.NoError(t, s.SyncRefundReviews(ctx))
		for i := 0; i < 20; i++ {
			n, err := s.IngestUsageEvents(ctx)
			require.NoError(t, err)
			if n == 0 {
				break
			}
		}
		for i := 0; i < 100; i++ {
			ok, err := s.ProcessConsumptionEvent(ctx)
			require.NoError(t, err)
			if !ok {
				return
			}
		}
		t.Fatal("queue did not drain")
	}

	recordUsage := func(uid int64, amount, request string, requestType int) {
		t.Helper()
		exec(`UPDATE users SET balance = balance - $2::numeric WHERE id = $1`, uid, amount)
		exec(`INSERT INTO usage_logs(user_id, request_id, actual_cost, billing_type, request_type) VALUES($1, $2, $3::numeric, 0, $4)`, uid, request, amount, requestType)
		drain()
	}

	t.Run("audiences fail closed and validate real users", func(t *testing.T) {
		exec(`UPDATE activity_audiences SET mode='selected', user_ids='{}'`)
		status, err := s.Status(ctx, 1)
		require.NoError(t, err)
		require.False(t, status.CheckinVisible)
		_, err = s.Checkin(ctx, 1)
		require.ErrorIs(t, err, ErrActivityIneligible)
		summary, err := s.ConsumptionStatus(ctx, 1)
		require.NoError(t, err)
		require.Nil(t, summary.Campaign)
		require.Empty(t, summary.History)
		require.Error(t, s.UpdateAudience(ctx, 99, "checkin", ActivityAudience{Mode: "selected", UserIDs: []int64{999999}}))
		require.NoError(t, s.UpdateAudience(ctx, 99, "checkin", ActivityAudience{Mode: "selected", UserIDs: []int64{1, 1}}))
		audience, err := s.Audience(ctx, "checkin")
		require.NoError(t, err)
		require.Equal(t, []int64{1}, audience.UserIDs)
		exec(`UPDATE activity_audiences SET mode='all'`)
	})

	t.Run("checkin requires completed recharge when enabled and credits isolated wallet first", func(t *testing.T) {
		exec(`UPDATE activity_config SET checkin_enabled=true, checkin_require_recharge=true, checkin_rollout_percent=100, checkin_min_spend_cents=0, checkin_daily_cents=25, checkin_milestone_cents=50, checkin_monthly_cap_cents=1500, checkin_monthly_budget_cents=10000, total_budget_cents=0, user_lifetime_cap_cents=10000 WHERE id=1`)
		// Without a completed recharge order, check-in is blocked against trial-credit farming.
		_, err := s.Checkin(ctx, 1)
		require.ErrorIs(t, err, ErrActivityIneligible)
		exec(`INSERT INTO payment_orders(id, user_id, amount, pay_amount, status, order_type, paid_at) VALUES(100, 1, 10, 70, 'COMPLETED', 'balance', NOW())`)

		st, err := s.Checkin(ctx, 1)
		require.NoError(t, err)
		require.True(t, st.CheckedToday)
		require.Equal(t, 0.25, st.DailyReward)
		require.Equal(t, 0.50, st.MilestoneReward)
		require.Equal(t, int64(25), st.WalletBalanceCents)
		// Existing shared budget may remain zero even after rewards have been paid.
		cfg, err := s.Config(ctx)
		require.NoError(t, err)
		require.NoError(t, s.UpdateConfig(ctx, *cfg))
		// users.balance is untouched until manual withdrawal!
		require.Equal(t, "120.00000000", scalar(`SELECT balance::text FROM users WHERE id=1`))

		w, err := s.WithdrawWallet(ctx, 1)
		require.NoError(t, err)
		require.Equal(t, int64(25), w.WithdrawnCents)
		require.Equal(t, int64(0), w.AvailableCents)
		require.Equal(t, "120.25000000", scalar(`SELECT balance::text FROM users WHERE id=1`))
		require.Equal(t, "0.25000000", scalar(`SELECT unabsorbed_gift::text FROM activity_wallets WHERE user_id=1`))
		_, err = s.WithdrawWallet(ctx, 1)
		require.ErrorIs(t, err, ErrActivityWalletEmpty)

		// Idle-mode optimization: with no active consumption campaign yet, usage advances cursor
		// and burns unabsorbed_gift in-place without bloating activity_usage_events.
		recordUsage(1, "0.05", "idle-burn-gift", 0)
		require.Equal(t, "0", scalar(`SELECT COUNT(*)::text FROM activity_usage_events`))
		require.Equal(t, "0.20000000", scalar(`SELECT unabsorbed_gift::text FROM activity_wallets WHERE user_id=1`))
		exec(`UPDATE activity_wallets SET unabsorbed_gift=0.25 WHERE user_id=1`)

		_, err = s.UpdateRuntime(ctx, 99, false, true)
		require.Error(t, err)
		h, err := s.UpdateRuntime(ctx, 99, true, true)
		require.NoError(t, err)
		require.True(t, h.ProcessingEnabled && h.AwardsEnabled)
	})

	t.Run("lottery uses only its dedicated budget with a zero shared budget", func(t *testing.T) {
		exec(`INSERT INTO users(id,balance) VALUES(777,100)`)
		cfg, err := s.UpdateAdminLotteryConfig(ctx, 99, AdminLotteryConfig{
			Enabled: true, UnitPayCents: 1000, BudgetCents: 5000, Tiers: defaultLotteryTiers(),
		})
		require.NoError(t, err)
		require.Equal(t, int64(5000), cfg.BudgetCents)
		exec(`INSERT INTO payment_orders(id,user_id,amount,pay_amount,status,order_type,paid_at) VALUES(777,777,10,10,'COMPLETED','balance',clock_timestamp())`)
		status, err := s.LotteryStatus(ctx, 777)
		require.NoError(t, err)
		require.False(t, status.PoolPaused)
		require.Equal(t, 1, status.RemainingDraws)
		draw, err := s.DrawLottery(ctx, 777, 1, "independent-budget")
		require.NoError(t, err)
		require.Positive(t, draw.TotalRewardCents)
		require.Equal(t, 0, draw.Status.RemainingDraws)
		cfg.Enabled = false
		_, err = s.UpdateAdminLotteryConfig(ctx, 99, *cfg)
		require.NoError(t, err)
	})

	exec(`INSERT INTO activity_consumption_campaigns(id,name,starts_at,ends_at,claim_until,rollout_percent,budget_cents,state,published_at) VALUES(1,'test',NOW()-interval '1 hour',NOW()+interval '30 days',NOW()+interval '37 days',100,10000,'active',NOW()-interval '2 hours');
		INSERT INTO activity_consumption_tiers VALUES(1,1,1000,30),(1,2,5000,120),(1,3,10000,250);
		INSERT INTO users(id,balance) VALUES(2,100),(3,100),(4,100),(5,100),(11,100),(12,100);`)
	exec(`SELECT setval(pg_get_serial_sequence('activity_consumption_campaigns', 'id'), 1)`)

	t.Run("withdrawn gift is absorbed before counting milestone progress", func(t *testing.T) {
		// User 1 withdrew $0.25 gift above; spending $10.10 should absorb $0.25 and count $9.85 (< $10 tier 1).
		recordUsage(1, "10.10", "absorb-check-1", 0)
		require.Equal(t, "9.85000000", scalar(`SELECT activity_consumption_spend(1, 1)::text`))
		require.Equal(t, "0", scalar(`SELECT COUNT(*)::text FROM activity_consumption_entitlements WHERE user_id=1`))
		// Spending another $0.15 reaches exact $10.00 eligible spend!
		recordUsage(1, "0.15", "absorb-check-2", 0)
		require.Equal(t, "10.00000000", scalar(`SELECT activity_consumption_spend(1, 1)::text`))
		require.Equal(t, "30", scalar(`SELECT activity_reserved_cents(1)::text`))
	})

	t.Run("read-only refund sync ignores REFUND_FAILED and blocks real refunds until reviewed", func(t *testing.T) {
		recordUsage(2, "12", "user2-spend", 0)
		require.Equal(t, "30", scalar(`SELECT activity_reserved_cents(2)::text`))
		exec(`INSERT INTO payment_orders(id,user_id,amount,status,order_type,paid_at,updated_at) VALUES(199,2,25,'REFUND_FAILED','balance',NOW(),NOW())`)
		drain()
		require.Equal(t, "0", scalar(`SELECT COUNT(*)::text FROM activity_refund_reviews WHERE order_id=199`))
		exec(`INSERT INTO payment_orders(id,user_id,amount,status,order_type,paid_at,updated_at) VALUES(200,2,25,'REFUND_REQUESTED','balance',NOW(),NOW())`)
		// Even before worker sync, ClaimConsumption detects refund order synchronously!
		require.ErrorIs(t, s.ClaimConsumption(ctx, 2, 1, 0), ErrActivityReviewBlocked)
		drain()
		require.Equal(t, "pending", scalar(`SELECT review_status FROM activity_refund_reviews WHERE order_id=200`))
		require.Equal(t, "review", scalar(`SELECT state FROM activity_consumption_entitlements WHERE user_id=2 AND position=1`))
		require.ErrorIs(t, s.ResolveConsumptionReview(ctx, 99, 2, true, "still pending"), ErrActivityReviewBlocked)
		exec(`UPDATE payment_orders SET status='PARTIALLY_REFUNDED',refund_amount=5,updated_at=NOW() WHERE id=200`)
		drain()
		require.NoError(t, s.ResolveConsumptionReview(ctx, 99, 2, true, "partial refund unrelated"))
		require.NoError(t, s.ClaimConsumption(ctx, 2, 1, 0))
		require.Equal(t, "30", scalar(`SELECT available_cents::text FROM activity_wallets WHERE user_id=2`))
	})

	t.Run("selected allowlist bypasses rollout percent and past start aligns to now on publish", func(t *testing.T) {
		// User 188 has 188 % 100 = 88 >= 10% rollout, yet must qualify when explicitly allowlisted.
		exec(`INSERT INTO users(id, balance) VALUES(188, 100); UPDATE activity_config SET checkin_require_recharge=false, checkin_rollout_percent=10 WHERE id=1`)
		require.NoError(t, s.UpdateAudience(ctx, 99, "checkin", ActivityAudience{Mode: "selected", UserIDs: []int64{188}}))
		st, err := s.Checkin(ctx, 188)
		require.NoError(t, err)
		require.True(t, st.CheckedToday)
		exec(`UPDATE activity_audiences SET mode='all' WHERE kind='checkin'`)

		now := time.Now()
		flash := ConsumptionCampaign{
			Name: "flash-1-tier", StartsAt: now.Add(-5 * time.Minute), EndsAt: now.Add(24 * time.Hour), ClaimUntil: now.Add(49 * time.Hour),
			RolloutPercent: 10, BudgetCents: 500, Tiers: []ConsumptionTier{{Position: 1, ThresholdCents: 500, RewardCents: 50}},
		}
		cid, err := s.SaveConsumptionCampaign(ctx, 99, flash)
		require.NoError(t, err)
		exec(`UPDATE activity_consumption_campaigns SET state='closed', stopped_at=NOW() WHERE id=1`)
		require.NoError(t, s.SetConsumptionState(ctx, 99, cid, "active"))
		flash.ID = cid
		flash.BudgetCents = 1000
		_, err = s.SaveConsumptionCampaign(ctx, 99, flash)
		require.NoError(t, err, "published budget increases are independent of the legacy shared budget")
		require.NoError(t, s.UpdateAudience(ctx, 99, "consumption", ActivityAudience{Mode: "selected", UserIDs: []int64{188}}))
		recordUsage(188, "6", "allowlist-188", 0)
		require.Equal(t, "50", scalar(`SELECT reward_cents::text FROM activity_consumption_entitlements WHERE campaign_id=$1 AND user_id=188`, cid))
		exec(`UPDATE activity_consumption_campaigns SET state='closed', stopped_at=NOW() WHERE id=$1`, cid)
		exec(`UPDATE activity_consumption_campaigns SET state='active', stopped_at=NULL WHERE id=1; UPDATE activity_audiences SET mode='all' WHERE kind='consumption'`)
	})

	t.Run("precision, concurrent claims into wallet, concurrent withdrawals, and reversal", func(t *testing.T) {
		recordUsage(3, "9.99999999", "boundary-before", 0)
		require.Equal(t, "0", scalar(`SELECT COUNT(*)::text FROM activity_consumption_entitlements WHERE user_id=3`))
		recordUsage(3, "0.00000001", "boundary-exact", 0)
		require.Equal(t, "30", scalar(`SELECT activity_reserved_cents(3)::text`))

		var wg sync.WaitGroup
		errs := make(chan error, 12)
		for i := 0; i < 12; i++ {
			wg.Add(1)
			go func() { defer wg.Done(); errs <- s.ClaimConsumption(ctx, 3, 1, 0) }()
		}
		wg.Wait()
		close(errs)
		for err := range errs {
			require.NoError(t, err)
		}
		require.Equal(t, "30", scalar(`SELECT available_cents::text FROM activity_wallets WHERE user_id=3`))
		require.Equal(t, "90.00000000", scalar(`SELECT balance::text FROM users WHERE id=3`))

		// Concurrent withdrawals: exactly 1 succeeds, others get ErrActivityWalletEmpty.
		wResults := make(chan error, 8)
		for i := 0; i < 8; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, e := s.WithdrawWallet(ctx, 3)
				wResults <- e
			}()
		}
		wg.Wait()
		close(wResults)
		successCount := 0
		for e := range wResults {
			if e == nil {
				successCount++
			} else {
				require.ErrorIs(t, e, ErrActivityWalletEmpty)
			}
		}
		require.Equal(t, 1, successCount)
		require.Equal(t, "90.30000000", scalar(`SELECT balance::text FROM users WHERE id=3`))

		var eventID int64
		require.NoError(t, db.QueryRow(`SELECT usage_log_id FROM activity_usage_events WHERE request_id='boundary-before'`).Scan(&eventID))
		require.NoError(t, s.CorrectConsumption(ctx, 99, eventID, "upstream credit"))
		require.Equal(t, "review", scalar(`SELECT state FROM activity_consumption_entitlements WHERE user_id=3`))
		require.NoError(t, s.ResolveConsumptionReview(ctx, 99, 3, false, "paid reward handled manually"))
	})

	t.Run("cyber excluded and worker retry resilience", func(t *testing.T) {
		recordUsage(4, "20", "cyber", 4)
		require.Equal(t, "0", scalar(`SELECT COUNT(*)::text FROM activity_consumption_entitlements WHERE user_id=4`))

		exec(`INSERT INTO usage_logs(user_id, request_id, actual_cost, billing_type, request_type) VALUES(11, 'worker-failure', 15, 0, 0)`)
		_, err := s.IngestUsageEvents(ctx)
		require.NoError(t, err)
		exec(`CREATE OR REPLACE FUNCTION activity_process_consumption_event(p_event_id BIGINT) RETURNS BOOLEAN LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'simulated reward failure'; END $$`)
		ok, err := s.ProcessConsumptionEvent(ctx)
		require.NoError(t, err)
		require.True(t, ok)
		require.Equal(t, "1", scalar(`SELECT attempts::text FROM activity_consumption_outbox WHERE user_id=11`))
		function, err := os.ReadFile(filepath.Join("..", "..", "migrations", "266_activity_consumption_master_switch.sql"))
		require.NoError(t, err)
		exec(string(function))
		exec(`UPDATE activity_consumption_outbox SET next_attempt_at=NOW() WHERE user_id=11`)
		drain()
		require.Equal(t, "0", scalar(`SELECT COUNT(*)::text FROM activity_consumption_outbox WHERE user_id=11 AND processed_at IS NULL`))
		require.Equal(t, "30", scalar(`SELECT activity_reserved_cents(11)::text`))
	})
}
