-- Separate source accounting; users.balance remains the authoritative total.
CREATE TABLE IF NOT EXISTS activity_funds (
 user_id BIGINT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
 principal NUMERIC(20,8) NOT NULL DEFAULT 0 CHECK(principal>=0),
 gift NUMERIC(20,8) NOT NULL DEFAULT 0 CHECK(gift>=0),
 unverified NUMERIC(20,8) NOT NULL DEFAULT 0 CHECK(unverified>=0)
);
INSERT INTO activity_funds(user_id,unverified) SELECT id,GREATEST(balance,0) FROM users ON CONFLICT DO NOTHING;
CREATE TABLE IF NOT EXISTS activity_fund_ledger (
 id BIGSERIAL PRIMARY KEY, user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 source TEXT NOT NULL, principal NUMERIC(20,8) NOT NULL DEFAULT 0,
 gift NUMERIC(20,8) NOT NULL DEFAULT 0, unverified NUMERIC(20,8) NOT NULL DEFAULT 0,
 request_id TEXT, api_key_id BIGINT, eligible_cost NUMERIC(20,8) NOT NULL DEFAULT 0,
 created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(), note TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_activity_fund_user_time ON activity_fund_ledger(user_id,created_at);
CREATE UNIQUE INDEX IF NOT EXISTS uq_activity_fund_settlement ON activity_fund_ledger(request_id,api_key_id) WHERE source='usage';
INSERT INTO activity_fund_ledger(user_id,source,unverified,note)
 SELECT user_id,'opening',unverified,'Historical funds require source confirmation' FROM activity_funds;
CREATE TABLE IF NOT EXISTS activity_fund_holds (
 user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE, batch_id TEXT NOT NULL,
 principal NUMERIC(20,8) NOT NULL, gift NUMERIC(20,8) NOT NULL, unverified NUMERIC(20,8) NOT NULL,
 PRIMARY KEY(user_id,batch_id)
);
CREATE TABLE IF NOT EXISTS activity_consumption_campaigns (
 id BIGSERIAL PRIMARY KEY, name TEXT NOT NULL,
 starts_at TIMESTAMPTZ NOT NULL, ends_at TIMESTAMPTZ NOT NULL, claim_until TIMESTAMPTZ NOT NULL,
 rollout_percent INT NOT NULL DEFAULT 10 CHECK(rollout_percent BETWEEN 1 AND 100),
 budget_cents BIGINT NOT NULL DEFAULT 0 CHECK(budget_cents>=0),
 state TEXT NOT NULL DEFAULT 'draft' CHECK(state IN ('draft','active','paused','closed')),
 published_at TIMESTAMPTZ, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 CHECK(ends_at>starts_at), CHECK(claim_until>=ends_at+interval '1 day')
);
CREATE TABLE IF NOT EXISTS activity_consumption_tiers (
 campaign_id BIGINT REFERENCES activity_consumption_campaigns(id), position INT NOT NULL CHECK(position BETWEEN 1 AND 3),
 threshold_cents BIGINT NOT NULL CHECK(threshold_cents>0), reward_cents INT NOT NULL CHECK(reward_cents>0),
 PRIMARY KEY(campaign_id,position)
);
CREATE TABLE IF NOT EXISTS activity_consumption_entitlements (
 campaign_id BIGINT NOT NULL, user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 position INT NOT NULL, reward_cents INT NOT NULL CHECK(reward_cents>0),
 state TEXT NOT NULL DEFAULT 'reserved' CHECK(state IN ('reserved','claimed','expired','revoked','review')),
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), claimed_at TIMESTAMPTZ,
 PRIMARY KEY(campaign_id,user_id,position),
 FOREIGN KEY(campaign_id,position) REFERENCES activity_consumption_tiers(campaign_id,position)
);
CREATE INDEX IF NOT EXISTS idx_consumption_entitlements_user ON activity_consumption_entitlements(user_id,state);
CREATE TABLE IF NOT EXISTS activity_consumption_audit (
 id BIGSERIAL PRIMARY KEY, actor_id BIGINT NOT NULL, action TEXT NOT NULL, target_id BIGINT,
 detail JSONB NOT NULL DEFAULT '{}', created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
-- Old second-recharge rewards remain in the ledger. Stop new awards on transition.
UPDATE activity_config SET second_recharge_enabled=FALSE WHERE id=1;

CREATE OR REPLACE FUNCTION activity_reserved_cents(uid BIGINT DEFAULT NULL) RETURNS BIGINT LANGUAGE SQL STABLE AS $$
 SELECT COALESCE(SUM(e.reward_cents),0)::bigint FROM activity_consumption_entitlements e
 JOIN activity_consumption_campaigns c ON c.id=e.campaign_id
 WHERE e.state IN ('reserved','review') AND e.claimed_at IS NULL AND (e.state='review' OR c.claim_until>NOW()) AND (uid IS NULL OR e.user_id=uid)
$$;
CREATE OR REPLACE FUNCTION activity_consumption_spend(cid BIGINT,uid BIGINT) RETURNS NUMERIC LANGUAGE SQL STABLE AS $$
 SELECT COALESCE(SUM(l.eligible_cost),0) FROM activity_fund_ledger l JOIN activity_consumption_campaigns c ON c.id=cid
 WHERE l.user_id=uid AND l.created_at>=c.starts_at AND l.created_at<c.ends_at
$$;
-- Caller always locks the user first, then the shared config row (same order as check-in).
CREATE OR REPLACE FUNCTION activity_reserve_consumption(uid BIGINT) RETURNS VOID LANGUAGE plpgsql AS $$
DECLARE c RECORD; t RECORD; shared_limit BIGINT; user_limit BIGINT; spent BIGINT; user_spent BIGINT; used BIGINT; progress NUMERIC;
BEGIN
 IF NOT EXISTS(SELECT 1 FROM activity_consumption_campaigns WHERE state='active' AND starts_at<=NOW() AND ends_at>NOW() AND uid%100<rollout_percent) THEN RETURN; END IF;
 SELECT total_budget_cents,user_lifetime_cap_cents INTO shared_limit,user_limit FROM activity_config WHERE id=1 FOR UPDATE;
 IF NOT EXISTS(SELECT 1 FROM users WHERE id=uid AND status='active' AND deleted_at IS NULL) OR
 EXISTS(SELECT 1 FROM activity_consumption_entitlements WHERE user_id=uid AND state='review') OR
 EXISTS(SELECT 1 FROM activity_refund_reviews WHERE user_id=uid AND review_status='pending') THEN RETURN; END IF;
 FOR c IN SELECT * FROM activity_consumption_campaigns WHERE state='active' AND starts_at<=NOW() AND ends_at>NOW() AND uid%100<rollout_percent ORDER BY id LOOP
  progress:=activity_consumption_spend(c.id,uid);
  FOR t IN SELECT * FROM activity_consumption_tiers WHERE campaign_id=c.id ORDER BY position LOOP
   IF progress*100<t.threshold_cents OR EXISTS(SELECT 1 FROM activity_consumption_entitlements WHERE campaign_id=c.id AND user_id=uid AND position=t.position) THEN CONTINUE; END IF;
   SELECT COALESCE(SUM(amount_cents),0) INTO spent FROM activity_reward_ledger;
   SELECT COALESCE(SUM(amount_cents),0) INTO user_spent FROM activity_reward_ledger WHERE user_id=uid;
   SELECT COALESCE(SUM(reward_cents),0) INTO used FROM activity_consumption_entitlements WHERE campaign_id=c.id AND (claimed_at IS NOT NULL OR (state IN ('reserved','review') AND (state='review' OR c.claim_until>NOW())));
   IF spent+activity_reserved_cents()+t.reward_cents>shared_limit OR user_spent+activity_reserved_cents(uid)+t.reward_cents>user_limit OR used+t.reward_cents>c.budget_cents THEN CONTINUE; END IF;
   INSERT INTO activity_consumption_entitlements(campaign_id,user_id,position,reward_cents) VALUES(c.id,uid,t.position,t.reward_cents) ON CONFLICT DO NOTHING;
  END LOOP;
 END LOOP;
END $$;

-- Payment code redemption precedes the balance increment in the same transaction.
-- Only verifiable completed payment fulfillment qualifies as principal.
CREATE OR REPLACE FUNCTION activity_payment_source() RETURNS TRIGGER LANGUAGE plpgsql AS $$
DECLARE p RECORD;
BEGIN
 IF NEW.status='used' AND OLD.status IS DISTINCT FROM NEW.status AND NEW.type='balance' AND NEW.value>0 THEN
  SELECT id,user_id,amount,bonus_amount INTO p FROM payment_orders WHERE recharge_code=NEW.code AND user_id=NEW.used_by AND order_type='balance' AND paid_at IS NOT NULL AND status='RECHARGING' AND amount=NEW.value LIMIT 1;
  IF FOUND THEN
   PERFORM set_config('activity.credit',json_build_object('user',p.user_id,'amount',p.amount,'principal',GREATEST(p.amount-COALESCE(p.bonus_amount,0),0),'source','payment:'||p.id)::text,true);
  END IF;
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER activity_payment_source_trigger AFTER UPDATE OF status ON redeem_codes FOR EACH ROW EXECUTE FUNCTION activity_payment_source();

-- Every balance path is accounted for. Untagged credits are gifts; opening balances
-- stay unverified. Untagged debits do not create reward progress.
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
 IF eligible>0 THEN PERFORM activity_reserve_consumption(NEW.id); END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER activity_track_funds_trigger AFTER INSERT OR UPDATE OF balance,frozen_balance ON users FOR EACH ROW EXECUTE FUNCTION activity_track_funds();

-- Any refund event suspends new rewards until reviewed, retaining existing funds.
CREATE OR REPLACE FUNCTION activity_consumption_refund_review() RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
 IF NEW.order_type='balance' AND NEW.status IN ('REFUND_REQUESTED','REFUNDING','REFUND_PENDING','PARTIALLY_REFUNDED','REFUNDED','REFUND_FAILED') AND OLD.status IS DISTINCT FROM NEW.status THEN
  INSERT INTO activity_refund_reviews(order_id,user_id,order_status) VALUES(NEW.id,NEW.user_id,NEW.status)
  ON CONFLICT(order_id) DO UPDATE SET order_status=EXCLUDED.order_status,review_status='pending',updated_at=NOW();
  UPDATE activity_consumption_entitlements SET state='review' WHERE user_id=NEW.user_id AND state='reserved';
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER activity_consumption_refund_trigger AFTER UPDATE OF status ON payment_orders FOR EACH ROW EXECUTE FUNCTION activity_consumption_refund_review();

-- Refund reservation and return are explicit source movements, never gift cash-out.
CREATE OR REPLACE FUNCTION activity_refund_deduct(uid BIGINT,amount NUMERIC) RETURNS NUMERIC LANGUAGE plpgsql AS $$
DECLARE available NUMERIC; p NUMERIC; n NUMERIC;
BEGIN
 SELECT balance INTO available FROM users WHERE id=uid AND deleted_at IS NULL FOR UPDATE;
 IF NOT FOUND THEN RAISE EXCEPTION 'User not found'; END IF;
 SELECT principal INTO p FROM activity_funds WHERE user_id=uid FOR UPDATE;
 n:=LEAST(amount,GREATEST(available,0),p);
 IF n>0 THEN
  -- Refund context overrides allocation in activity_track_funds via separate function below.
  PERFORM set_config('activity.refund_amount',n::text,true);
  UPDATE users SET balance=balance-n,updated_at=NOW() WHERE id=uid;
  PERFORM set_config('activity.refund_amount','',true);
 END IF;
 RETURN n;
END $$;
CREATE OR REPLACE FUNCTION activity_refund_restore(uid BIGINT,amount NUMERIC) RETURNS VOID LANGUAGE plpgsql AS $$
BEGIN
 PERFORM 1 FROM users WHERE id=uid AND deleted_at IS NULL FOR UPDATE;
 IF NOT FOUND THEN RAISE EXCEPTION 'User not found'; END IF;
 PERFORM set_config('activity.credit',json_build_object('user',uid,'amount',amount,'principal',amount,'source','refund_return')::text,true);
 UPDATE users SET balance=balance+amount,updated_at=NOW() WHERE id=uid;
END $$;
ALTER TABLE activity_consumption_campaigns ADD COLUMN IF NOT EXISTS stopped_at TIMESTAMPTZ;
CREATE OR REPLACE FUNCTION activity_consumption_spend(cid BIGINT,uid BIGINT) RETURNS NUMERIC LANGUAGE SQL STABLE AS $$
 SELECT COALESCE(SUM(l.eligible_cost),0) FROM activity_fund_ledger l JOIN activity_consumption_campaigns c ON c.id=cid
 WHERE l.user_id=uid AND l.created_at>=c.starts_at AND l.created_at<LEAST(c.ends_at,COALESCE(c.stopped_at,c.ends_at))
$$;
