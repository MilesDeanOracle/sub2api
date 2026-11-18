package service

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

// Opt in with ACTIVITY_TEST_DSN. Each run owns an isolated schema and never
// modifies application tables, balances or campaign configuration.
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
	exec(`CREATE TABLE users(id BIGINT PRIMARY KEY,balance NUMERIC(20,8) NOT NULL DEFAULT 0,frozen_balance NUMERIC(20,8) NOT NULL DEFAULT 0,status TEXT NOT NULL DEFAULT 'active',deleted_at TIMESTAMPTZ,created_at TIMESTAMPTZ DEFAULT NOW(),updated_at TIMESTAMPTZ DEFAULT NOW());
 CREATE TABLE payment_orders(id BIGINT PRIMARY KEY,user_id BIGINT,amount NUMERIC(20,8),bonus_amount NUMERIC(20,8),status TEXT,order_type TEXT,paid_at TIMESTAMPTZ,recharge_code TEXT);
 CREATE TABLE redeem_codes(code TEXT PRIMARY KEY,status TEXT,type TEXT,value NUMERIC(20,8),used_by BIGINT);
 CREATE TABLE usage_logs(user_id BIGINT,request_id TEXT,actual_cost NUMERIC,request_type INT,created_at TIMESTAMPTZ);
 INSERT INTO users(id,balance) VALUES(1,120);`)
	for _, name := range []string{"257_activity_rewards.sql", "258_activity_checkin_min_spend.sql", "259_consumption_rewards.sql", "260_consumption_refund_lock_order.sql", "261_consumption_batch_rounding.sql", "262_activity_production_hardening.sql", "263_activity_audiences.sql"} {
		b, err := os.ReadFile(filepath.Join("..", "..", "migrations", name))
		require.NoError(t, err)
		exec(string(b))
	}
	exec(`UPDATE activity_runtime SET processing_enabled=true,awards_enabled=true WHERE id=1; UPDATE activity_audiences SET mode='all'`)
	s := NewActivityService(db, nil, nil)
	ctx := context.Background()
	drain := func() {
		t.Helper()
		for i := 0; i < 100; i++ {
			ok, err := s.ProcessConsumptionEvent(ctx)
			require.NoError(t, err)
			if !ok {
				return
			}
		}
		t.Fatal("queue did not drain")
	}
	settle := func(uid int64, amount, request string, kind int) {
		t.Helper()
		tx, err := db.Begin()
		require.NoError(t, err)
		defer tx.Rollback()
		_, err = tx.Exec(`SELECT set_config('activity.settlement',json_build_object('kind','usage','request',$1::text,'key',1,'request_type',$2::int)::text,true)`, request, kind)
		require.NoError(t, err)
		_, err = tx.Exec(`UPDATE users SET balance=balance-$2::numeric WHERE id=$1`, uid, amount)
		require.NoError(t, err)
		require.NoError(t, tx.Commit())
		drain()
	}

	t.Run("audiences fail closed and validate real users", func(t *testing.T) {
		exec(`UPDATE activity_audiences SET mode='selected',user_ids='{}'`)
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
		require.Error(t, s.UpdateAudience(ctx, 99, "checkin", ActivityAudience{Mode: "selected", UserIDs: []int64{-1}}))
		require.Error(t, s.UpdateAudience(ctx, 99, "checkin", ActivityAudience{Mode: "typo"}))
		require.NoError(t, s.UpdateAudience(ctx, 99, "checkin", ActivityAudience{Mode: "selected", UserIDs: []int64{1, 1}}))
		audience, err := s.Audience(ctx, "checkin")
		require.NoError(t, err)
		require.Equal(t, []int64{1}, audience.UserIDs)
		tx, err := db.BeginTx(ctx, nil)
		require.NoError(t, err)
		require.NoError(t, activityAudienceAllowed(ctx, tx, "checkin", 1))
		require.NoError(t, tx.Rollback())
		require.Equal(t, "false", scalar(`SELECT activity_audience_allowed('consumption',1)::text`))
		require.Equal(t, "1", scalar(`SELECT COUNT(*)::text FROM activity_consumption_audit WHERE action='audience:checkin'`))
		exec(`UPDATE activity_audiences SET mode='all'`)
	})
	t.Run("opening and mixed sources", func(t *testing.T) {
		require.Equal(t, "120.00000000", scalar(`SELECT unverified::text FROM activity_funds WHERE user_id=1`))
		require.NoError(t, s.ReconcileConsumptionFunds(ctx, 99, 1, "100", "principal", "paid invoice"))
		exec(`UPDATE users SET balance=balance+5 WHERE id=1`)
		settle(1, "30", "mixed", 0)
		require.Equal(t, "5.00000000", scalar(`SELECT eligible_cost::text FROM activity_fund_ledger WHERE request_id='mixed'`))
		require.Equal(t, "95.00000000", scalar(`SELECT principal::text FROM activity_funds WHERE user_id=1`))
		require.Error(t, s.ReconcileConsumptionFunds(ctx, 99, 1, "1", "principal", "cannot exceed unverified"))
		require.Equal(t, "95.00000000", scalar(`SELECT balance::text FROM users WHERE id=1`))
	})
	t.Run("verified recharge and refund", func(t *testing.T) {
		exec(`INSERT INTO users(id) VALUES(2);INSERT INTO payment_orders VALUES(2,2,25,5,'RECHARGING','balance',NOW(),'paid');INSERT INTO redeem_codes VALUES('paid','unused','balance',25,NULL)`)
		tx, err := db.Begin()
		require.NoError(t, err)
		defer tx.Rollback()
		_, err = tx.Exec(`UPDATE redeem_codes SET status='used',used_by=2 WHERE code='paid';UPDATE users SET balance=balance+25 WHERE id=2`)
		require.NoError(t, err)
		require.NoError(t, tx.Commit())
		require.Equal(t, "20.00000000", scalar(`SELECT principal::text FROM activity_funds WHERE user_id=2`))
		require.Equal(t, "5.00000000", scalar(`SELECT gift::text FROM activity_funds WHERE user_id=2`))
		require.Equal(t, "20.00000000", scalar(`SELECT activity_refund_deduct(2,100)::numeric(20,8)::text`))
		require.Equal(t, "5.00000000", scalar(`SELECT balance::text FROM users WHERE id=2`))
		exec(`SELECT activity_refund_restore(2,20)`)
		require.Equal(t, "20.00000000", scalar(`SELECT principal::text FROM activity_funds WHERE user_id=2`))
		exec(`UPDATE payment_orders SET status='REFUND_REQUESTED' WHERE id=2`)
		require.Equal(t, "pending", scalar(`SELECT review_status FROM activity_refund_reviews WHERE user_id=2`))
	})
	exec(`UPDATE activity_config SET total_budget_cents=100000,user_lifetime_cap_cents=10000;
 INSERT INTO activity_consumption_campaigns(id,name,starts_at,ends_at,claim_until,rollout_percent,budget_cents,state,published_at) VALUES(1,'test',NOW()-interval '1 hour',NOW()+interval '30 days',NOW()+interval '37 days',100,10000,'active',NOW());
 INSERT INTO activity_consumption_tiers VALUES(1,1,1000,30),(1,2,5000,120),(1,3,10000,250);
 INSERT INTO users(id,balance) VALUES(3,100),(4,100),(5,100),(6,100);
 UPDATE activity_funds SET principal=gift,gift=0 WHERE user_id IN(3,4,5,6)`)

	t.Run("scope controls visibility claims and worker reservations independently", func(t *testing.T) {
		exec(`INSERT INTO users(id,balance) VALUES(901,100),(902,100);UPDATE activity_funds SET principal=gift,gift=0 WHERE user_id IN(901,902)`)
		require.NoError(t, s.UpdateAudience(ctx, 99, "consumption", ActivityAudience{Mode: "selected", UserIDs: []int64{901}}))
		settle(902, "10", "audience-excluded", 0)
		settle(901, "10", "audience-included", 0)
		require.Equal(t, "0", scalar(`SELECT COUNT(*)::text FROM activity_consumption_entitlements WHERE user_id=902`))
		require.Equal(t, "1", scalar(`SELECT COUNT(*)::text FROM activity_consumption_entitlements WHERE user_id=901`))
		hidden, err := s.ConsumptionStatus(ctx, 902)
		require.NoError(t, err)
		require.Nil(t, hidden.Campaign)
		visible, err := s.ConsumptionStatus(ctx, 901)
		require.NoError(t, err)
		require.NotNil(t, visible.Campaign)
		require.ErrorIs(t, s.ClaimConsumption(ctx, 902, 1, 0), ErrActivityIneligible)
		require.NoError(t, s.UpdateAudience(ctx, 99, "consumption", ActivityAudience{Mode: "selected", UserIDs: []int64{}}))
		require.ErrorIs(t, s.ClaimConsumption(ctx, 901, 1, 0), ErrActivityIneligible)
		require.Equal(t, "1", scalar(`SELECT COUNT(*)::text FROM activity_consumption_entitlements WHERE user_id=901 AND state='reserved'`))
		require.Equal(t, "true", scalar(`SELECT activity_audience_allowed('checkin',902)::text`))
		require.NoError(t, s.UpdateAudience(ctx, 99, "consumption", ActivityAudience{Mode: "all"}))
		visible, err = s.ConsumptionStatus(ctx, 902)
		require.NoError(t, err)
		require.NotNil(t, visible.Campaign)
		exec(`DELETE FROM activity_consumption_entitlements WHERE user_id=901`)
	})
	t.Run("precision and concurrent claims", func(t *testing.T) {
		settle(3, "9.99999999", "boundary-before", 0)
		require.Equal(t, "0", scalar(`SELECT COUNT(*)::text FROM activity_consumption_entitlements WHERE user_id=3`))
		settle(3, "0.00000001", "boundary-exact", 0)
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
		require.Equal(t, "1", scalar(`SELECT COUNT(*)::text FROM activity_reward_ledger WHERE user_id=3`))
		require.Equal(t, "90.30000000", scalar(`SELECT balance::text FROM users WHERE id=3`))
		require.Equal(t, "0.30000000", scalar(`SELECT gift::text FROM activity_funds WHERE user_id=3`))
		settle(3, "0.3", "gift-spent", 0)
		require.Equal(t, "0.00000000", scalar(`SELECT eligible_cost::text FROM activity_fund_ledger WHERE request_id='gift-spent'`))
		require.NoError(t, s.CorrectConsumption(ctx, 99, mustLedgerID(t, db, "boundary-before"), "upstream credit"))
		require.Equal(t, "review", scalar(`SELECT state FROM activity_consumption_entitlements WHERE user_id=3`))
		require.NoError(t, s.ResolveConsumptionReview(ctx, 99, 3, false, "paid reward handled manually"))
	})
	t.Run("cyber excluded and budget full rewards", func(t *testing.T) {
		settle(4, "20", "cyber", 4)
		require.Equal(t, "0", scalar(`SELECT COUNT(*)::text FROM activity_consumption_entitlements WHERE user_id=4`))
		exec(`UPDATE activity_consumption_campaigns SET budget_cents=60 WHERE id=1`)
		settle(4, "60", "ordinary", 0)
		require.Equal(t, "1", scalar(`SELECT COUNT(*)::text FROM activity_consumption_entitlements WHERE user_id=4`))
		require.Equal(t, "30", scalar(`SELECT activity_reserved_cents()::text`))
		settle(5, "10", "budget-exhausted", 0)
		require.Equal(t, "0", scalar(`SELECT COUNT(*)::text FROM activity_consumption_entitlements WHERE user_id=5`))
		exec(`UPDATE activity_consumption_campaigns SET budget_cents=10000 WHERE id=1`)
	})
	t.Run("batch hold capture and release", func(t *testing.T) {
		exec(`UPDATE users SET balance=balance+5 WHERE id=6`)
		batch := func(id string, query string) {
			t.Helper()
			tx, err := db.Begin()
			require.NoError(t, err)
			defer tx.Rollback()
			_, err = tx.Exec(`SELECT set_config('activity.settlement',json_build_object('kind','batch','batch',$1::text,'request',$1::text,'key',1)::text,true)`, id)
			require.NoError(t, err)
			_, err = tx.Exec(query)
			require.NoError(t, err)
			require.NoError(t, tx.Commit())
			drain()
		}
		batch("batch1", `UPDATE users SET balance=balance-20,frozen_balance=frozen_balance+20 WHERE id=6`)
		require.Equal(t, "0", scalar(`SELECT COUNT(*)::text FROM activity_consumption_entitlements WHERE user_id=6`))
		batch("batch1", `UPDATE users SET balance=balance+10,frozen_balance=frozen_balance-20 WHERE id=6`)
		require.Equal(t, "5.00000000", scalar(`SELECT eligible_cost::text FROM activity_fund_ledger WHERE user_id=6 AND source='batch_capture'`))
		require.Equal(t, "95.00000000", scalar(`SELECT principal::text FROM activity_funds WHERE user_id=6`))
		batch("batch2", `UPDATE users SET balance=balance-20,frozen_balance=frozen_balance+20 WHERE id=6`)
		batch("batch2", `UPDATE users SET balance=balance+20,frozen_balance=frozen_balance-20 WHERE id=6`)
		require.Equal(t, "95.00000000", scalar(`SELECT principal::text FROM activity_funds WHERE user_id=6`))
		require.Equal(t, "0", scalar(`SELECT COUNT(*)::text FROM activity_fund_holds WHERE user_id=6`))
		batch("batch-round", `UPDATE users SET balance=balance-20,frozen_balance=frozen_balance+20 WHERE id=6`)
		batch("batch-round", `UPDATE users SET balance=balance-0.00000001,frozen_balance=frozen_balance-20 WHERE id=6`)
		require.Equal(t, scalar(`SELECT balance::text FROM users WHERE id=6`), scalar(`SELECT principal::text FROM activity_funds WHERE user_id=6`))
	})
	t.Run("cross all tiers and respect shared commitments", func(t *testing.T) {
		settle(5, "90", "cross-all", 0)
		require.Equal(t, "400", scalar(`SELECT activity_reserved_cents(5)::text`))
		require.NoError(t, s.ClaimConsumption(ctx, 5, 1, 0))
		require.NoError(t, s.ClaimConsumption(ctx, 5, 1, 0))
		require.Equal(t, "3", scalar(`SELECT COUNT(*)::text FROM activity_reward_ledger WHERE user_id=5`))
		require.Equal(t, "4.00000000", scalar(`SELECT gift::text FROM activity_funds WHERE user_id=5`))
		exec(`UPDATE activity_config SET total_budget_cents=490`)
		settle(4, "10", "shared-cap", 0)
		require.Equal(t, "0", scalar(`SELECT COUNT(*)::text FROM activity_consumption_entitlements WHERE user_id=4 AND position=2`))
		require.Equal(t, "490", scalar(`SELECT (COALESCE(SUM(amount_cents),0)+activity_reserved_cents())::text FROM activity_reward_ledger`))
		exec(`UPDATE activity_config SET total_budget_cents=100000`)
	})
	t.Run("expiry frees reservations", func(t *testing.T) {
		exec(`UPDATE activity_consumption_campaigns SET starts_at=NOW()-interval '40 days',ends_at=NOW()-interval '8 days',claim_until=NOW()-interval '1 day' WHERE id=1`)
		require.Equal(t, "0", scalar(`SELECT activity_reserved_cents()::text`))
		require.NoError(t, s.ClaimConsumption(ctx, 4, 1, 0))
		require.Equal(t, "0", scalar(`SELECT COUNT(*)::text FROM activity_reward_ledger WHERE user_id=4`))
	})
	t.Run("settlement is isolated from global budget locks", func(t *testing.T) {
		exec(`INSERT INTO users(id,balance) VALUES(10,100); UPDATE activity_funds SET principal=gift,gift=0 WHERE user_id=10`)
		lock, err := db.Begin()
		require.NoError(t, err)
		defer lock.Rollback()
		_, err = lock.Exec(`SELECT id FROM activity_config WHERE id=1 FOR UPDATE`)
		require.NoError(t, err)
		debit, err := db.Begin()
		require.NoError(t, err)
		defer debit.Rollback()
		_, err = debit.Exec(`SET LOCAL lock_timeout='200ms'; SELECT set_config('activity.settlement','{"kind":"usage","request":"isolated","key":1}',true); UPDATE users SET balance=balance-1 WHERE id=10`)
		require.NoError(t, err)
		require.NoError(t, debit.Commit())
		require.NoError(t, lock.Rollback())
		require.Equal(t, "99.00000000", scalar(`SELECT balance::text FROM users WHERE id=10`))
		require.Equal(t, "1", scalar(`SELECT COUNT(*)::text FROM activity_consumption_outbox WHERE user_id=10 AND processed_at IS NULL`))
	})
	t.Run("disabled gates preserve settlement and recover backlog", func(t *testing.T) {
		exec(`UPDATE activity_runtime SET processing_enabled=false,awards_enabled=false`)
		require.ErrorIs(t, s.ClaimConsumption(ctx, 10, 1, 0), ErrActivityDisabled)
		ok, err := s.ProcessConsumptionEvent(ctx)
		require.NoError(t, err)
		require.False(t, ok)
		exec(`UPDATE activity_runtime SET processing_enabled=true,awards_enabled=true`)
		drain()
		require.Equal(t, "0", scalar(`SELECT COUNT(*)::text FROM activity_consumption_outbox WHERE user_id=10 AND processed_at IS NULL`))
	})
	t.Run("worker failures retry without rolling back settled balances", func(t *testing.T) {
		exec(`INSERT INTO users(id,balance) VALUES(11,100); UPDATE activity_funds SET principal=gift,gift=0 WHERE user_id=11`)
		// Session-local settings and updates must share a transaction.
		debit, err := db.Begin()
		require.NoError(t, err)
		_, err = debit.Exec(`SELECT set_config('activity.settlement','{"kind":"usage","request":"worker-failure","key":1}',true);UPDATE users SET balance=balance-1 WHERE id=11`)
		require.NoError(t, err)
		require.NoError(t, debit.Commit())
		exec(`CREATE OR REPLACE FUNCTION activity_process_consumption_event(event_id BIGINT) RETURNS BOOLEAN LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'simulated reward failure'; END $$`)
		ok, err := s.ProcessConsumptionEvent(ctx)
		require.NoError(t, err)
		require.True(t, ok)
		require.Equal(t, "99.00000000", scalar(`SELECT balance::text FROM users WHERE id=11`))
		require.Equal(t, "1", scalar(`SELECT attempts::text FROM activity_consumption_outbox WHERE user_id=11`))
		require.Equal(t, "1", scalar(`SELECT COUNT(*)::text FROM activity_consumption_outbox WHERE user_id=11 AND processed_at IS NULL AND last_error<>''`))
		migration, err := os.ReadFile(filepath.Join("..", "..", "migrations", "263_activity_audiences.sql"))
		require.NoError(t, err)
		function := strings.Split(string(migration), "CREATE OR REPLACE FUNCTION activity_process_consumption_event")[1]
		function = strings.Split(function, "-- A rolled-back binary")[0]
		exec("CREATE OR REPLACE FUNCTION activity_process_consumption_event" + function)
		exec(`UPDATE activity_consumption_outbox SET next_attempt_at=NOW() WHERE user_id=11`)
		drain()
		require.Equal(t, "2", scalar(`SELECT attempts::text FROM activity_consumption_outbox WHERE user_id=11`))
		require.Equal(t, "0", scalar(`SELECT COUNT(*)::text FROM activity_consumption_outbox WHERE user_id=11 AND processed_at IS NULL`))
	})
	t.Run("delayed events retain eligibility across pause and do not double reserve", func(t *testing.T) {
		exec(`INSERT INTO users(id,balance) VALUES(12,100);UPDATE activity_funds SET principal=gift,gift=0 WHERE user_id=12;
      INSERT INTO activity_consumption_campaigns(id,name,starts_at,ends_at,claim_until,rollout_percent,budget_cents,state,published_at) VALUES(3,'delayed',NOW()-interval '1 hour',NOW()+interval '1 day',NOW()+interval '2 days',100,10000,'active',NOW()-interval '2 hours');
      INSERT INTO activity_consumption_tiers VALUES(3,1,1000,30),(3,2,5000,120),(3,3,10000,250);
      UPDATE activity_runtime SET processing_enabled=false`)
		settle(12, "10", "delayed-before-pause", 0)
		require.Equal(t, "0", scalar(`SELECT COUNT(*)::text FROM activity_consumption_entitlements WHERE user_id=12`))
		exec(`UPDATE activity_consumption_campaigns SET state='paused',stopped_at=clock_timestamp() WHERE id=3;UPDATE activity_runtime SET processing_enabled=true`)
		var wg sync.WaitGroup
		errs := make(chan error, 8)
		for i := 0; i < 8; i++ {
			wg.Add(1)
			go func() { defer wg.Done(); _, err := s.ProcessConsumptionEvent(ctx); errs <- err }()
		}
		wg.Wait()
		close(errs)
		for err := range errs {
			require.NoError(t, err)
		}
		require.Equal(t, "30", scalar(`SELECT activity_reserved_cents(12)::text`))
		settle(12, "50", "delayed-after-pause", 0)
		require.Equal(t, "1", scalar(`SELECT COUNT(*)::text FROM activity_consumption_entitlements WHERE user_id=12`))
		exec(`DELETE FROM activity_consumption_entitlements WHERE campaign_id=3;DELETE FROM activity_consumption_tiers WHERE campaign_id=3;DELETE FROM activity_consumption_campaigns WHERE id=3`)
	})
	t.Run("draft publication freeze and stop", func(t *testing.T) {
		now := time.Now()
		c := ConsumptionCampaign{Name: "next campaign", StartsAt: now.Add(time.Hour), EndsAt: now.Add(31 * 24 * time.Hour), ClaimUntil: now.Add(38 * 24 * time.Hour), RolloutPercent: 10, BudgetCents: 400, Tiers: []ConsumptionTier{{Position: 1, ThresholdCents: 1000, RewardCents: 30}, {Position: 2, ThresholdCents: 5000, RewardCents: 120}, {Position: 3, ThresholdCents: 10000, RewardCents: 250}}}
		// Explicit id was used for fixtures; keep the sequence above it.
		exec(`SELECT setval(pg_get_serial_sequence('activity_consumption_campaigns','id'),1)`)
		id, err := s.SaveConsumptionCampaign(ctx, 99, c)
		require.NoError(t, err)
		c.ID = id
		require.NoError(t, s.SetConsumptionState(ctx, 99, id, "active"))
		c.Tiers[0].ThresholdCents = 2000
		_, err = s.SaveConsumptionCampaign(ctx, 99, c)
		require.NoError(t, err)
		require.Equal(t, "1000", scalar(`SELECT threshold_cents::text FROM activity_consumption_tiers WHERE campaign_id=$1 AND position=1`, id))
		c.BudgetCents = 399
		_, err = s.SaveConsumptionCampaign(ctx, 99, c)
		require.Error(t, err)
		require.NoError(t, s.SetConsumptionState(ctx, 99, id, "paused"))
		require.Error(t, s.SetConsumptionState(ctx, 99, id, "active"))
		campaigns, err := s.ConsumptionCampaigns(ctx)
		require.NoError(t, err)
		require.Len(t, campaigns, 2)
	})
}
func mustLedgerID(t *testing.T, db *sql.DB, request string) int64 {
	t.Helper()
	var id int64
	require.NoError(t, db.QueryRow(`SELECT id FROM activity_fund_ledger WHERE request_id=$1`, request).Scan(&id))
	return id
}
