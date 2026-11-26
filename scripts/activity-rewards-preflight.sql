-- READ ONLY. Inspect isolated activity tables and worker status.
BEGIN READ ONLY;
SET LOCAL statement_timeout='60s';
SELECT * FROM activity_runtime;
SELECT * FROM activity_worker_status;
SELECT * FROM activity_refund_sync_queue ORDER BY next_attempt_at,user_id;
SELECT COUNT(*) AS wallet_users,
       COALESCE(SUM(available_cents), 0) AS total_available_cents,
       COALESCE(SUM(total_earned_cents), 0) AS total_earned_cents,
       COALESCE(SUM(total_withdrawn_cents), 0) AS total_withdrawn_cents,
       COALESCE(SUM(recovery_debt_cents), 0) AS recovery_debt_cents
FROM activity_wallets;
SELECT COUNT(*) FILTER (WHERE processed_at IS NULL) AS pending,
       COUNT(*) FILTER (WHERE processed_at IS NULL AND last_error <> '') AS deferred,
       MIN(created_at) FILTER (WHERE processed_at IS NULL) AS oldest_pending
FROM activity_consumption_outbox;
SELECT COUNT(*) FILTER (WHERE review_status = 'pending') AS pending_refund_reviews
FROM activity_refund_reviews;
-- Held recovery without an in-flight order needs operator reconciliation.
SELECT r.*,o.status AS order_status
FROM activity_reward_recoveries r
LEFT JOIN payment_orders o ON r.kind='lottery_refund' AND split_part(r.source_id,':',1)=o.id::text
WHERE r.state='held'
ORDER BY r.created_at;
SELECT COUNT(*) AS legacy_draws_requiring_attribution FROM activity_lottery_draws WHERE order_id IS NULL;
SELECT kind,state,COUNT(*) AS count,SUM(amount_cents) AS reward_cents,SUM(debt_cents) AS original_debt_cents
FROM activity_reward_recoveries GROUP BY kind,state;
SELECT COALESCE(SUM(GREATEST(net,0)),0) AS outstanding_tickets
FROM (SELECT user_id,SUM(delta_tickets) AS net FROM activity_lottery_ticket_ledger GROUP BY user_id) t;
-- Budget accounting is gross: recovered rewards do not reopen spent budget.
SELECT * FROM activity_lottery_config;
ROLLBACK;
