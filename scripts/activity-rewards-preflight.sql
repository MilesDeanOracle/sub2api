-- READ ONLY. Run against a restored rehearsal database first.
-- Set PGHOST, PGPORT, PGUSER, PGDATABASE and .pgpass, then run:
-- psql -X -v ON_ERROR_STOP=1 -f scripts/activity-rewards-preflight.sql
BEGIN READ ONLY;
SET LOCAL statement_timeout='60s';
SELECT COUNT(*) AS users,COUNT(*) FILTER(WHERE balance>0) AS positive_balances,
 COUNT(*) FILTER(WHERE frozen_balance>0) AS users_with_frozen_funds FROM users;
SELECT COUNT(*) AS missing_or_mismatched_fund_accounts
 FROM users u LEFT JOIN activity_funds f ON f.user_id=u.id
 WHERE f.user_id IS NULL OR f.principal+f.gift+f.unverified<>GREATEST(u.balance,0);
SELECT COUNT(*) AS unresolved_frozen_source_accounts
 FROM users u LEFT JOIN (SELECT user_id,SUM(principal+gift+unverified) held FROM activity_fund_holds GROUP BY user_id) h ON h.user_id=u.id
 WHERE COALESCE(h.held,0)<>u.frozen_balance;
SELECT COUNT(*) AS users_with_unverified_history,COALESCE(SUM(unverified),0) AS unverified_total FROM activity_funds WHERE unverified>0;
SELECT COUNT(*) AS ambiguous_refunds FROM payment_orders WHERE status IN ('REFUNDING','REFUND_PENDING');
SELECT * FROM activity_runtime;
SELECT COUNT(*) FILTER(WHERE processed_at IS NULL) AS pending,
 COUNT(*) FILTER(WHERE processed_at IS NULL AND last_error<>'') AS deferred,
 MIN(created_at) FILTER(WHERE processed_at IS NULL) AS oldest_pending FROM activity_consumption_outbox;
SELECT * FROM activity_worker_status;
ROLLBACK;
