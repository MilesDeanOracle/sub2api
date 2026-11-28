-- Each activity uses its own budget. Keep the existing per-user reward cap.
CREATE OR REPLACE FUNCTION activity_process_consumption_event(p_event_id BIGINT) RETURNS BOOLEAN LANGUAGE plpgsql AS $$
DECLARE
    ev activity_usage_events%ROWTYPE;
    c RECORD;
    t RECORD;
    user_limit BIGINT;
    user_spent BIGINT;
    used BIGINT;
    progress NUMERIC;
    retry BOOLEAN := FALSE;
BEGIN
    SELECT * INTO STRICT ev FROM activity_usage_events WHERE usage_log_id = p_event_id;
    PERFORM 1 FROM activity_wallets WHERE user_id = ev.user_id FOR UPDATE;
    PERFORM 1 FROM activity_audiences WHERE kind = 'consumption' FOR SHARE;

    IF NOT EXISTS (SELECT 1 FROM users WHERE id = ev.user_id AND status = 'active' AND deleted_at IS NULL) THEN
        RETURN TRUE;
    END IF;

    FOR c IN
        SELECT * FROM activity_consumption_campaigns
        WHERE published_at IS NOT NULL
          AND state IN ('active', 'paused', 'closed')
          AND published_at <= ev.occurred_at
          AND starts_at <= ev.occurred_at
          AND ev.occurred_at < LEAST(ends_at, COALESCE(stopped_at, ends_at))
          AND claim_until > NOW()
          AND activity_rollout_allowed('consumption', rollout_percent, ev.user_id)
        ORDER BY id
    LOOP
        IF EXISTS (SELECT 1 FROM activity_consumption_entitlements WHERE user_id = ev.user_id AND state = 'review')
           OR EXISTS (SELECT 1 FROM activity_refund_reviews WHERE user_id = ev.user_id AND review_status = 'pending') THEN
            RETURN FALSE;
        END IF;

        progress := activity_consumption_spend(c.id, ev.user_id);
        SELECT user_lifetime_cap_cents INTO user_limit FROM activity_config WHERE id = 1 FOR UPDATE;

        FOR t IN SELECT * FROM activity_consumption_tiers WHERE campaign_id = c.id ORDER BY position LOOP
            IF progress * 100 < t.threshold_cents
               OR EXISTS (SELECT 1 FROM activity_consumption_entitlements WHERE campaign_id = c.id AND user_id = ev.user_id AND position = t.position) THEN
                CONTINUE;
            END IF;

            SELECT COALESCE(SUM(amount_cents), 0) INTO user_spent FROM activity_reward_ledger WHERE user_id = ev.user_id AND kind <> 'lottery';
            SELECT COALESCE(SUM(reward_cents), 0) INTO used
            FROM activity_consumption_entitlements
            WHERE campaign_id = c.id
              AND (claimed_at IS NOT NULL OR (state IN ('reserved', 'review') AND (state = 'review' OR c.claim_until > NOW())));

            IF user_spent + activity_reserved_cents(ev.user_id) + t.reward_cents > user_limit
               OR used + t.reward_cents > c.budget_cents THEN
                retry := TRUE;
                CONTINUE;
            END IF;

            INSERT INTO activity_consumption_entitlements (campaign_id, user_id, position, reward_cents)
            VALUES (c.id, ev.user_id, t.position, t.reward_cents)
            ON CONFLICT DO NOTHING;
        END LOOP;
    END LOOP;

    RETURN NOT retry;
END $$;
