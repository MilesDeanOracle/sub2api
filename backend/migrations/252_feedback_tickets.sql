-- Feedback is persisted before asynchronous enrichment, so restarts cannot lose jobs.
CREATE TABLE IF NOT EXISTS feedback_tickets (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id),
    title VARCHAR(120) NOT NULL CHECK (char_length(btrim(title)) BETWEEN 1 AND 120),
    category VARCHAR(20) NOT NULL CHECK (category IN ('usage', 'suggestion', 'cooperation', 'other')),
    content TEXT NOT NULL CHECK (char_length(btrim(content)) BETWEEN 1 AND 10000),
    rating SMALLINT NOT NULL CHECK (rating BETWEEN 1 AND 5),
    status VARCHAR(20) NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'replied', 'resolved')),
    snapshot JSONB,
    enrichment_status VARCHAR(20) NOT NULL DEFAULT 'pending' CHECK (enrichment_status IN ('pending', 'ready', 'failed')),
    enrichment_attempts INTEGER NOT NULL DEFAULT 0,
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_feedback_user_created ON feedback_tickets(user_id, created_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_feedback_created ON feedback_tickets(created_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_feedback_category_created ON feedback_tickets(category, created_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_feedback_pending ON feedback_tickets(next_attempt_at, id) WHERE enrichment_status = 'pending';

CREATE TABLE IF NOT EXISTS feedback_replies (
    id BIGSERIAL PRIMARY KEY,
    ticket_id BIGINT NOT NULL REFERENCES feedback_tickets(id) ON DELETE CASCADE,
    admin_id BIGINT NOT NULL REFERENCES users(id),
    content TEXT NOT NULL CHECK (char_length(btrim(content)) BETWEEN 1 AND 10000),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_feedback_replies_ticket ON feedback_replies(ticket_id, id);
