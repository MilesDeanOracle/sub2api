-- The runner requires an acknowledged drained window on existing installations.
LOCK TABLE users,usage_logs IN ACCESS EXCLUSIVE MODE;
DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM users WHERE COALESCE(frozen_balance,0)>0)
 OR EXISTS(SELECT 1 FROM payment_orders WHERE status IN ('REFUNDING','REFUND_PENDING')) THEN
 RAISE EXCEPTION 'Settle balance holds and pending refunds before activity billing cutover';
 END IF;
END $$;
-- Negative IDs distinguish transactional settlements from historical usage-log IDs.
CREATE SEQUENCE activity_billing_event_seq AS BIGINT INCREMENT BY -1 MAXVALUE -1 START WITH -1;
SELECT setval('activity_billing_event_seq',LEAST(-1,COALESCE((SELECT MIN(usage_log_id)-1 FROM activity_usage_events WHERE usage_log_id<0),-1)),false);
ALTER TABLE activity_usage_events ADD COLUMN billing_recorded BOOLEAN NOT NULL DEFAULT FALSE;
CREATE UNIQUE INDEX uq_activity_billing_request ON activity_usage_events(request_id,api_key_id) WHERE billing_recorded;
CREATE TABLE activity_usage_ingested(usage_log_id BIGINT PRIMARY KEY);
CREATE TABLE activity_billing_holds(
 user_id BIGINT NOT NULL,batch_id TEXT NOT NULL,
 gift NUMERIC(20,8) NOT NULL CHECK(gift>=0),
 PRIMARY KEY(user_id,batch_id)
);
-- Previously processed positive log IDs remain deduplicated on upgrade.
INSERT INTO activity_usage_ingested SELECT usage_log_id FROM activity_usage_events WHERE usage_log_id>0 ON CONFLICT DO NOTHING;
DO $$ BEGIN
 IF to_regclass('activity_legacy_transition') IS NOT NULL THEN
  -- Legacy source-ledger events have already been converted by 267. Do not
  -- reinterpret their usage logs under the new funding rules.
  INSERT INTO activity_usage_ingested SELECT id FROM usage_logs ON CONFLICT DO NOTHING;
 END IF;
END $$;

ALTER TABLE activity_worker_status ADD COLUMN usage_reconcile_id BIGINT NOT NULL DEFAULT 0;
-- Match the billing dedup key length so adding reward persistence cannot reject
-- an otherwise valid core billing request with a long request identifier.
ALTER TABLE activity_usage_events ALTER COLUMN request_id TYPE VARCHAR(255);
ALTER TABLE activity_first_calls ALTER COLUMN request_id TYPE VARCHAR(255);
CREATE TABLE activity_billing_config(
 id INT PRIMARY KEY CHECK(id=1),cutover_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);
INSERT INTO activity_billing_config(id) VALUES(1);
