-- Forward-only hardening. Existing 257–261 migrations must remain immutable.
CREATE TABLE activity_runtime (
 id INT PRIMARY KEY CHECK(id=1),
 processing_enabled BOOLEAN NOT NULL DEFAULT FALSE,
 awards_enabled BOOLEAN NOT NULL DEFAULT FALSE,
 updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
INSERT INTO activity_runtime(id) VALUES(1);
CREATE TABLE activity_consumption_outbox (
 ledger_id BIGINT PRIMARY KEY REFERENCES activity_fund_ledger(id),
 user_id BIGINT NOT NULL,
 created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
 processed_at TIMESTAMPTZ,
 attempts INT NOT NULL DEFAULT 0,
 next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 last_error TEXT NOT NULL DEFAULT ''
);
CREATE INDEX idx_activity_outbox_pending ON activity_consumption_outbox(next_attempt_at,ledger_id) WHERE processed_at IS NULL;
CREATE INDEX idx_activity_outbox_user_pending ON activity_consumption_outbox(user_id) WHERE processed_at IS NULL;
CREATE TABLE activity_worker_status (
 id INT PRIMARY KEY CHECK(id=1), last_success_at TIMESTAMPTZ,
 last_error_at TIMESTAMPTZ, last_error TEXT NOT NULL DEFAULT ''
);
INSERT INTO activity_worker_status(id) VALUES(1);
-- Durable event insertion is local to the settlement transaction: no global
-- budget lock, eligibility scan, or reward computation on the API debit path.
CREATE FUNCTION activity_enqueue_consumption() RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
 IF NEW.eligible_cost<>0 THEN
  INSERT INTO activity_consumption_outbox(ledger_id,user_id) VALUES(NEW.id,NEW.user_id) ON CONFLICT DO NOTHING;
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER activity_enqueue_consumption_trigger AFTER INSERT ON activity_fund_ledger
 FOR EACH ROW EXECUTE FUNCTION activity_enqueue_consumption();
-- Catch up previously recorded eligible settlements on an existing installation.
INSERT INTO activity_consumption_outbox(ledger_id,user_id)
 SELECT id,user_id FROM activity_fund_ledger WHERE eligible_cost<>0 ON CONFLICT DO NOTHING;
-- Preserve source totals for the billing rounding tolerance on batch capture.
CREATE OR REPLACE FUNCTION activity_track_funds() RETURNS TRIGGER LANGUAGE plpgsql AS $$
DECLARE f activity_funds%ROWTYPE; delta NUMERIC; taken NUMERIC; p NUMERIC:=0; g NUMERIC:=0; u NUMERIC:=0;
 ctx JSONB; credit JSONB; src TEXT:='adjustment'; eligible NUMERIC:=0; hold activity_fund_holds%ROWTYPE;
 cost NUMERIC; remaining NUMERIC; batch TEXT;
BEGIN
 IF TG_OP='INSERT' THEN
  INSERT INTO activity_funds(user_id,gift) VALUES(NEW.id,GREATEST(NEW.balance,0));
  INSERT INTO activity_fund_ledger(user_id,source,gift) VALUES(NEW.id,'registration',GREATEST(NEW.balance,0));
  RETURN NEW;
 END IF;
 delta:=NEW.balance-OLD.balance;
 IF delta=0 AND NEW.frozen_balance IS NOT DISTINCT FROM OLD.frozen_balance THEN RETURN NEW; END IF;
 SELECT * INTO f FROM activity_funds WHERE user_id=NEW.id FOR UPDATE;
 IF NOT FOUND THEN RAISE EXCEPTION 'Missing activity fund source account'; END IF;
 ctx:=NULLIF(current_setting('activity.settlement',true),'')::jsonb;
 credit:=NULLIF(current_setting('activity.credit',true),'')::jsonb;
 batch:=COALESCE(ctx->>'batch','');
 IF NEW.frozen_balance>OLD.frozen_balance THEN
  -- Reserve exact source composition per batch; no consumption yet.
  taken:=OLD.balance-NEW.balance; g:=LEAST(taken,f.gift); u:=LEAST(taken-g,f.unverified); p:=LEAST(taken-g-u,f.principal);
  IF batch='' THEN batch:='legacy:'||txid_current(); END IF;
  INSERT INTO activity_fund_holds(user_id,batch_id,principal,gift,unverified) VALUES(NEW.id,batch,p,g,u);
  src:='hold'; p:=-p; g:=-g; u:=-u;
 ELSIF NEW.frozen_balance<OLD.frozen_balance THEN
  SELECT * INTO hold FROM activity_fund_holds WHERE user_id=NEW.id AND batch_id=batch FOR UPDATE;
  IF FOUND THEN
   cost:=GREATEST(OLD.frozen_balance-NEW.frozen_balance-delta,0);
   remaining:=cost; g:=hold.gift-LEAST(remaining,hold.gift); remaining:=GREATEST(remaining-hold.gift,0);
   u:=hold.unverified-LEAST(remaining,hold.unverified); remaining:=GREATEST(remaining-hold.unverified,0);
   p:=hold.principal-LEAST(remaining,hold.principal); eligible:=LEAST(remaining,hold.principal);
   -- Billing permits a one-unit (1e-8) rounding excess over the hold.
   -- Consume that excess from available sources instead of leaving a phantom balance.
   remaining:=GREATEST(cost-hold.gift-hold.unverified-hold.principal,0);
   taken:=LEAST(remaining,f.gift); g:=g-taken; remaining:=remaining-taken;
   taken:=LEAST(remaining,f.unverified); u:=u-taken; remaining:=remaining-taken;
   taken:=LEAST(remaining,f.principal); p:=p-taken; eligible:=eligible+taken;
   DELETE FROM activity_fund_holds WHERE user_id=NEW.id AND batch_id=batch;
   src:=CASE WHEN cost>0 THEN 'batch_capture' ELSE 'batch_release' END;
  ELSE
   -- Pre-migration holds retain their total value but no guessed principal.
   u:=GREATEST(delta,0); src:='legacy_hold_release';
  END IF;
 ELSIF delta>0 THEN
  src:='gift'; g:=delta;
  IF (credit->>'user')::bigint=NEW.id AND (credit->>'amount')::numeric=delta THEN
   p:=LEAST(delta,(credit->>'principal')::numeric); g:=delta-p; src:=credit->>'source';
  END IF;
  -- Credits first repay any pre-existing overdraft; do not create spendable money twice.
  taken:=LEAST(GREATEST(-OLD.balance,0),delta); remaining:=taken;
  cost:=LEAST(remaining,g); g:=g-cost; remaining:=remaining-cost;
  p:=GREATEST(p-remaining,0);
  PERFORM set_config('activity.credit','',true);
 ELSIF delta<0 THEN
  taken:=-delta;
  IF NULLIF(current_setting('activity.refund_amount',true),'') IS NOT NULL THEN
   p:=-taken; src:='refund_hold';
  ELSE
  g:=LEAST(taken,f.gift); u:=LEAST(taken-g,f.unverified); p:=LEAST(taken-g-u,f.principal);
  IF ctx->>'kind'='usage' THEN src:='usage'; IF COALESCE((ctx->>'request_type')::int,0)<>4 THEN eligible:=p; END IF; END IF;
  p:=-p; g:=-g; u:=-u;
  END IF;
 END IF;
 UPDATE activity_funds SET principal=principal+p,gift=gift+g,unverified=unverified+u WHERE user_id=NEW.id;
 INSERT INTO activity_fund_ledger(user_id,source,principal,gift,unverified,request_id,api_key_id,eligible_cost)
 VALUES(NEW.id,src,p,g,u,ctx->>'request',NULLIF(ctx->>'key','')::bigint,eligible);
 PERFORM set_config('activity.settlement','',true);

 RETURN NEW;
END $$;

-- Worker-only reservation. Event time determines the campaign; processing delay
-- never extends the claim deadline or counts spending after pause/end.
CREATE FUNCTION activity_process_consumption_event(event_id BIGINT) RETURNS BOOLEAN LANGUAGE plpgsql AS $$
DECLARE event activity_fund_ledger%ROWTYPE; c RECORD; t RECORD; shared_limit BIGINT; user_limit BIGINT;
 spent BIGINT; user_spent BIGINT; used BIGINT; progress NUMERIC; retry BOOLEAN:=FALSE;
BEGIN
 SELECT * INTO STRICT event FROM activity_fund_ledger WHERE id=event_id;
 PERFORM 1 FROM users WHERE id=event.user_id FOR UPDATE;
 -- Do not resurrect revoked or reviewed rewards through queue retries.
 IF NOT EXISTS(SELECT 1 FROM users WHERE id=event.user_id AND status='active' AND deleted_at IS NULL) THEN RETURN TRUE; END IF;
 FOR c IN SELECT * FROM activity_consumption_campaigns
 WHERE published_at IS NOT NULL AND state IN ('active','paused','closed')
 AND published_at<=event.created_at AND starts_at<=event.created_at
 AND event.created_at<LEAST(ends_at,COALESCE(stopped_at,ends_at))
 AND claim_until>NOW() AND event.user_id%100<rollout_percent ORDER BY id LOOP
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
-- A rolled-back binary must not accidentally restore synchronous reservation.
CREATE OR REPLACE FUNCTION activity_reserve_consumption(uid BIGINT) RETURNS VOID LANGUAGE plpgsql AS $$
BEGIN RAISE EXCEPTION 'Synchronous reward processing disabled; use the durable worker'; END $$;
