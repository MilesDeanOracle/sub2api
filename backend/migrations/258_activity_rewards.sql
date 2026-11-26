-- Isolated in-site activity tables.
-- ZERO modifications, triggers, or foreign keys on sub2api core tables
-- (users, usage_logs, payment_orders, redeem_codes, settings, etc.).

CREATE TABLE IF NOT EXISTS activity_config (
    id INT PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    first_call_tracking_started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    checkin_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    checkin_require_recharge BOOLEAN NOT NULL DEFAULT TRUE,
    checkin_rollout_percent INT NOT NULL DEFAULT 50 CHECK (checkin_rollout_percent BETWEEN 0 AND 100),
    second_recharge_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    checkin_daily_cents INT NOT NULL DEFAULT 10 CHECK (checkin_daily_cents BETWEEN 0 AND 1500),
    checkin_milestone_cents INT NOT NULL DEFAULT 50 CHECK (checkin_milestone_cents BETWEEN 0 AND 1500),
    checkin_monthly_cap_cents INT NOT NULL DEFAULT 1500 CHECK (checkin_monthly_cap_cents BETWEEN 0 AND 1500),
    checkin_monthly_budget_cents BIGINT NOT NULL DEFAULT 0 CHECK (checkin_monthly_budget_cents >= 0),
    second_recharge_reward_cents INT NOT NULL DEFAULT 0 CHECK (second_recharge_reward_cents >= 0),
    second_recharge_total_budget_cents BIGINT NOT NULL DEFAULT 0 CHECK (second_recharge_total_budget_cents >= 0),
    second_recharge_min_pay_cents INT NOT NULL DEFAULT 5000 CHECK (second_recharge_min_pay_cents >= 0),
    total_budget_cents BIGINT NOT NULL DEFAULT 0 CHECK (total_budget_cents >= 0),
    user_lifetime_cap_cents BIGINT NOT NULL DEFAULT 0 CHECK (user_lifetime_cap_cents >= 0),
    checkin_min_spend_cents BIGINT NOT NULL DEFAULT 1 CHECK (checkin_min_spend_cents >= 0),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
INSERT INTO activity_config (id) VALUES (1) ON CONFLICT DO NOTHING;

CREATE TABLE IF NOT EXISTS activity_runtime (
    id INT PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    processing_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    awards_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
INSERT INTO activity_runtime (id) VALUES (1) ON CONFLICT DO NOTHING;

CREATE TABLE IF NOT EXISTS activity_worker_status (
    id INT PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    cursor_initialized BOOLEAN NOT NULL DEFAULT FALSE,
    last_usage_log_id BIGINT NOT NULL DEFAULT 0,
    last_refund_scan_at TIMESTAMPTZ,
    last_success_at TIMESTAMPTZ,
    last_error_at TIMESTAMPTZ,
    last_error TEXT NOT NULL DEFAULT ''
);
INSERT INTO activity_worker_status (id) VALUES (1) ON CONFLICT DO NOTHING;

CREATE TABLE IF NOT EXISTS activity_audiences (
    kind TEXT PRIMARY KEY CHECK (kind IN ('checkin', 'consumption')),
    mode TEXT NOT NULL DEFAULT 'selected' CHECK (mode IN ('selected', 'all')),
    user_ids BIGINT[] NOT NULL DEFAULT '{}',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
INSERT INTO activity_audiences (kind) VALUES ('checkin'), ('consumption') ON CONFLICT DO NOTHING;

CREATE TABLE IF NOT EXISTS activity_first_calls (
    user_id BIGINT PRIMARY KEY,
    request_id VARCHAR(64) NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS activity_checkins (
    user_id BIGINT NOT NULL,
    local_day DATE NOT NULL,
    streak INT NOT NULL,
    reward_cents INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, local_day)
);

-- Separate activity reward wallet: rewards accumulate here first and only enter
-- users.balance when the user manually withdraws them.
CREATE TABLE IF NOT EXISTS activity_wallets (
    user_id BIGINT PRIMARY KEY,
    available_cents BIGINT NOT NULL DEFAULT 0 CHECK (available_cents >= 0),
    total_earned_cents BIGINT NOT NULL DEFAULT 0 CHECK (total_earned_cents >= 0),
    total_withdrawn_cents BIGINT NOT NULL DEFAULT 0 CHECK (total_withdrawn_cents >= 0),
    unabsorbed_gift NUMERIC(20,8) NOT NULL DEFAULT 0 CHECK (unabsorbed_gift >= 0),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS activity_reward_ledger (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL,
    kind VARCHAR(24) NOT NULL,
    source_id VARCHAR(80) NOT NULL,
    amount_cents BIGINT NOT NULL CHECK (amount_cents > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (kind, source_id)
);
CREATE INDEX IF NOT EXISTS idx_activity_reward_user_month
    ON activity_reward_ledger (user_id, created_at);
CREATE UNIQUE INDEX IF NOT EXISTS uq_activity_second_reward_user
    ON activity_reward_ledger (user_id) WHERE kind = 'second_recharge';

CREATE TABLE IF NOT EXISTS activity_wallet_ledger (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL,
    direction VARCHAR(16) NOT NULL CHECK (direction IN ('credit', 'withdraw')),
    kind VARCHAR(24) NOT NULL,
    source_id VARCHAR(80) NOT NULL,
    amount_cents BIGINT NOT NULL CHECK (amount_cents > 0),
    wallet_after_cents BIGINT NOT NULL CHECK (wallet_after_cents >= 0),
    user_balance_after NUMERIC(20,8),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (direction, kind, source_id)
);
CREATE INDEX IF NOT EXISTS idx_activity_wallet_ledger_user_time
    ON activity_wallet_ledger (user_id, created_at DESC);

CREATE TABLE IF NOT EXISTS activity_usage_events (
    usage_log_id BIGINT PRIMARY KEY,
    user_id BIGINT NOT NULL,
    request_id VARCHAR(64) NOT NULL DEFAULT '',
    api_key_id BIGINT NOT NULL DEFAULT 0,
    actual_cost NUMERIC(20,8) NOT NULL CHECK (actual_cost >= 0),
    absorbed_gift NUMERIC(20,8) NOT NULL DEFAULT 0 CHECK (absorbed_gift >= 0),
    eligible_cost NUMERIC(20,8) NOT NULL CHECK (eligible_cost >= 0),
    occurred_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    reversed_at TIMESTAMPTZ,
    reverse_note TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_activity_usage_events_user_time
    ON activity_usage_events (user_id, occurred_at);

CREATE TABLE IF NOT EXISTS activity_consumption_outbox (
    event_id BIGINT PRIMARY KEY REFERENCES activity_usage_events(usage_log_id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    processed_at TIMESTAMPTZ,
    attempts INT NOT NULL DEFAULT 0,
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_error TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_activity_outbox_pending
    ON activity_consumption_outbox (next_attempt_at, event_id) WHERE processed_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_activity_outbox_user_pending
    ON activity_consumption_outbox (user_id) WHERE processed_at IS NULL;

CREATE TABLE IF NOT EXISTS activity_refund_reviews (
    order_id BIGINT PRIMARY KEY,
    user_id BIGINT NOT NULL,
    order_status VARCHAR(30) NOT NULL,
    review_status VARCHAR(20) NOT NULL DEFAULT 'pending',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_activity_refund_reviews_user_pending
    ON activity_refund_reviews (user_id) WHERE review_status = 'pending';

CREATE TABLE IF NOT EXISTS activity_consumption_campaigns (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    starts_at TIMESTAMPTZ NOT NULL,
    ends_at TIMESTAMPTZ NOT NULL,
    claim_until TIMESTAMPTZ NOT NULL,
    stopped_at TIMESTAMPTZ,
    rollout_percent INT NOT NULL DEFAULT 10 CHECK (rollout_percent BETWEEN 1 AND 100),
    budget_cents BIGINT NOT NULL DEFAULT 0 CHECK (budget_cents >= 0),
    state TEXT NOT NULL DEFAULT 'draft' CHECK (state IN ('draft', 'active', 'paused', 'closed')),
    published_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (ends_at > starts_at),
    CHECK (claim_until >= ends_at + interval '1 day')
);

CREATE TABLE IF NOT EXISTS activity_consumption_tiers (
    campaign_id BIGINT NOT NULL REFERENCES activity_consumption_campaigns(id) ON DELETE CASCADE,
    position INT NOT NULL CHECK (position BETWEEN 1 AND 5),
    threshold_cents BIGINT NOT NULL CHECK (threshold_cents > 0),
    reward_cents INT NOT NULL CHECK (reward_cents > 0),
    PRIMARY KEY (campaign_id, position)
);

CREATE TABLE IF NOT EXISTS activity_consumption_entitlements (
    campaign_id BIGINT NOT NULL,
    user_id BIGINT NOT NULL,
    position INT NOT NULL,
    reward_cents INT NOT NULL CHECK (reward_cents > 0),
    state TEXT NOT NULL DEFAULT 'reserved' CHECK (state IN ('reserved', 'claimed', 'expired', 'revoked', 'review')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    claimed_at TIMESTAMPTZ,
    PRIMARY KEY (campaign_id, user_id, position),
    FOREIGN KEY (campaign_id, position) REFERENCES activity_consumption_tiers(campaign_id, position) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_consumption_entitlements_user
    ON activity_consumption_entitlements (user_id, state);

CREATE TABLE IF NOT EXISTS activity_consumption_audit (
    id BIGSERIAL PRIMARY KEY,
    actor_id BIGINT NOT NULL,
    action TEXT NOT NULL,
    target_id BIGINT,
    detail JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE OR REPLACE FUNCTION activity_audience_allowed(activity_kind TEXT, uid BIGINT) RETURNS BOOLEAN LANGUAGE SQL STABLE AS $$
    SELECT COALESCE(
        (SELECT mode = 'all' OR uid = ANY(user_ids) FROM activity_audiences WHERE kind = activity_kind),
        FALSE
    )
$$;

-- Allowlisted users in 'selected' mode bypass percentage rollout modulo checks.
CREATE OR REPLACE FUNCTION activity_rollout_allowed(activity_kind TEXT, rollout INT, uid BIGINT) RETURNS BOOLEAN LANGUAGE SQL STABLE AS $$
    SELECT COALESCE(
        (SELECT CASE
            WHEN mode = 'selected' THEN uid = ANY(user_ids)
            WHEN mode = 'all' THEN uid % 100 < rollout
            ELSE FALSE
        END
        FROM activity_audiences WHERE kind = activity_kind),
        FALSE
    )
$$;

CREATE OR REPLACE FUNCTION activity_reserved_cents(uid BIGINT DEFAULT NULL) RETURNS BIGINT LANGUAGE SQL STABLE AS $$
    SELECT COALESCE(SUM(e.reward_cents), 0)::bigint
    FROM activity_consumption_entitlements e
    JOIN activity_consumption_campaigns c ON c.id = e.campaign_id
    WHERE e.state IN ('reserved', 'review')
      AND e.claimed_at IS NULL
      AND (e.state = 'review' OR c.claim_until > NOW())
      AND (uid IS NULL OR e.user_id = uid)
$$;

CREATE OR REPLACE FUNCTION activity_consumption_spend(cid BIGINT, uid BIGINT) RETURNS NUMERIC LANGUAGE SQL STABLE AS $$
    SELECT COALESCE(SUM(e.eligible_cost), 0)
    FROM activity_usage_events e
    JOIN activity_consumption_campaigns c ON c.id = cid
    WHERE e.user_id = uid
      AND e.reversed_at IS NULL
      AND e.occurred_at >= c.starts_at
      AND e.occurred_at < LEAST(c.ends_at, COALESCE(c.stopped_at, c.ends_at))
$$;

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
            SELECT COALESCE(SUM(amount_cents), 0) INTO user_spent FROM activity_reward_ledger WHERE user_id = ev.user_id;
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
