#!/usr/bin/env bash
set -euo pipefail
: "${PGHOST:?Set PGHOST explicitly to the intended database server}"
: "${PGDATABASE:?Set PGDATABASE to the database name (not a connection URI)}"
action="${1:-status}"
case "$action" in
 status) exec psql -X -v ON_ERROR_STOP=1 -f "$(dirname "$0")/activity-rewards-preflight.sql" ;;
 stop-awards) statement='UPDATE activity_runtime SET awards_enabled=false,updated_at=NOW() WHERE id=1;' ;;
 stop-processing) statement='UPDATE activity_runtime SET processing_enabled=false,updated_at=NOW() WHERE id=1;' ;;
 enable-processing) statement='UPDATE activity_runtime SET processing_enabled=true,updated_at=NOW() WHERE id=1;' ;;
 enable-awards) statement="DO \$\$ BEGIN
 IF EXISTS(SELECT 1 FROM users u LEFT JOIN activity_funds f ON f.user_id=u.id WHERE f.user_id IS NULL OR f.principal+f.gift+f.unverified<>GREATEST(u.balance,0)) THEN RAISE EXCEPTION 'Funds reconciliation failed'; END IF;
 IF EXISTS(SELECT 1 FROM users u LEFT JOIN (SELECT user_id,SUM(principal+gift+unverified) held FROM activity_fund_holds GROUP BY user_id) h ON h.user_id=u.id WHERE COALESCE(h.held,0)<>u.frozen_balance) THEN RAISE EXCEPTION 'Drain or reconcile frozen funds before activation'; END IF;
 IF NOT EXISTS(SELECT 1 FROM activity_runtime WHERE id=1 AND processing_enabled) THEN RAISE EXCEPTION 'Enable reward processing first'; END IF;
 END \$\$;
 UPDATE activity_runtime SET awards_enabled=true,updated_at=NOW() WHERE id=1;" ;;
 *) echo 'Usage: activity-rewards-control.sh status|stop-awards|stop-processing|enable-processing|enable-awards' >&2; exit 2 ;;
esac
: "${ACTIVITY_OPERATOR_ID:?Set ACTIVITY_OPERATOR_ID to the administrator responsible for this change}"
[[ "$ACTIVITY_OPERATOR_ID" =~ ^[1-9][0-9]*$ ]] || { echo 'ACTIVITY_OPERATOR_ID must be a positive integer' >&2; exit 2; }
# Use libpq environment or .pgpass; credentials never enter command arguments.
psql -X -v ON_ERROR_STOP=1 -v actor="$ACTIVITY_OPERATOR_ID" -v action="$action" <<SQL
BEGIN;
SET LOCAL lock_timeout='5s';
SET LOCAL statement_timeout='60s';
$statement
INSERT INTO activity_consumption_audit(actor_id,action,detail) VALUES(:'actor'::bigint,'runtime:' || :'action',json_build_object('operator',current_user));
COMMIT;
SQL
