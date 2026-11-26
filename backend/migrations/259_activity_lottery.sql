-- Isolated recharge-lottery tables (Mystery Chest).
-- ZERO modifications, triggers, or foreign keys on sub2api core tables
-- (users, usage_logs, payment_orders, redeem_codes, settings, etc.).

ALTER TABLE activity_audiences DROP CONSTRAINT IF EXISTS activity_audiences_kind_check;
ALTER TABLE activity_audiences
    ADD CONSTRAINT activity_audiences_kind_check CHECK (kind IN ('checkin', 'consumption', 'lottery'));
INSERT INTO activity_audiences (kind) VALUES ('lottery') ON CONFLICT DO NOTHING;

CREATE TABLE IF NOT EXISTS activity_lottery_config (
    id INT PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    enabled BOOLEAN NOT NULL DEFAULT FALSE,
    unit_pay_cents INT NOT NULL DEFAULT 1000 CHECK (unit_pay_cents >= 100),
    budget_cents BIGINT NOT NULL DEFAULT 0 CHECK (budget_cents >= 0),
    tiers JSONB NOT NULL DEFAULT '[{"position":1,"reward_cents":50,"weight":8800},{"position":2,"reward_cents":100,"weight":950},{"position":3,"reward_cents":300,"weight":200},{"position":4,"reward_cents":500,"weight":40},{"position":5,"reward_cents":1000,"weight":9},{"position":6,"reward_cents":5000,"weight":1}]'::jsonb,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
INSERT INTO activity_lottery_config (id) VALUES (1) ON CONFLICT DO NOTHING;

CREATE TABLE IF NOT EXISTS activity_lottery_windows (
    id BIGSERIAL PRIMARY KEY,
    unit_pay_cents INT NOT NULL CHECK (unit_pay_cents >= 100),
    opened_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    closed_at TIMESTAMPTZ,
    CHECK (closed_at IS NULL OR closed_at >= opened_at)
);
CREATE INDEX IF NOT EXISTS idx_activity_lottery_windows_time
    ON activity_lottery_windows (opened_at, closed_at);

CREATE TABLE IF NOT EXISTS activity_lottery_orders (
    order_id BIGINT PRIMARY KEY,
    user_id BIGINT NOT NULL,
    unit_pay_cents INT NOT NULL CHECK (unit_pay_cents >= 100),
    granted_tickets INT NOT NULL CHECK (granted_tickets >= 0),
    effective_tickets INT NOT NULL CHECK (effective_tickets >= 0),
    last_refund_cents BIGINT NOT NULL DEFAULT 0 CHECK (last_refund_cents >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_activity_lottery_orders_user
    ON activity_lottery_orders (user_id);

CREATE TABLE IF NOT EXISTS activity_lottery_ticket_ledger (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL,
    source_type VARCHAR(24) NOT NULL CHECK (source_type IN ('order_grant', 'order_refund', 'draw')),
    source_id VARCHAR(80) NOT NULL,
    order_id BIGINT,
    delta_tickets INT NOT NULL CHECK (delta_tickets <> 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (source_type, source_id)
);
CREATE INDEX IF NOT EXISTS idx_activity_lottery_tickets_user
    ON activity_lottery_ticket_ledger (user_id, id DESC);

CREATE TABLE IF NOT EXISTS activity_lottery_draws (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL,
    batch_key VARCHAR(64) NOT NULL,
    draw_count SMALLINT NOT NULL CHECK (draw_count IN (1, 10)),
    draw_index SMALLINT NOT NULL CHECK (draw_index BETWEEN 1 AND 10),
    tier_position SMALLINT NOT NULL CHECK (tier_position BETWEEN 1 AND 6),
    reward_cents INT NOT NULL CHECK (reward_cents > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, batch_key, draw_index)
);
CREATE INDEX IF NOT EXISTS idx_activity_lottery_draws_user_time
    ON activity_lottery_draws (user_id, created_at DESC, id DESC);

-- Lottery rewards share site-wide total_budget_cents, while user_lifetime_cap_cents
-- applies only to non-lottery subsidies (check-in, consumption, second_recharge).
CREATE OR REPLACE FUNCTION activity_process_consumption_event(p_event_id BIGINT) RETURNS BOOLEAN LANGUAGE plpgsql AS $$
DECLARE
    ev activity_usage_events%ROWTYPE;
    c RECORD;
    t RECORD;
    shared_limit BIGINT;
    user_limit BIGINT;
    spent BIGINT;
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
        SELECT total_budget_cents, user_lifetime_cap_cents INTO shared_limit, user_limit FROM activity_config WHERE id = 1 FOR UPDATE;

        FOR t IN SELECT * FROM activity_consumption_tiers WHERE campaign_id = c.id ORDER BY position LOOP
            IF progress * 100 < t.threshold_cents
               OR EXISTS (SELECT 1 FROM activity_consumption_entitlements WHERE campaign_id = c.id AND user_id = ev.user_id AND position = t.position) THEN
                CONTINUE;
            END IF;

            SELECT COALESCE(SUM(amount_cents), 0) INTO spent FROM activity_reward_ledger;
            SELECT COALESCE(SUM(amount_cents), 0) INTO user_spent FROM activity_reward_ledger WHERE user_id = ev.user_id AND kind <> 'lottery';
            SELECT COALESCE(SUM(reward_cents), 0) INTO used
            FROM activity_consumption_entitlements
            WHERE campaign_id = c.id
              AND (claimed_at IS NOT NULL OR (state IN ('reserved', 'review') AND (state = 'review' OR c.claim_until > NOW())));

            IF spent + activity_reserved_cents() + t.reward_cents > shared_limit
               OR user_spent + activity_reserved_cents(ev.user_id) + t.reward_cents > user_limit
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
