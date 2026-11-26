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
 IF NOT EXISTS(SELECT 1 FROM activity_runtime WHERE id=1 AND processing_enabled) THEN RAISE EXCEPTION 'Enable reward processing first'; END IF;
 END \$\$;
 UPDATE activity_runtime SET awards_enabled=true,updated_at=NOW() WHERE id=1;" ;;
 *) echo 'Usage: activity-rewards-control.sh status|stop-awards|stop-processing|enable-processing|enable-awards' >&2; exit 2 ;;
esac
: "${ACTIVITY_OPERATOR_ID:?Set ACTIVITY_OPERATOR_ID to the administrator responsible for this change}"
[[ "$ACTIVITY_OPERATOR_ID" =~ ^[1-9][0-9]*$ ]] || { echo 'ACTIVITY_OPERATOR_ID must be a positive integer' >&2; exit 2; }
psql -X -v ON_ERROR_STOP=1 -v actor="$ACTIVITY_OPERATOR_ID" -v action="$action" <<SQL
BEGIN;
SET LOCAL lock_timeout='5s';
SET LOCAL statement_timeout='60s';
$statement
INSERT INTO activity_consumption_audit(actor_id,action,detail) VALUES(:'actor'::bigint,'runtime:' || :'action',json_build_object('operator',current_user));
COMMIT;
SQL
