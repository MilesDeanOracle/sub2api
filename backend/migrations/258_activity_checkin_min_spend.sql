ALTER TABLE activity_config
    ADD COLUMN checkin_min_spend_cents BIGINT NOT NULL DEFAULT 1
    CHECK (checkin_min_spend_cents >= 0);
