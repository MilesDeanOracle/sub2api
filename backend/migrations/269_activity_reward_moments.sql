-- Independent, opt-in read projection without core billing foreign keys. Never backfill historical rewards.
CREATE TABLE activity_reward_moments_config (
 id INT PRIMARY KEY CHECK(id=1),
 enabled BOOLEAN NOT NULL DEFAULT FALSE,
 user_ids BIGINT[] NOT NULL DEFAULT '{}',
 alias_secret TEXT NOT NULL DEFAULT (gen_random_uuid()::text || gen_random_uuid()::text),
 updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
INSERT INTO activity_reward_moments_config(id) VALUES(1);

CREATE TABLE activity_reward_sharing (
 user_id BIGINT PRIMARY KEY,
 allowed BOOLEAN NOT NULL DEFAULT FALSE,
 granted_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
 updated_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);
CREATE TABLE activity_reward_moments (
 ledger_id BIGINT PRIMARY KEY REFERENCES activity_wallet_ledger(id) ON DELETE CASCADE,
 user_id BIGINT NOT NULL,
 kind TEXT NOT NULL CHECK(kind IN ('checkin','lottery','consumption')),
 source_id TEXT NOT NULL,
 campaign_id BIGINT REFERENCES activity_consumption_campaigns(id),
 amount_cents BIGINT NOT NULL CHECK(amount_cents>0),
 created_at TIMESTAMPTZ NOT NULL,
 UNIQUE(kind,source_id)
);
CREATE INDEX idx_activity_moments_recent ON activity_reward_moments(created_at DESC,ledger_id DESC);
CREATE INDEX idx_activity_moments_user ON activity_reward_moments(user_id);

CREATE FUNCTION activity_project_reward_moment() RETURNS TRIGGER LANGUAGE plpgsql AS $$
DECLARE cid BIGINT;
BEGIN
 IF NEW.direction<>'credit' OR NEW.kind NOT IN ('checkin','lottery','consumption') THEN RETURN NEW; END IF;
 -- Capture only rewards earned after consent, from a verifiable activity source.
 IF NOT EXISTS(SELECT 1 FROM activity_reward_sharing WHERE user_id=NEW.user_id AND allowed AND granted_at<=NEW.created_at) THEN RETURN NEW; END IF;
 IF NEW.kind='checkin' AND NOT EXISTS(
  SELECT 1 FROM activity_checkins c WHERE c.user_id=NEW.user_id AND c.user_id::text||':'||c.local_day::text=NEW.source_id AND c.reward_cents=NEW.amount_cents
 ) THEN RETURN NEW; END IF;
 IF NEW.kind='lottery' AND NOT EXISTS(
  SELECT 1 FROM activity_lottery_draws d WHERE d.user_id=NEW.user_id AND d.user_id::text||':'||d.batch_key=NEW.source_id
  GROUP BY d.batch_key HAVING SUM(d.reward_cents)=NEW.amount_cents AND bool_and(d.revoked_at IS NULL)
 ) THEN RETURN NEW; END IF;
 IF NEW.kind='consumption' THEN
  SELECT e.campaign_id INTO cid FROM activity_consumption_entitlements e
   WHERE e.user_id=NEW.user_id AND e.campaign_id::text||':'||e.user_id::text||':'||e.position::text=NEW.source_id
   AND e.state='claimed' AND e.claimed_at IS NOT NULL AND e.reward_cents=NEW.amount_cents;
  IF cid IS NULL THEN RETURN NEW; END IF;
 END IF;
 INSERT INTO activity_reward_moments(ledger_id,user_id,kind,source_id,campaign_id,amount_cents,created_at)
 VALUES(NEW.id,NEW.user_id,NEW.kind,NEW.source_id,cid,NEW.amount_cents,NEW.created_at) ON CONFLICT DO NOTHING;
 RETURN NEW;
END $$;
CREATE TRIGGER activity_project_reward_moment_trigger AFTER INSERT ON activity_wallet_ledger FOR EACH ROW EXECUTE FUNCTION activity_project_reward_moment();

CREATE FUNCTION activity_remove_revoked_moment() RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
 IF TG_TABLE_NAME='activity_lottery_draws' THEN
  IF NEW.revoked_at IS NOT NULL THEN DELETE FROM activity_reward_moments WHERE kind='lottery' AND source_id=NEW.user_id::text||':'||NEW.batch_key; END IF;
 ELSE
  IF NEW.state IN ('revoked','review') THEN DELETE FROM activity_reward_moments WHERE kind='consumption' AND source_id=NEW.campaign_id::text||':'||NEW.user_id::text||':'||NEW.position::text; END IF;
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER activity_remove_lottery_moment_trigger AFTER UPDATE OF revoked_at ON activity_lottery_draws FOR EACH ROW EXECUTE FUNCTION activity_remove_revoked_moment();
CREATE TRIGGER activity_remove_consumption_moment_trigger AFTER UPDATE OF state ON activity_consumption_entitlements FOR EACH ROW EXECUTE FUNCTION activity_remove_revoked_moment();
