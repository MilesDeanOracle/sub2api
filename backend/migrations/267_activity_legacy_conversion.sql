-- Transfer historical accounting, never historical rewards: those rewards were
-- already credited directly to users.balance by the retired application.
DO $$
BEGIN
 IF to_regclass('activity_legacy_transition') IS NULL THEN RETURN; END IF;
 ALTER TABLE activity_usage_events ALTER COLUMN request_id TYPE VARCHAR(255);
 INSERT INTO activity_wallets(user_id,total_earned_cents,total_withdrawn_cents)
 SELECT user_id,SUM(amount_cents),SUM(amount_cents) FROM activity_reward_ledger GROUP BY user_id
 ON CONFLICT(user_id) DO NOTHING;
 IF to_regclass('activity_funds') IS NOT NULL THEN
  INSERT INTO activity_wallets(user_id,unabsorbed_gift)
  SELECT user_id,gift+unverified FROM activity_funds
  ON CONFLICT(user_id) DO UPDATE SET unabsorbed_gift=EXCLUDED.unabsorbed_gift;
 ELSE
  -- Very early installations have no verified source account. Preserve their
  -- balance but do not turn unknown historical funding into reward progress.
  INSERT INTO activity_wallets(user_id,unabsorbed_gift)
  SELECT id,GREATEST(balance,0) FROM users
  ON CONFLICT(user_id) DO UPDATE SET unabsorbed_gift=EXCLUDED.unabsorbed_gift;
 END IF;
 IF to_regclass('activity_fund_ledger') IS NOT NULL THEN
  INSERT INTO activity_usage_events(usage_log_id,user_id,request_id,api_key_id,actual_cost,eligible_cost,occurred_at)
  SELECT -id,user_id,COALESCE(request_id,''),COALESCE(api_key_id,0),eligible_cost,eligible_cost,created_at
  FROM activity_fund_ledger WHERE eligible_cost>0 ON CONFLICT DO NOTHING;
  INSERT INTO activity_consumption_outbox(event_id,user_id,created_at)
  SELECT usage_log_id,user_id,occurred_at FROM activity_usage_events WHERE usage_log_id<0 ON CONFLICT DO NOTHING;
 END IF;
END $$;
