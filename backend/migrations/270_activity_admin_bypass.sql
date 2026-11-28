-- Admins participate without audience/rollout thresholds. Financial safeguards
-- (activity enabled/awards switches, lottery/campaign budgets, refund review)
-- remain enforced by the service layer.
CREATE OR REPLACE FUNCTION activity_user_is_admin(uid BIGINT) RETURNS BOOLEAN LANGUAGE SQL STABLE AS $$
    SELECT COALESCE(
        (SELECT role = 'admin' FROM users WHERE id = uid AND deleted_at IS NULL),
        FALSE
    )
$$;

CREATE OR REPLACE FUNCTION activity_audience_allowed(activity_kind TEXT, uid BIGINT) RETURNS BOOLEAN LANGUAGE SQL STABLE AS $$
    SELECT activity_user_is_admin(uid) OR COALESCE(
        (SELECT mode = 'all' OR uid = ANY(user_ids) FROM activity_audiences WHERE kind = activity_kind),
        FALSE
    )
$$;

-- Admins bypass the percentage rollout as well. Allowlisted users in
-- 'selected' mode keep the existing bypass behavior.
CREATE OR REPLACE FUNCTION activity_rollout_allowed(activity_kind TEXT, rollout INT, uid BIGINT) RETURNS BOOLEAN LANGUAGE SQL STABLE AS $$
    SELECT activity_user_is_admin(uid) OR COALESCE(
        (SELECT CASE
            WHEN mode = 'selected' THEN uid = ANY(user_ids)
            WHEN mode = 'all' THEN uid % 100 < rollout
            ELSE FALSE
        END
        FROM activity_audiences WHERE kind = activity_kind),
        FALSE
    )
$$;
