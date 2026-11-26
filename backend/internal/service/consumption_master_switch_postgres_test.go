package service

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConsumptionMasterSwitchPostgres(t *testing.T) {
	f := newActivityProductionFixture(t)
	config, err := f.service.ConsumptionConfig(f.ctx)
	require.NoError(t, err)
	require.True(t, config.Enabled)
	f.exec(`INSERT INTO activity_consumption_campaigns(id,name,starts_at,ends_at,claim_until,rollout_percent,budget_cents,state,published_at)
        VALUES(1,'switch test',NOW()-interval '1 hour',NOW()+interval '1 day',NOW()+interval '3 days',100,10000,'active',NOW()-interval '1 hour');
        INSERT INTO activity_consumption_tiers(campaign_id,position,threshold_cents,reward_cents) VALUES(1,1,1000,30),(1,2,2000,70);`)
	_, err = f.service.IngestUsageEvents(f.ctx)
	require.NoError(t, err)
	ingest := func() {
		t.Helper()
		_, err := f.service.IngestUsageEvents(f.ctx)
		require.NoError(t, err)
		for i := 0; i < 10; i++ {
			done, err := f.service.ProcessConsumptionEvent(f.ctx)
			require.NoError(t, err)
			if !done {
				break
			}
		}
	}
	f.exec(`INSERT INTO usage_logs(user_id,request_id,actual_cost) VALUES(1,'before-off',10)`)
	ingest()
	require.Equal(t, "1", f.scalar(`SELECT COUNT(*)::text FROM activity_consumption_entitlements WHERE state='reserved'`))

	config, err = f.service.SetConsumptionEnabled(f.ctx, 1, false)
	require.NoError(t, err)
	require.False(t, config.Enabled)
	_, err = f.service.SetConsumptionEnabled(f.ctx, 1, false)
	require.NoError(t, err)
	require.Equal(t, "1", f.scalar(`SELECT COUNT(*)::text FROM activity_consumption_pauses WHERE ended_at IS NULL`))
	require.Equal(t, "1", f.scalar(`SELECT COUNT(*)::text FROM activity_consumption_audit WHERE action='consumption_master_switch'`))
	require.Equal(t, "false", f.scalar(`SELECT activity_consumption_enabled_at(started_at)::text FROM activity_consumption_pauses`))
	require.Equal(t, "true", f.scalar(`SELECT activity_consumption_enabled_at(started_at-interval '1 microsecond')::text FROM activity_consumption_pauses`))

	// Re-running the migration must not reset an operator's off state or periods.
	raw, err := os.ReadFile(filepath.Join("..", "..", "migrations", "266_activity_consumption_master_switch.sql"))
	require.NoError(t, err)
	f.exec(string(raw))
	require.Equal(t, "false", f.scalar(`SELECT enabled::text FROM activity_consumption_config`))
	f.exec(`UPDATE activity_wallets SET unabsorbed_gift=5 WHERE user_id=1;
        INSERT INTO usage_logs(user_id,request_id,actual_cost) VALUES(1,'while-off-ingested',100)`)
	ingest()
	require.Equal(t, "0", f.scalar(`SELECT COUNT(*)::text FROM activity_usage_events WHERE request_id='while-off-ingested'`))
	require.Equal(t, "0.00000000", f.scalar(`SELECT unabsorbed_gift::text FROM activity_wallets WHERE user_id=1`))
	status, err := f.service.ConsumptionStatus(f.ctx, 1)
	require.NoError(t, err)
	require.False(t, status.Enabled)
	require.EqualValues(t, 30, status.ClaimableCents)
	require.NoError(t, f.service.ClaimConsumption(f.ctx, 1, 1, 1))
	require.Equal(t, "30", f.scalar(`SELECT available_cents::text FROM activity_wallets WHERE user_id=1`))

	// Delay this off-period event until after the operator has switched back on.
	f.exec(`INSERT INTO usage_logs(user_id,request_id,actual_cost) VALUES(1,'while-off-delayed',100)`)
	config, err = f.service.SetConsumptionEnabled(f.ctx, 1, true)
	require.NoError(t, err)
	require.True(t, config.Enabled)
	require.Equal(t, "true", f.scalar(`SELECT activity_consumption_enabled_at(ended_at)::text FROM activity_consumption_pauses`))
	ingest()
	require.Equal(t, "0", f.scalar(`SELECT COUNT(*)::text FROM activity_usage_events WHERE request_id='while-off-delayed'`))
	require.Equal(t, "10.00000000", f.scalar(`SELECT activity_consumption_spend(1,1)::text`))
	require.Equal(t, "active", f.scalar(`SELECT state FROM activity_consumption_campaigns WHERE id=1`))
	require.Equal(t, "true:true:true", f.scalar(`SELECT cfg.checkin_enabled::text||':'||lot.enabled::text||':'||r.processing_enabled::text FROM activity_config cfg CROSS JOIN activity_lottery_config lot CROSS JOIN activity_runtime r`))

	// Even a directly queued legacy event from the off interval cannot add progress.
	f.exec(`INSERT INTO activity_usage_events(usage_log_id,user_id,actual_cost,eligible_cost,occurred_at)
        SELECT 900,1,100,100,started_at FROM activity_consumption_pauses LIMIT 1;
        INSERT INTO activity_consumption_outbox(event_id,user_id) VALUES(900,1)`)
	ingest()
	require.Equal(t, "1", f.scalar(`SELECT COUNT(*)::text FROM activity_consumption_entitlements`))
	require.Equal(t, "10.00000000", f.scalar(`SELECT activity_consumption_spend(1,1)::text`))
	f.exec(`INSERT INTO usage_logs(user_id,request_id,actual_cost) VALUES(1,'after-on',10)`)
	ingest()
	require.Equal(t, "20.00000000", f.scalar(`SELECT activity_consumption_spend(1,1)::text`))
	require.Equal(t, "2", f.scalar(`SELECT COUNT(*)::text FROM activity_consumption_entitlements`))
	require.NoError(t, f.service.ClaimConsumption(f.ctx, 1, 1, 2))
	require.Equal(t, "100", f.scalar(`SELECT available_cents::text FROM activity_wallets WHERE user_id=1`))
}

func TestConsumptionMasterSwitchConcurrentPostgres(t *testing.T) {
	f := newActivityProductionFixture(t)
	for _, enabled := range []bool{false, true, false, true} {
		var wg sync.WaitGroup
		errs := make(chan error, 4)
		for i := 0; i < 4; i++ {
			wg.Add(1)
			go func() { defer wg.Done(); _, err := f.service.SetConsumptionEnabled(f.ctx, 1, enabled); errs <- err }()
		}
		wg.Wait()
		close(errs)
		for err := range errs {
			require.NoError(t, err)
		}
		config, err := f.service.ConsumptionConfig(f.ctx)
		require.NoError(t, err)
		require.Equal(t, enabled, config.Enabled)
	}
	require.Equal(t, "2", f.scalar(`SELECT COUNT(*)::text FROM activity_consumption_pauses`))
	require.Equal(t, "0", f.scalar(`SELECT COUNT(*)::text FROM activity_consumption_pauses WHERE ended_at IS NULL`))
	require.Equal(t, "4", f.scalar(`SELECT COUNT(*)::text FROM activity_consumption_audit WHERE action='consumption_master_switch'`))
}
