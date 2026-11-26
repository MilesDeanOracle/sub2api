-- Compatibility prelude for the retired 257-263 trigger-based lineage. The
-- original files/checksums are retained; fresh/isolated installations skip them.
-- Deploy with old application writers drained. No historical money is reissued.
DO $$
DECLARE con RECORD;
BEGIN
 IF to_regclass('activity_config') IS NULL THEN RETURN; END IF;
 IF EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema=current_schema() AND table_name='activity_config' AND column_name='checkin_require_recharge') THEN RETURN; END IF;
 LOCK TABLE users,usage_logs,payment_orders,redeem_codes IN ACCESS EXCLUSIVE MODE;
 IF EXISTS(SELECT 1 FROM users WHERE COALESCE(frozen_balance,0)>0)
 OR EXISTS(SELECT 1 FROM payment_orders WHERE status IN ('REFUNDING','REFUND_PENDING')) THEN
  RAISE EXCEPTION 'Activity legacy upgrade requires settled balance holds/refunds and drained old application writers';
 END IF;
 IF to_regclass('activity_fund_holds') IS NOT NULL THEN
  IF EXISTS(SELECT 1 FROM activity_fund_holds) THEN RAISE EXCEPTION 'Settle legacy activity fund holds before upgrade'; END IF;
 END IF;
 DROP TRIGGER IF EXISTS trg_activity_first_call ON usage_logs;
 DROP TRIGGER IF EXISTS trg_activity_refund_review ON payment_orders;
 DROP TRIGGER IF EXISTS activity_payment_source_trigger ON redeem_codes;
 DROP TRIGGER IF EXISTS activity_track_funds_trigger ON users;
 DROP TRIGGER IF EXISTS activity_consumption_refund_trigger ON payment_orders;
 IF to_regclass('activity_fund_ledger') IS NOT NULL THEN
  DROP TRIGGER IF EXISTS activity_enqueue_consumption_trigger ON activity_fund_ledger;
 END IF;
 DROP FUNCTION IF EXISTS activity_process_consumption_event(BIGINT);
 ALTER TABLE activity_config ADD COLUMN checkin_require_recharge BOOLEAN NOT NULL DEFAULT TRUE;
 ALTER TABLE activity_config ADD COLUMN IF NOT EXISTS checkin_min_spend_cents BIGINT NOT NULL DEFAULT 1;
 IF to_regclass('activity_worker_status') IS NOT NULL THEN
  ALTER TABLE activity_worker_status ADD COLUMN IF NOT EXISTS cursor_initialized BOOLEAN NOT NULL DEFAULT FALSE;
  ALTER TABLE activity_worker_status ADD COLUMN IF NOT EXISTS last_usage_log_id BIGINT NOT NULL DEFAULT 0;
  ALTER TABLE activity_worker_status ADD COLUMN IF NOT EXISTS last_refund_scan_at TIMESTAMPTZ;
 END IF;
 IF to_regclass('activity_consumption_outbox') IS NOT NULL THEN
  ALTER TABLE activity_consumption_outbox RENAME TO activity_legacy_consumption_outbox;
  ALTER INDEX IF EXISTS idx_activity_outbox_pending RENAME TO idx_activity_legacy_outbox_pending;
  ALTER INDEX IF EXISTS idx_activity_outbox_user_pending RENAME TO idx_activity_legacy_outbox_user_pending;
 END IF;
 IF to_regclass('activity_consumption_tiers') IS NOT NULL THEN
  ALTER TABLE activity_consumption_tiers DROP CONSTRAINT IF EXISTS activity_consumption_tiers_position_check;
  ALTER TABLE activity_consumption_tiers ADD CONSTRAINT activity_consumption_tiers_position_check CHECK(position BETWEEN 1 AND 5);
 END IF;
 -- Isolated activity persistence must not retain legacy cascading core FKs.
 FOR con IN SELECT c.conname,c.conrelid::regclass AS tbl FROM pg_constraint c
 JOIN pg_class own ON own.oid=c.conrelid JOIN pg_namespace ns ON ns.oid=own.relnamespace
 WHERE c.contype='f' AND ns.nspname=current_schema() AND own.relname LIKE 'activity_%'
 AND c.confrelid IN ('users'::regclass,'usage_logs'::regclass,'payment_orders'::regclass,'redeem_codes'::regclass)
 LOOP EXECUTE format('ALTER TABLE %s DROP CONSTRAINT %I',con.tbl,con.conname); END LOOP;
 CREATE TABLE activity_legacy_transition(id INT PRIMARY KEY CHECK(id=1), transitioned_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp());
 INSERT INTO activity_legacy_transition(id) VALUES(1);
END $$;
