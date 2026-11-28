package activitystore

import (
	"context"
	"database/sql"
)

// ConsumptionRuntime uses durable acknowledgements, rather than the diagnostic
// log cursor, to include late legacy commits and exclude unbilled modern logs.
func ConsumptionRuntime(ctx context.Context, db *sql.DB, uid int64) (enabled, awards, pending bool, err error) {
	err = db.QueryRowContext(ctx, `
	 SELECT cfg.enabled,r.awards_enabled,
	 EXISTS(SELECT 1 FROM activity_consumption_outbox WHERE user_id=$1 AND processed_at IS NULL)
	 OR (r.processing_enabled AND EXISTS(
	  SELECT 1 FROM usage_logs u
	  WHERE u.user_id=$1 AND COALESCE(u.billing_type,0)=0
	   AND u.actual_cost>0 AND COALESCE(u.request_type,0)<>4
	   AND u.created_at<b.cutover_at
	   AND NOT EXISTS(SELECT 1 FROM activity_usage_ingested i WHERE i.usage_log_id=u.id)
	   AND NOT EXISTS(SELECT 1 FROM activity_usage_events e WHERE e.billing_recorded AND e.request_id=u.request_id AND e.api_key_id=COALESCE(u.api_key_id,0))
	   AND EXISTS(SELECT 1 FROM activity_consumption_campaigns c
	    WHERE c.published_at IS NOT NULL AND c.published_at<=u.created_at
	     AND c.starts_at<=u.created_at AND u.created_at<LEAST(c.ends_at,COALESCE(c.stopped_at,c.ends_at))
	     AND c.claim_until>NOW() AND activity_rollout_allowed('consumption',c.rollout_percent,$1)
	     AND activity_consumption_enabled_at(u.created_at))
	 ))
	 FROM activity_runtime r CROSS JOIN activity_consumption_config cfg CROSS JOIN activity_billing_config b
	 WHERE r.id=1 AND cfg.id=1 AND b.id=1`, uid).Scan(&enabled, &awards, &pending)
	return
}
