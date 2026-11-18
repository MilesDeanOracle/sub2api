package service

import (
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestConsumptionCampaignValidation(t *testing.T) {
	now := time.Now()
	valid := ConsumptionCampaign{Name: "Consumption", StartsAt: now, EndsAt: now.Add(30 * 24 * time.Hour), ClaimUntil: now.Add(37 * 24 * time.Hour), RolloutPercent: 10, BudgetCents: 400, Tiers: []ConsumptionTier{{Position: 1, ThresholdCents: 1000, RewardCents: 30}, {Position: 2, ThresholdCents: 5000, RewardCents: 120}, {Position: 3, ThresholdCents: 10000, RewardCents: 250}}}
	require.NoError(t, validateConsumptionCampaign(valid))
	for _, tc := range []struct {
		name   string
		change func(*ConsumptionCampaign)
	}{
		{"unordered thresholds", func(c *ConsumptionCampaign) { c.Tiers[1].ThresholdCents = 1000 }},
		{"zero reward", func(c *ConsumptionCampaign) { c.Tiers[2].RewardCents = 0 }},
		{"partial budget", func(c *ConsumptionCampaign) { c.BudgetCents = 399 }},
		{"insufficient claim period", func(c *ConsumptionCampaign) { c.ClaimUntil = c.EndsAt }},
		{"invalid coverage", func(c *ConsumptionCampaign) { c.RolloutPercent = 101 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := valid
			c.Tiers = append([]ConsumptionTier(nil), valid.Tiers...)
			tc.change(&c)
			require.Error(t, validateConsumptionCampaign(c))
		})
	}
}
