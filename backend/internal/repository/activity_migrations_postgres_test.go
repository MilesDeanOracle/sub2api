package repository

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/repository/activitystore"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/Wei-Shaw/sub2api/migrations"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

func TestActivityMigrationLineagesPostgres(t *testing.T) {
	t.Setenv("ACTIVITY_MAINTENANCE_CONFIRMED", "1")
	dsn := os.Getenv("ACTIVITY_TEST_DSN")
	if dsn == "" {
		t.Skip("set ACTIVITY_TEST_DSN for isolated migration tests")
	}
	for _, lineage := range []string{"fresh", "legacy", "legacy_partial", "isolated"} {
		t.Run(lineage, func(t *testing.T) {
			ctx := context.Background()
			root, err := sql.Open("postgres", dsn)
			require.NoError(t, err)
			schema := fmt.Sprintf("activity_upgrade_%d", time.Now().UnixNano())
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
				_, e := root.Exec(`DROP SCHEMA ` + schema + ` CASCADE`)
				require.NoError(t, e)
				require.NoError(t, root.Close())
			})
			exec := func(query string, args ...any) { t.Helper(); _, e := db.Exec(query, args...); require.NoError(t, e) }
			exec(`CREATE TABLE settings(key TEXT PRIMARY KEY,value TEXT NOT NULL);
   CREATE TABLE users(id BIGINT PRIMARY KEY,balance NUMERIC(20,8) NOT NULL DEFAULT 0,frozen_balance NUMERIC(20,8) NOT NULL DEFAULT 0,status TEXT DEFAULT 'active',deleted_at TIMESTAMPTZ,created_at TIMESTAMPTZ DEFAULT NOW(),updated_at TIMESTAMPTZ DEFAULT NOW());
   CREATE TABLE usage_logs(id BIGSERIAL PRIMARY KEY,user_id BIGINT,api_key_id BIGINT DEFAULT 1,request_id TEXT,actual_cost NUMERIC(20,10),billing_type INT DEFAULT 0,request_type INT DEFAULT 0,created_at TIMESTAMPTZ DEFAULT NOW());
   CREATE TABLE redeem_codes(id BIGINT PRIMARY KEY,status TEXT);`)
			paymentSQL, err := migrations.FS.ReadFile("092_payment_orders.sql")
			require.NoError(t, err)
			exec(string(paymentSQL))
			exec(`ALTER TABLE payment_orders ADD COLUMN bonus_amount NUMERIC DEFAULT 0,ADD COLUMN provider_key TEXT,ADD COLUMN provider_snapshot JSONB`)
			exec(schemaMigrationsTableDDL)
			exec(atlasSchemaRevisionsTableDDL)
			files, err := fs.Glob(migrations.FS, "*activity*.sql")
			require.NoError(t, err)
			// Legacy consumption migrations have different names and are intentionally
			// included unchanged to exercise the runner's historical checksum validation.
			legacy := []string{"257_activity_rewards.sql", "258_activity_checkin_min_spend.sql", "259_consumption_rewards.sql", "260_consumption_refund_lock_order.sql", "261_consumption_batch_rounding.sql", "262_activity_production_hardening.sql", "263_activity_audiences.sql"}
			all := fstest.MapFS{}
			for _, name := range append(files, legacy...) {
				raw, e := migrations.FS.ReadFile(name)
				require.NoError(t, e)
				all[name] = &fstest.MapFile{Data: raw}
			}
			apply := func(name string) {
				t.Helper()
				raw := all[name].Data
				exec(string(raw))
				checksum := fmt.Sprintf("%x", sha256.Sum256([]byte(strings.TrimSpace(string(raw)))))
				exec(`INSERT INTO schema_migrations(filename,checksum) VALUES($1,$2)`, name, checksum)
			}
			if strings.HasPrefix(lineage, "legacy") {
				exec(`INSERT INTO users(id,balance) VALUES(1,100)`)
				for i, name := range legacy {
					if lineage == "legacy_partial" && i > 1 {
						break
					}
					apply(name)
				}
				// A historical award is already part of users.balance and must not be
				// credited to the new wallet a second time.
				exec(`INSERT INTO activity_reward_ledger(user_id,kind,source_id,amount_cents) VALUES(1,'checkin','old',25)`)
				if lineage == "legacy" {
					exec(`INSERT INTO activity_fund_ledger(user_id,source,request_id,api_key_id,eligible_cost) VALUES(1,'usage','old-call',1,12)`)
				}
			}
			if lineage == "isolated" {
				for _, name := range []string{"258_activity_rewards.sql", "259_activity_lottery.sql", "260_activity_independent_budgets.sql", "264_activity_production_safety.sql", "265_activity_lottery_nine_grid.sql", "266_activity_consumption_master_switch.sql"} {
					apply(name)
				}
				exec(`INSERT INTO users(id,balance) VALUES(1,100);INSERT INTO activity_wallets(user_id,available_cents,total_earned_cents,unabsorbed_gift) VALUES(1,25,25,2)`)
			}
			require.NoError(t, applyMigrationsFS(ctx, db, all))
			require.NoError(t, applyMigrationsFS(ctx, db, all), "restarting must preserve checksums and not replay conversion")
			var coreTriggers int
			require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM pg_trigger tg JOIN pg_class c ON c.oid=tg.tgrelid JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname=current_schema() AND NOT tg.tgisinternal AND c.relname IN ('users','usage_logs','payment_orders','redeem_codes')`).Scan(&coreTriggers))
			require.Zero(t, coreTriggers)
			var coreFKs int
			require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM pg_constraint c JOIN pg_class own ON own.oid=c.conrelid JOIN pg_namespace n ON n.oid=own.relnamespace WHERE c.contype='f' AND n.nspname=current_schema() AND c.confrelid IN ('users'::regclass,'usage_logs'::regclass,'payment_orders'::regclass,'redeem_codes'::regclass)`).Scan(&coreFKs))
			require.Zero(t, coreFKs)
			if strings.HasPrefix(lineage, "legacy") {
				var available, earned, withdrawn int64
				require.NoError(t, db.QueryRow(`SELECT available_cents,total_earned_cents,total_withdrawn_cents FROM activity_wallets WHERE user_id=1`).Scan(&available, &earned, &withdrawn))
				require.Zero(t, available)
				require.EqualValues(t, 25, earned)
				require.Equal(t, earned, withdrawn)
				var balance string
				require.NoError(t, db.QueryRow(`SELECT balance::text FROM users WHERE id=1`).Scan(&balance))
				require.Equal(t, "100.00000000", balance)
				if lineage == "legacy" {
					var spend string
					require.NoError(t, db.QueryRow(`SELECT SUM(eligible_cost)::text FROM activity_usage_events`).Scan(&spend))
					require.Equal(t, "12.00000000", spend)
				}
			}
			if lineage == "isolated" {
				var available int64
				require.NoError(t, db.QueryRow(`SELECT available_cents FROM activity_wallets WHERE user_id=1`).Scan(&available))
				require.EqualValues(t, 25, available)
			}
			if lineage == "isolated" {
				t.Run("atomic_billing", func(t *testing.T) { testActivityAtomicBilling(t, db) })
			}

		})
	}
}

func testActivityAtomicBilling(t *testing.T, db *sql.DB) {
	ctx := context.Background()
	exec := func(q string, args ...any) { t.Helper(); _, err := db.Exec(q, args...); require.NoError(t, err) }
	scalar := func(q string, args ...any) string {
		t.Helper()
		var v string
		require.NoError(t, db.QueryRow(q, args...).Scan(&v))
		return v
	}
	exec(`CREATE TABLE usage_billing_dedup(id BIGSERIAL PRIMARY KEY,request_id TEXT,api_key_id BIGINT,request_fingerprint TEXT,UNIQUE(request_id,api_key_id));CREATE TABLE usage_billing_dedup_archive(request_id TEXT,api_key_id BIGINT,request_fingerprint TEXT,UNIQUE(request_id,api_key_id));UPDATE activity_audiences SET mode='all';UPDATE activity_config SET user_lifetime_cap_cents=10000;
 INSERT INTO activity_consumption_campaigns(id,name,starts_at,ends_at,claim_until,rollout_percent,budget_cents,state,published_at) VALUES(1,'atomic',NOW()-interval '1 hour',NOW()+interval '1 day',NOW()+interval '3 days',100,10000,'active',NOW()-interval '1 hour');
 INSERT INTO activity_consumption_tiers(campaign_id,position,threshold_cents,reward_cents) VALUES(1,1,100,10)`)
	repo := NewUsageBillingRepository(nil, db)
	bill := func(id string, cost float64) *service.UsageBillingCommand {
		return &service.UsageBillingCommand{RequestID: id, APIKeyID: 1, UserID: 1, BalanceCost: cost}
	}
	cmd := bill("atomic-before-transfer", 10)
	first, err := repo.Apply(ctx, cmd)
	require.NoError(t, err)
	require.True(t, first.Applied)
	require.Equal(t, "8.00000000", scalar(`SELECT eligible_cost::text FROM activity_usage_events WHERE request_id=$1`, cmd.RequestID))
	exec(`UPDATE users SET balance=balance+10 WHERE id=1;UPDATE activity_wallets SET unabsorbed_gift=unabsorbed_gift+10 WHERE user_id=1`)
	duplicate, err := repo.Apply(ctx, cmd)
	require.NoError(t, err)
	require.False(t, duplicate.Applied)
	require.Equal(t, "10.00000000", scalar(`SELECT unabsorbed_gift::text FROM activity_wallets WHERE user_id=1`))
	_, err = repo.Apply(ctx, bill("atomic-after-transfer", 10))
	require.NoError(t, err)
	require.Equal(t, "0.00000000", scalar(`SELECT eligible_cost::text FROM activity_usage_events WHERE request_id='atomic-after-transfer'`))
	exec(`INSERT INTO usage_logs(user_id,api_key_id,request_id,actual_cost) VALUES(1,1,'atomic-before-transfer',10),(1,1,'unbilled-simple-mode',100)`)
	svc := service.NewActivityService(db, nil, nil)
	_, err = svc.IngestUsageEvents(ctx)
	require.NoError(t, err)
	require.Equal(t, "2", scalar(`SELECT COUNT(*)::text FROM activity_usage_events WHERE billing_recorded`))
	require.Equal(t, "0", scalar(`SELECT COUNT(*)::text FROM activity_usage_events WHERE request_id='unbilled-simple-mode'`))
	for i := 0; i < 5; i++ {
		_, err = svc.ProcessConsumptionEvent(ctx)
		require.NoError(t, err)
	}
	require.Equal(t, "8.00000000", scalar(`SELECT activity_consumption_spend(1,1)::text`))
	// Merged bills retain command order and atomically burn the same gift pool.
	exec(`UPDATE activity_wallets SET unabsorbed_gift=3 WHERE user_id=1`)
	cmds := []*service.UsageBillingCommand{bill("batch-1", 2), bill("batch-2", 4)}
	outcomes, err := repo.ApplyBatch(ctx, cmds)
	require.NoError(t, err)
	require.True(t, outcomes[0].Result.Applied)
	require.True(t, outcomes[1].Result.Applied)
	require.Equal(t, "0.00000000", scalar(`SELECT eligible_cost::text FROM activity_usage_events WHERE request_id='batch-1'`))
	require.Equal(t, "3.00000000", scalar(`SELECT eligible_cost::text FROM activity_usage_events WHERE request_id='batch-2'`))
	_, err = repo.ApplyBatch(ctx, cmds)
	require.NoError(t, err)
	require.Equal(t, "2", scalar(`SELECT COUNT(*)::text FROM activity_usage_events WHERE request_id IN ('batch-1','batch-2')`))
	// A reserved image job retains its original gift split across later transfers.
	exec(`UPDATE activity_wallets SET unabsorbed_gift=5 WHERE user_id=1`)
	hold := &service.BatchImageBalanceHoldCommand{RequestID: service.BatchImageHoldRequestID("image-1"), APIKeyID: 1, UserID: 1, BatchID: "image-1", HoldAmount: 10}
	_, err = repo.ReserveBatchImageBalance(ctx, hold)
	require.NoError(t, err)
	require.Equal(t, "0.00000000", scalar(`SELECT unabsorbed_gift::text FROM activity_wallets WHERE user_id=1`))
	exec(`UPDATE users SET balance=balance+10 WHERE id=1;UPDATE activity_wallets SET unabsorbed_gift=10 WHERE user_id=1`)
	capture := *hold
	capture.RequestID = service.BatchImageCaptureRequestID("image-1")
	capture.RequestFingerprint = ""
	capture.ActualAmount = 8
	_, err = repo.CaptureBatchImageBalance(ctx, &capture)
	require.NoError(t, err)
	require.Equal(t, "3.00000000", scalar(`SELECT eligible_cost::text FROM activity_usage_events WHERE request_id=$1`, capture.RequestID))
	require.Equal(t, "10.00000000", scalar(`SELECT unabsorbed_gift::text FROM activity_wallets WHERE user_id=1`))
	_, err = repo.CaptureBatchImageBalance(ctx, &capture)
	require.NoError(t, err)
	// Sequence allocation can precede another transaction's commit. The worker
	// must process the earlier event even after a later one has already completed.
	exec(`INSERT INTO users(id,balance) VALUES(2,100),(3,100)`)
	early, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer early.Rollback()
	_, err = early.Exec(`UPDATE users SET balance=balance-10 WHERE id=2`)
	require.NoError(t, err)
	require.NoError(t, activitystore.RecordUsage(ctx, early, 2, 2, "late-atomic-commit", 10, true))
	later := &service.UsageBillingCommand{RequestID: "early-atomic-commit", APIKeyID: 3, UserID: 3, BalanceCost: 10}
	_, err = repo.Apply(ctx, later)
	require.NoError(t, err)
	for i := 0; i < 10; i++ {
		_, err = svc.ProcessConsumptionEvent(ctx)
		require.NoError(t, err)
	}
	require.Equal(t, "1", scalar(`SELECT COUNT(*)::text FROM activity_consumption_entitlements WHERE user_id=3`))
	require.NoError(t, early.Commit())
	for i := 0; i < 10; i++ {
		_, err = svc.ProcessConsumptionEvent(ctx)
		require.NoError(t, err)
	}
	require.Equal(t, "1", scalar(`SELECT COUNT(*)::text FROM activity_consumption_entitlements WHERE user_id=2`))
	// Disabled consumption periods still absorb gifts, but cannot add progress.
	_, err = svc.SetConsumptionEnabled(ctx, 1, false)
	require.NoError(t, err)
	exec(`UPDATE activity_wallets SET unabsorbed_gift=1 WHERE user_id=1`)
	_, err = repo.Apply(ctx, bill("atomic-during-pause", 5))
	require.NoError(t, err)
	_, err = svc.SetConsumptionEnabled(ctx, 1, true)
	require.NoError(t, err)
	for i := 0; i < 10; i++ {
		_, err = svc.ProcessConsumptionEvent(ctx)
		require.NoError(t, err)
	}
	require.Equal(t, "0.00000000", scalar(`SELECT unabsorbed_gift::text FROM activity_wallets WHERE user_id=1`))
	require.Equal(t, "14.00000000", scalar(`SELECT activity_consumption_spend(1,1)::text`))

	// Releasing a job returns its reserved gifts exactly once.
	exec(`UPDATE activity_wallets SET unabsorbed_gift=4 WHERE user_id=1`)
	held := &service.BatchImageBalanceHoldCommand{RequestID: service.BatchImageHoldRequestID("release-1"), APIKeyID: 1, UserID: 1, BatchID: "release-1", HoldAmount: 8}
	_, err = repo.ReserveBatchImageBalance(ctx, held)
	require.NoError(t, err)
	require.Equal(t, "0.00000000", scalar(`SELECT unabsorbed_gift::text FROM activity_wallets WHERE user_id=1`))
	release := *held
	release.RequestID = service.BatchImageReleaseRequestID("release-1")
	release.RequestFingerprint = ""
	_, err = repo.ReleaseBatchImageBalance(ctx, &release)
	require.NoError(t, err)
	_, err = repo.ReleaseBatchImageBalance(ctx, &release)
	require.NoError(t, err)
	require.Equal(t, "4.00000000", scalar(`SELECT unabsorbed_gift::text FROM activity_wallets WHERE user_id=1`))
	// Billing identifiers can be longer than the old activity log's 64 characters.
	_, err = repo.Apply(ctx, bill(strings.Repeat("long", 40), 1))
	require.NoError(t, err)
	// Blocked requests still consume the gift they spend, but earn no reward progress.
	blocked := bill("blocked-request", 2)
	blocked.RequestType = service.RequestTypeCyberBlocked
	_, err = repo.Apply(ctx, blocked)
	require.NoError(t, err)
	require.Equal(t, "0.00000000", scalar(`SELECT eligible_cost::text FROM activity_usage_events WHERE request_id='blocked-request'`))

	// If persisting the durable event fails, neither the debit nor dedup claim commits.
	before := scalar(`SELECT balance::text FROM users WHERE id=1`)
	exec(`ALTER TABLE activity_usage_events ADD CONSTRAINT reject_test_event CHECK(request_id<>'atomic-rollback')`)
	_, err = repo.Apply(ctx, bill("atomic-rollback", 1))
	require.Error(t, err)
	require.Equal(t, before, scalar(`SELECT balance::text FROM users WHERE id=1`))
	require.Equal(t, "0", scalar(`SELECT COUNT(*)::text FROM usage_billing_dedup WHERE request_id='atomic-rollback'`))
}

func TestActivityMaintenanceGate(t *testing.T) {
	for _, tc := range []struct {
		name, confirmed     string
		hasUsers, wantError bool
	}{
		{"existing installation needs drained acknowledgement", "", true, true},
		{"empty installation can migrate", "", false, false},
		{"operator confirmed drained installation", "1", true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("ACTIVITY_MAINTENANCE_CONFIRMED", tc.confirmed)
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()
			conn, err := db.Conn(context.Background())
			require.NoError(t, err)
			defer conn.Close()
			if tc.confirmed != "1" {
				mock.ExpectQuery("SELECT EXISTS").WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(tc.hasUsers))
			}
			err = requireActivityMaintenance(context.Background(), conn)
			if tc.wantError {
				require.ErrorContains(t, err, "drained maintenance window")
			} else {
				require.NoError(t, err)
			}
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
