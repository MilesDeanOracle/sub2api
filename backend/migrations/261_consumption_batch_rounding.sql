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
 IF eligible>0 THEN PERFORM activity_reserve_consumption(NEW.id); END IF;
 RETURN NEW;
END $$;
