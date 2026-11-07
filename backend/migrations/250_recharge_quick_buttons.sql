CREATE TABLE IF NOT EXISTS recharge_quick_buttons (
    id BIGSERIAL PRIMARY KEY,
    label VARCHAR(50) NOT NULL DEFAULT '',
    icon_url TEXT NOT NULL DEFAULT '',
    url TEXT NOT NULL,
    open_method VARCHAR(20) NOT NULL DEFAULT 'new_tab',
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    sort_order INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
-- Indexes
CREATE INDEX IF NOT EXISTS idx_recharge_quick_buttons_enabled ON recharge_quick_buttons(enabled);
