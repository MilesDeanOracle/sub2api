package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

const pelicanUsageQuery = `SELECT u.id,u.request_id,u.api_key_id,COALESCE(k.name,''),u.group_id,COALESCE(g.name,''),COALESCE(g.platform,''),
u.account_id,COALESCE(a.name,''),COALESCE(a.platform,''),COALESCE(a.type,''),
COALESCE(u.requested_model,''),u.model,COALESCE(u.upstream_model,''),
u.input_tokens::bigint,u.output_tokens::bigint,u.cache_read_tokens::bigint,u.total_cost,u.duration_ms,u.first_token_ms,u.created_at
FROM usage_logs u
LEFT JOIN api_keys k ON k.id=u.api_key_id
LEFT JOIN groups g ON g.id=u.group_id
LEFT JOIN accounts a ON a.id=u.account_id
WHERE u.user_id=$1 AND u.request_id IN ($2::text, 'client:' || $2::text)
ORDER BY u.id DESC LIMIT 1`

// FindPelicanUsage resolves a gateway request ID to the usage log owned by the
// user. Requiring the owner makes a forged request ID useless for reading other
// users' routing details, and a missing row means the run did not go through
// this site.
func (r *feedbackRepository) FindPelicanUsage(ctx context.Context, userID int64, requestID string) (*service.PelicanUsageInfo, error) {
	var u service.PelicanUsageInfo
	var groupID, durationMs, firstTokenMs sql.NullInt64
	err := r.db.QueryRowContext(ctx, pelicanUsageQuery, userID, requestID).Scan(
		&u.UsageLogID, &u.RequestID, &u.APIKeyID, &u.APIKeyName, &groupID, &u.GroupName, &u.GroupPlatform,
		&u.AccountID, &u.AccountName, &u.AccountPlatform, &u.AccountType,
		&u.RequestedModel, &u.Model, &u.UpstreamModel,
		&u.InputTokens, &u.OutputTokens, &u.CacheReadTokens, &u.TotalCost, &durationMs, &firstTokenMs, &u.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrPelicanUsageNotFound
	}
	if err != nil {
		return nil, err
	}
	if groupID.Valid {
		u.GroupID = &groupID.Int64
	}
	if durationMs.Valid {
		u.DurationMs = &durationMs.Int64
	}
	if firstTokenMs.Valid {
		u.FirstTokenMs = &firstTokenMs.Int64
	}
	return &u, nil
}

func (r *feedbackRepository) CreatePelicanReport(ctx context.Context, userID int64, rec service.PelicanReportRecord, writeSource func(id int64) (string, error)) (int64, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	var id int64
	err = tx.QueryRowContext(ctx, `INSERT INTO feedback_tickets (user_id,title,category,content,rating,pelican_run_id,pelican_request_id,pelican_report) VALUES ($1,$2,$3,$4,NULL,$5,$6,$7) RETURNING id`,
		userID, rec.Title, service.FeedbackCategoryPelican, rec.Content, rec.RunID, rec.RequestID, string(rec.Report)).Scan(&id)
	if isUniqueViolation(err) {
		return 0, service.ErrPelicanReportDuplicate
	}
	if err != nil {
		return 0, err
	}
	name, err := writeSource(id)
	if err != nil {
		return 0, errors.Join(service.ErrPelicanReportSourceWrite, err)
	}
	if _, err = tx.ExecContext(ctx, `UPDATE feedback_tickets SET pelican_source_file=$2 WHERE id=$1`, id, name); err != nil {
		return 0, err
	}
	return id, tx.Commit()
}

func (r *feedbackRepository) PelicanSourceFile(ctx context.Context, id int64) (string, error) {
	var name sql.NullString
	err := r.db.QueryRowContext(ctx, `SELECT pelican_source_file FROM feedback_tickets WHERE id=$1`, id).Scan(&name)
	if errors.Is(err, sql.ErrNoRows) {
		return "", service.ErrFeedbackNotFound
	}
	if err != nil {
		return "", err
	}
	if !name.Valid || name.String == "" {
		return "", service.ErrPelicanReportNoSource
	}
	return name.String, nil
}
