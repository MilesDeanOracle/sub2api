package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestActivityWalletExperiencePostgres(t *testing.T) {
	f := newActivityProductionFixture(t)
	f.exec(`UPDATE activity_runtime SET awards_enabled=false`)
	checkin, err := f.service.Status(f.ctx, 1)
	require.NoError(t, err)
	require.True(t, checkin.WithdrawBlocked)
	require.Equal(t, "awards_paused", checkin.WithdrawBlockReason)
	lottery, err := f.service.LotteryStatus(f.ctx, 1)
	require.NoError(t, err)
	require.True(t, lottery.WithdrawBlocked)
	require.Equal(t, "awards_paused", lottery.WithdrawBlockReason)

	f.exec(`UPDATE activity_runtime SET awards_enabled=true;
		INSERT INTO activity_consumption_campaigns(id,name,starts_at,ends_at,claim_until,rollout_percent,budget_cents,state,published_at) VALUES
		(1,'current',NOW()-interval '1 hour',NOW()+interval '1 day',NOW()+interval '3 days',100,10000,'active',NOW()-interval '1 hour'),
		(2,'ended with reward',NOW()-interval '2 days',NOW()-interval '1 hour',NOW()+interval '2 days',100,10000,'closed',NOW()-interval '2 days');
		INSERT INTO activity_consumption_tiers(campaign_id,position,threshold_cents,reward_cents) VALUES(1,1,1000,30),(2,1,1000,30);
		INSERT INTO activity_consumption_entitlements(campaign_id,user_id,position,reward_cents) VALUES(2,1,1,30);`)
	// The wallet's claim shortcut should lead to the reward, even if a newer
	// active campaign has no claimable reward yet. Explicit selection still wins.
	status, err := f.service.ConsumptionStatus(f.ctx, 1)
	require.NoError(t, err)
	require.EqualValues(t, 2, status.Campaign.ID)
	require.EqualValues(t, 30, status.ClaimableCents)
	selected, err := f.service.ConsumptionStatus(f.ctx, 1, 1)
	require.NoError(t, err)
	require.EqualValues(t, 1, selected.Campaign.ID)
	require.NoError(t, f.service.ClaimConsumption(f.ctx, 1, 2, 1))
	status, err = f.service.ConsumptionStatus(f.ctx, 1)
	require.NoError(t, err)
	require.EqualValues(t, 1, status.Campaign.ID)
}
