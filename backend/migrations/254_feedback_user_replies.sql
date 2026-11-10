-- Existing replies remain administrator messages; user follow-ups share the thread.
ALTER TABLE feedback_replies ADD COLUMN author_role VARCHAR(10) NOT NULL DEFAULT 'admin';
ALTER TABLE feedback_replies ADD COLUMN user_id BIGINT REFERENCES users(id);
ALTER TABLE feedback_replies ALTER COLUMN admin_id DROP NOT NULL;
ALTER TABLE feedback_replies ADD CONSTRAINT feedback_reply_author_check CHECK (
    (author_role = 'admin' AND admin_id IS NOT NULL AND user_id IS NULL)
    OR (author_role = 'user' AND user_id IS NOT NULL AND admin_id IS NULL)
);
CREATE INDEX idx_feedback_admin_replies_unread ON feedback_replies(ticket_id, id)
    WHERE author_role = 'admin' AND read_at IS NULL;
