-- Default fail-closed for production testing. Visibility is independent of
-- financial gates and campaign rollout percentages. No funds are rewritten.
CREATE TABLE activity_audiences (
 kind TEXT PRIMARY KEY CHECK(kind IN ('checkin','consumption')),
 mode TEXT NOT NULL DEFAULT 'selected' CHECK(mode IN ('selected','all')),
 user_ids BIGINT[] NOT NULL DEFAULT '{}',
 updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
INSERT INTO activity_audiences(kind) VALUES('checkin'),('consumption');
CREATE FUNCTION activity_audience_allowed(activity_kind TEXT,uid BIGINT) RETURNS BOOLEAN LANGUAGE SQL STABLE AS $$
 SELECT COALESCE((SELECT mode='all' OR uid=ANY(user_ids) FROM activity_audiences WHERE kind=activity_kind),FALSE)
$$;
CREATE OR REPLACE FUNCTION activity_process_consumption_event(event_id BIGINT) RETURNS BOOLEAN LANGUAGE plpgsql AS $$
DECLARE event activity_fund_ledger%ROWTYPE; c RECORD; t RECORD; shared_limit BIGINT; user_limit BIGINT;
 spent BIGINT; user_spent BIGINT; used BIGINT; progress NUMERIC; retry BOOLEAN:=FALSE;
BEGIN
 SELECT * INTO STRICT event FROM activity_fund_ledger WHERE id=event_id;
 PERFORM 1 FROM users WHERE id=event.user_id FOR UPDATE;
 PERFORM 1 FROM activity_audiences WHERE kind='consumption' FOR SHARE;
 -- Do not resurrect revoked or reviewed rewards through queue retries.
 IF NOT EXISTS(SELECT 1 FROM users WHERE id=event.user_id AND status='active' AND deleted_at IS NULL) THEN RETURN TRUE; END IF;
 FOR c IN SELECT * FROM activity_consumption_campaigns
 WHERE published_at IS NOT NULL AND state IN ('active','paused','closed')
 AND published_at<=event.created_at AND starts_at<=event.created_at
 AND event.created_at<LEAST(ends_at,COALESCE(stopped_at,ends_at))
 AND claim_until>NOW() AND activity_audience_allowed('consumption',event.user_id) AND event.user_id%100<rollout_percent ORDER BY id LOOP
 IF EXISTS(SELECT 1 FROM activity_consumption_entitlements WHERE user_id=event.user_id AND state='review') OR
 EXISTS(SELECT 1 FROM activity_refund_reviews WHERE user_id=event.user_id AND review_status='pending') THEN RETURN FALSE; END IF;
  progress:=activity_consumption_spend(c.id,event.user_id);
  -- The worker may serialize award reservations; API debits do not take this lock.
  SELECT total_budget_cents,user_lifetime_cap_cents INTO shared_limit,user_limit FROM activity_config WHERE id=1 FOR UPDATE;
  FOR t IN SELECT * FROM activity_consumption_tiers WHERE campaign_id=c.id ORDER BY position LOOP
   IF progress*100<t.threshold_cents OR EXISTS(SELECT 1 FROM activity_consumption_entitlements WHERE campaign_id=c.id AND user_id=event.user_id AND position=t.position) THEN CONTINUE; END IF;
   SELECT COALESCE(SUM(amount_cents),0) INTO spent FROM activity_reward_ledger;
   SELECT COALESCE(SUM(amount_cents),0) INTO user_spent FROM activity_reward_ledger WHERE user_id=event.user_id;
   SELECT COALESCE(SUM(reward_cents),0) INTO used FROM activity_consumption_entitlements
    WHERE campaign_id=c.id AND (claimed_at IS NOT NULL OR (state IN ('reserved','review') AND (state='review' OR c.claim_until>NOW())));
   IF spent+activity_reserved_cents()+t.reward_cents>shared_limit OR user_spent+activity_reserved_cents(event.user_id)+t.reward_cents>user_limit OR used+t.reward_cents>c.budget_cents THEN retry:=TRUE; CONTINUE; END IF;
   INSERT INTO activity_consumption_entitlements(campaign_id,user_id,position,reward_cents)
    VALUES(c.id,event.user_id,t.position,t.reward_cents) ON CONFLICT DO NOTHING;
  END LOOP;
 END LOOP;
 RETURN NOT retry;
END $$;
