-- Pelican test reports: one-click feedback tickets created from a successful
-- pelican run. They carry no rating and keep the run metadata for admins.
ALTER TABLE feedback_tickets DROP CONSTRAINT IF EXISTS feedback_tickets_category_check;
ALTER TABLE feedback_tickets ADD CONSTRAINT feedback_tickets_category_check
    CHECK (category IN ('usage', 'suggestion', 'cooperation', 'other', 'pelican'));

ALTER TABLE feedback_tickets ALTER COLUMN rating DROP NOT NULL;
ALTER TABLE feedback_tickets DROP CONSTRAINT IF EXISTS feedback_tickets_rating_required;
ALTER TABLE feedback_tickets ADD CONSTRAINT feedback_tickets_rating_required
    CHECK (rating IS NOT NULL OR category = 'pelican');

ALTER TABLE feedback_tickets ADD COLUMN IF NOT EXISTS pelican_run_id VARCHAR(64);
ALTER TABLE feedback_tickets ADD COLUMN IF NOT EXISTS pelican_request_id VARCHAR(200);
ALTER TABLE feedback_tickets ADD COLUMN IF NOT EXISTS pelican_report JSONB;
ALTER TABLE feedback_tickets ADD COLUMN IF NOT EXISTS pelican_source_file VARCHAR(255);

-- The gateway request ID is verified server-side, so it is the dedupe key:
-- the same run can only be reported once.
CREATE UNIQUE INDEX IF NOT EXISTS uq_feedback_pelican_request
    ON feedback_tickets(pelican_request_id) WHERE pelican_request_id IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS uq_feedback_pelican_run
    ON feedback_tickets(user_id, pelican_run_id) WHERE pelican_run_id IS NOT NULL;
