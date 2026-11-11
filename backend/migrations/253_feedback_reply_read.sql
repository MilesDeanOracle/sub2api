-- Track each reply independently: reading an old reply must not consume a newer one.
ALTER TABLE feedback_replies ADD COLUMN IF NOT EXISTS read_at TIMESTAMPTZ;
CREATE INDEX IF NOT EXISTS idx_feedback_replies_unread ON feedback_replies(ticket_id, id) WHERE read_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_feedback_open ON feedback_tickets(id) WHERE status = 'open';
