package service

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func newRewardMomentsFixture(t *testing.T) *activityProductionFixture {
	f := newActivityProductionFixture(t)
	raw, err := os.ReadFile(filepath.Join("..", "..", "migrations", "269_activity_reward_moments.sql"))
	require.NoError(t, err)
	f.exec(string(raw))
	require.Equal(t, "false:0", f.scalar(`SELECT enabled::text||':'||cardinality(user_ids)::text FROM activity_reward_moments_config`))
	f.exec(`INSERT INTO users(id,balance) VALUES(2,100),(3,100); UPDATE activity_reward_moments_config SET enabled=true,user_ids=ARRAY[2]::bigint[]`)
	return f
}

func TestRewardMomentsPrivacyPostgres(t *testing.T) {
	f := newRewardMomentsFixture(t)
	sharing, err := f.service.RewardSharing(f.ctx, 1)
	require.NoError(t, err)
	require.False(t, sharing.Allowed)
	// Unconsented awards are never projected or later backfilled.
	_, err = f.service.Checkin(f.ctx, 1)
	require.NoError(t, err)
	require.Equal(t, "0", f.scalar(`SELECT COUNT(*)::text FROM activity_reward_moments`))
	_, err = f.service.SetRewardSharing(f.ctx, 1, true)
	require.NoError(t, err)
	feed, err := f.service.RewardMoments(f.ctx, 2)
	require.NoError(t, err)
	require.Empty(t, feed.Items)
	result, err := f.service.DrawLottery(f.ctx, 1, 1, "public-real-draw")
	require.NoError(t, err)
	feed, err = f.service.RewardMoments(f.ctx, 2)
	require.NoError(t, err)
	require.Len(t, feed.Items, 1)
	require.EqualValues(t, result.TotalRewardCents, feed.Items[0].AmountCents)
	alias := feed.Items[0].Alias
	again, err := f.service.RewardMoments(f.ctx, 2)
	require.NoError(t, err)
	require.Equal(t, alias, again.Items[0].Alias)
	raw, err := json.Marshal(feed)
	require.NoError(t, err)
	for _, private := range []string{"user_id", "source_id", "ledger_id", "alias_secret", "balance", "email", "batch_key"} {
		require.NotContains(t, string(raw), private)
	}
	// Retry returns the original draw without adding an announcement.
	_, err = f.service.DrawLottery(f.ctx, 1, 1, "public-real-draw")
	require.NoError(t, err)
	require.Equal(t, "1", f.scalar(`SELECT COUNT(*)::text FROM activity_reward_moments`))
	hidden, err := f.service.RewardMoments(f.ctx, 3)
	require.NoError(t, err)
	require.False(t, hidden.Enabled)
	require.Empty(t, hidden.Items)
	f.exec(`UPDATE activity_audiences SET mode='selected',user_ids=ARRAY[1]::bigint[] WHERE kind='lottery'`)
	hidden, err = f.service.RewardMoments(f.ctx, 2)
	require.NoError(t, err)
	require.Empty(t, hidden.Items)
	f.exec(`UPDATE activity_audiences SET mode='all' WHERE kind='lottery'`)
	_, err = f.service.SetRewardSharing(f.ctx, 1, false)
	require.NoError(t, err)
	hidden, err = f.service.RewardMoments(f.ctx, 2)
	require.NoError(t, err)
	require.Empty(t, hidden.Items)
	_, err = f.service.SetRewardSharing(f.ctx, 1, true)
	require.NoError(t, err)
	hidden, err = f.service.RewardMoments(f.ctx, 2)
	require.NoError(t, err)
	require.Empty(t, hidden.Items)
}

func TestRewardMomentsRevocationPostgres(t *testing.T) {
	f := newRewardMomentsFixture(t)
	_, err := f.service.SetRewardSharing(f.ctx, 1, true)
	require.NoError(t, err)
	_, err = f.service.Checkin(f.ctx, 1)
	require.NoError(t, err)
	_, err = f.service.DrawLottery(f.ctx, 1, 1, "revoked-public-draw")
	require.NoError(t, err)
	require.Equal(t, "2", f.scalar(`SELECT COUNT(*)::text FROM activity_reward_moments`))
	f.exec(`UPDATE activity_lottery_draws SET revoked_at=NOW() WHERE batch_key='revoked-public-draw'`)
	require.Equal(t, "1", f.scalar(`SELECT COUNT(*)::text FROM activity_reward_moments`))
	// Transfer and unverified/manual wallet entries never become new rewards.
	_, err = f.service.WithdrawWallet(f.ctx, 1)
	require.NoError(t, err)
	f.exec(`INSERT INTO activity_wallet_ledger(user_id,direction,kind,source_id,amount_cents,wallet_after_cents) VALUES(1,'credit','checkin','manual-test',99,99)`)
	require.Equal(t, "1", f.scalar(`SELECT COUNT(*)::text FROM activity_reward_moments`))
	f.exec(`UPDATE activity_reward_moments SET created_at=NOW()-interval '8 days'`)
	feed, err := f.service.RewardMoments(f.ctx, 2)
	require.NoError(t, err)
	require.Empty(t, feed.Items)
}

func TestRewardMomentsConsumptionPostgres(t *testing.T) {
	f := newRewardMomentsFixture(t)
	_, err := f.service.SetRewardSharing(f.ctx, 1, true)
	require.NoError(t, err)
	f.exec(`INSERT INTO activity_consumption_campaigns(id,name,starts_at,ends_at,claim_until,rollout_percent,budget_cents,state,published_at) VALUES(1,'Visible campaign',NOW()-interval '1 day',NOW()+interval '1 day',NOW()+interval '3 days',100,10000,'active',NOW()-interval '1 day');
 INSERT INTO activity_consumption_tiers(campaign_id,position,threshold_cents,reward_cents) VALUES(1,1,100,50);
 INSERT INTO activity_consumption_entitlements(campaign_id,user_id,position,reward_cents) VALUES(1,1,1,50)`)
	feed, err := f.service.RewardMoments(f.ctx, 2)
	require.NoError(t, err)
	require.Empty(t, feed.Items)
	require.NoError(t, f.service.ClaimConsumption(f.ctx, 1, 1, 0))
	feed, err = f.service.RewardMoments(f.ctx, 2)
	require.NoError(t, err)
	require.Len(t, feed.Items, 1)
	require.Equal(t, "Visible campaign", feed.Items[0].CampaignName)
	f.exec(`UPDATE activity_audiences SET mode='selected',user_ids=ARRAY[1]::bigint[] WHERE kind='consumption'`)
	feed, err = f.service.RewardMoments(f.ctx, 2)
	require.NoError(t, err)
	require.Empty(t, feed.Items)
	f.exec(`UPDATE activity_audiences SET mode='all' WHERE kind='consumption'; UPDATE activity_consumption_entitlements SET state='revoked' WHERE user_id=1`)
	require.Equal(t, "0", f.scalar(`SELECT COUNT(*)::text FROM activity_reward_moments`))
}

func TestRewardMomentsConfigPostgres(t *testing.T) {
	f := newRewardMomentsFixture(t)
	require.NoError(t, f.service.UpdateRewardMomentsConfig(f.ctx, 2, RewardMomentsConfig{Enabled: false, UserIDs: []int64{2}}))
	_, err := f.service.SetRewardSharing(f.ctx, 1, true)
	require.NoError(t, err)
	_, err = f.service.Checkin(f.ctx, 1)
	require.NoError(t, err)
	feed, err := f.service.RewardMoments(f.ctx, 2)
	require.NoError(t, err)
	require.False(t, feed.Enabled)
	require.Empty(t, feed.Items)
	require.Error(t, f.service.UpdateRewardMomentsConfig(f.ctx, 2, RewardMomentsConfig{UserIDs: []int64{9999}}))
	require.NoError(t, f.service.UpdateRewardMomentsConfig(f.ctx, 2, RewardMomentsConfig{Enabled: true, UserIDs: []int64{2, 2}}))
	config, err := f.service.RewardMomentsConfig(f.ctx)
	require.NoError(t, err)
	require.Equal(t, []int64{2}, config.UserIDs)
	require.Equal(t, "2", f.scalar(`SELECT COUNT(*)::text FROM activity_consumption_audit WHERE action='reward_moments_config'`))
	// The independent display switch never disables personal funds/actions.
	_, err = f.service.WithdrawWallet(f.ctx, 1)
	require.NoError(t, err)
}

func TestRewardMomentsEarliestClaimPostgres(t *testing.T) {
	f := newRewardMomentsFixture(t)
	f.exec(`INSERT INTO activity_consumption_campaigns(id,name,starts_at,ends_at,claim_until,rollout_percent,budget_cents,state,published_at) VALUES
 (1,'Expires first',NOW()-interval '1 day',NOW()+interval '1 day',NOW()+interval '3 days',100,10000,'active',NOW()-interval '1 day'),
 (2,'Expires later',NOW()-interval '1 day',NOW()+interval '1 day',NOW()+interval '4 days',100,10000,'active',NOW()-interval '1 day');
 INSERT INTO activity_consumption_tiers(campaign_id,position,threshold_cents,reward_cents) VALUES(1,1,100,50),(2,1,100,100);
 INSERT INTO activity_consumption_entitlements(campaign_id,user_id,position,reward_cents) VALUES(1,1,1,50),(2,1,1,100)`)
	status, err := f.service.ConsumptionStatus(f.ctx, 1)
	require.NoError(t, err)
	require.EqualValues(t, 1, status.Campaign.ID)
	require.EqualValues(t, 50, status.ClaimableCents)
	require.NoError(t, f.service.ClaimConsumption(f.ctx, 1, status.Campaign.ID, 0))
	status, err = f.service.ConsumptionStatus(f.ctx, 1)
	require.NoError(t, err)
	require.EqualValues(t, 2, status.Campaign.ID)
	require.EqualValues(t, 100, status.ClaimableCents)
}
