-- Activity state is isolated from payment and usage tables so upstream schema
-- changes do not rewrite reward history.
CREATE TABLE IF NOT EXISTS activity_first_calls (
    user_id BIGINT PRIMARY KEY,
    request_id VARCHAR(64) NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS activity_checkins (
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    local_day DATE NOT NULL,
    streak INT NOT NULL,
    reward_cents INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, local_day)
);

CREATE TABLE IF NOT EXISTS activity_reward_ledger (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    kind VARCHAR(24) NOT NULL,
    source_id VARCHAR(80) NOT NULL,
    amount_cents INT NOT NULL CHECK (amount_cents > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (kind, source_id)
);
CREATE INDEX IF NOT EXISTS idx_activity_reward_user_month
    ON activity_reward_ledger(user_id, created_at);
CREATE UNIQUE INDEX IF NOT EXISTS uq_activity_second_reward_user
    ON activity_reward_ledger(user_id) WHERE kind='second_recharge';

CREATE TABLE IF NOT EXISTS activity_refund_reviews (
    order_id BIGINT PRIMARY KEY,
    user_id BIGINT NOT NULL,
    order_status VARCHAR(30) NOT NULL,
    review_status VARCHAR(20) NOT NULL DEFAULT 'pending',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE OR REPLACE FUNCTION activity_flag_refund_review() RETURNS trigger AS $$
BEGIN
    IF NEW.order_type = 'balance' AND NEW.status IN
        ('REFUND_REQUESTED','REFUNDING','REFUND_PENDING','PARTIALLY_REFUNDED','REFUNDED','REFUND_FAILED')
       AND OLD.status IS DISTINCT FROM NEW.status
       AND EXISTS (SELECT 1 FROM activity_reward_ledger
                   WHERE user_id=NEW.user_id AND kind='second_recharge')
       AND NEW.id IN (SELECT id FROM payment_orders
                      WHERE user_id=NEW.user_id AND order_type='balance' AND paid_at IS NOT NULL
                      ORDER BY paid_at,id LIMIT 2) THEN
        INSERT INTO activity_refund_reviews(order_id,user_id,order_status)
        VALUES(NEW.id,NEW.user_id,NEW.status)
        ON CONFLICT (order_id) DO UPDATE
        SET order_status=EXCLUDED.order_status,updated_at=NOW();
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
DROP TRIGGER IF EXISTS trg_activity_refund_review ON payment_orders;
CREATE TRIGGER trg_activity_refund_review AFTER UPDATE OF status ON payment_orders
    FOR EACH ROW EXECUTE FUNCTION activity_flag_refund_review();

CREATE OR REPLACE FUNCTION activity_capture_first_call() RETURNS trigger AS $$
BEGIN
    IF NEW.actual_cost > 0 AND COALESCE(NEW.request_type, 0) <> 4
       AND NOT EXISTS (SELECT 1 FROM activity_first_calls WHERE user_id=NEW.user_id)
       AND EXISTS (SELECT 1 FROM users u CROSS JOIN activity_config c
                   WHERE u.id=NEW.user_id AND u.created_at>=c.first_call_tracking_started_at) THEN
        INSERT INTO activity_first_calls(user_id, request_id, occurred_at)
        VALUES (NEW.user_id, NEW.request_id, NEW.created_at)
        ON CONFLICT (user_id) DO NOTHING;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
DROP TRIGGER IF EXISTS trg_activity_first_call ON usage_logs;
CREATE TRIGGER trg_activity_first_call AFTER INSERT ON usage_logs
    FOR EACH ROW EXECUTE FUNCTION activity_capture_first_call();

CREATE TABLE IF NOT EXISTS activity_config (
    id INT PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    first_call_tracking_started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    checkin_enabled BOOLEAN NOT NULL DEFAULT FALSE,
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
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
INSERT INTO activity_config (id) VALUES (1) ON CONFLICT DO NOTHING;
