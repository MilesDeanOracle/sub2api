-- Expand the perimeter to eight independently configurable prizes.
-- Preserve existing positions, amounts, weights, tickets and draw history.
-- Zero-weight slots are disclosed as inactive and never selected.
ALTER TABLE activity_lottery_config ALTER COLUMN tiers SET DEFAULT
    '[{"position":1,"reward_cents":50,"weight":8738},{"position":2,"reward_cents":100,"weight":961},{"position":3,"reward_cents":200,"weight":150},{"position":4,"reward_cents":300,"weight":100},{"position":5,"reward_cents":500,"weight":40},{"position":6,"reward_cents":1000,"weight":9},{"position":7,"reward_cents":2000,"weight":1},{"position":8,"reward_cents":5000,"weight":1}]'::jsonb;

-- A pristine installation can start with all eight active prizes (EV 62.9 cents).
-- Existing/custom pools require an explicit operator switch after settling tickets.
UPDATE activity_lottery_config SET tiers = '[{"position":1,"reward_cents":50,"weight":8738},{"position":2,"reward_cents":100,"weight":961},{"position":3,"reward_cents":200,"weight":150},{"position":4,"reward_cents":300,"weight":100},{"position":5,"reward_cents":500,"weight":40},{"position":6,"reward_cents":1000,"weight":9},{"position":7,"reward_cents":2000,"weight":1},{"position":8,"reward_cents":5000,"weight":1}]'::jsonb
WHERE NOT enabled AND tiers = '[{"position":1,"reward_cents":50,"weight":8800},{"position":2,"reward_cents":100,"weight":950},{"position":3,"reward_cents":300,"weight":200},{"position":4,"reward_cents":500,"weight":40},{"position":5,"reward_cents":1000,"weight":9},{"position":6,"reward_cents":5000,"weight":1}]'::jsonb
  AND NOT EXISTS (SELECT 1 FROM activity_lottery_windows)
  AND NOT EXISTS (SELECT 1 FROM activity_lottery_orders)
  AND NOT EXISTS (SELECT 1 FROM activity_lottery_ticket_ledger)
  AND NOT EXISTS (SELECT 1 FROM activity_lottery_draws);

UPDATE activity_lottery_config
SET tiers = tiers || '[{"position":7,"reward_cents":200,"weight":0},{"position":8,"reward_cents":2000,"weight":0}]'::jsonb,
    updated_at = NOW()
WHERE jsonb_array_length(tiers) = 6;

ALTER TABLE activity_lottery_draws DROP CONSTRAINT IF EXISTS activity_lottery_draws_tier_position_check;
ALTER TABLE activity_lottery_draws ADD CONSTRAINT activity_lottery_draws_tier_position_check
    CHECK (tier_position BETWEEN 1 AND 8);
