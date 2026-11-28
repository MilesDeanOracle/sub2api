-- Activity-only accounting metadata; no triggers or foreign keys on core tables.
ALTER TABLE activity_wallets ADD COLUMN IF NOT EXISTS recovery_debt_cents BIGINT NOT NULL DEFAULT 0 CHECK (recovery_debt_cents >= 0);
ALTER TABLE activity_refund_reviews ADD COLUMN IF NOT EXISTS refund_amount NUMERIC(20,8) NOT NULL DEFAULT 0;
ALTER TABLE activity_lottery_draws ADD COLUMN IF NOT EXISTS order_id BIGINT;
ALTER TABLE activity_lottery_draws ADD COLUMN IF NOT EXISTS order_ticket_index INT;
ALTER TABLE activity_lottery_draws ADD COLUMN IF NOT EXISTS revoked_at TIMESTAMPTZ;
CREATE UNIQUE INDEX IF NOT EXISTS uq_activity_draw_order_ticket ON activity_lottery_draws(order_id, order_ticket_index);

ALTER TABLE activity_wallet_ledger DROP CONSTRAINT IF EXISTS activity_wallet_ledger_direction_check;
ALTER TABLE activity_wallet_ledger ADD CONSTRAINT activity_wallet_ledger_direction_check CHECK (direction IN ('credit','withdraw','revoke'));
ALTER TABLE activity_lottery_ticket_ledger DROP CONSTRAINT IF EXISTS activity_lottery_ticket_ledger_source_type_check;
ALTER TABLE activity_lottery_ticket_ledger ADD CONSTRAINT activity_lottery_ticket_ledger_source_type_check CHECK (source_type IN ('order_grant','order_refund','draw','refund_recovery'));

CREATE TABLE IF NOT EXISTS activity_reward_recoveries (
    kind VARCHAR(24) NOT NULL,
    source_id VARCHAR(80) NOT NULL,
    user_id BIGINT NOT NULL,
    amount_cents BIGINT NOT NULL CHECK (amount_cents > 0),
    wallet_cents BIGINT NOT NULL CHECK (wallet_cents >= 0),
    balance_cents BIGINT NOT NULL CHECK (balance_cents >= 0),
    debt_cents BIGINT NOT NULL CHECK (debt_cents >= 0),
    absorbed_gift NUMERIC(20,8) NOT NULL DEFAULT 0,
    state TEXT NOT NULL CHECK (state IN ('held','settled','released')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (kind, source_id),
    CHECK (amount_cents = wallet_cents + balance_cents + debt_cents)
);
CREATE INDEX IF NOT EXISTS idx_activity_recoveries_user ON activity_reward_recoveries(user_id);

-- Durable per-user work avoids one bad legacy record holding the scan cursor.
CREATE TABLE IF NOT EXISTS activity_refund_sync_queue (
 user_id BIGINT PRIMARY KEY,
 updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 last_error TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_activity_refund_sync_due ON activity_refund_sync_queue(next_attempt_at,user_id);
