package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type feedbackRepository struct{ db *sql.DB }

func NewFeedbackRepository(db *sql.DB) service.FeedbackRepository { return &feedbackRepository{db: db} }

func (r *feedbackRepository) Create(ctx context.Context, userID int64, in service.FeedbackInput) (int64, error) {
	var id int64
	err := r.db.QueryRowContext(ctx, `INSERT INTO feedback_tickets (user_id,title,category,content,rating) VALUES ($1,$2,$3,$4,$5) RETURNING id`, userID, in.Title, in.Category, in.Content, in.Rating).Scan(&id)
	return id, err
}

// User endpoints never select the private administrator snapshot.
func feedbackColumns(admin bool) string {
	snapshot := "NULL::jsonb"
	if admin {
		snapshot = "f.snapshot"
	}
	return `f.id,f.user_id,f.title,f.category,f.content,f.rating,f.status,` + snapshot + `,f.enrichment_status,f.created_at,f.updated_at,(SELECT COUNT(*) FROM feedback_replies r WHERE r.ticket_id=f.id),(SELECT COUNT(*) FROM feedback_replies r WHERE r.ticket_id=f.id AND r.author_role='admin' AND r.read_at IS NULL)`
}

type feedbackScanner interface{ Scan(...any) error }

func scanFeedback(row feedbackScanner) (service.FeedbackTicket, error) {
	var f service.FeedbackTicket
	var snapshot []byte
	err := row.Scan(&f.ID, &f.UserID, &f.Title, &f.Category, &f.Content, &f.Rating, &f.Status, &snapshot, &f.EnrichmentStatus, &f.CreatedAt, &f.UpdatedAt, &f.ReplyCount, &f.UnreadReplyCount)
	f.Snapshot = json.RawMessage(snapshot)
	return f, err
}

func (r *feedbackRepository) List(ctx context.Context, f service.FeedbackFilter) (*service.FeedbackList, error) {
	result := &service.FeedbackList{Items: []service.FeedbackTicket{}, CategoryCounts: map[string]int64{}, Page: f.Page, PageSize: f.PageSize}
	// Category counts are grouped on the server and scoped to the same owner/status.
	counts, err := r.db.QueryContext(ctx, `SELECT category,COUNT(*) FROM feedback_tickets f WHERE ($1::bigint=0 OR user_id=$1) AND ($2::text='' OR status=$2) AND (NOT $3::boolean OR EXISTS (SELECT 1 FROM feedback_replies r WHERE r.ticket_id=f.id AND r.author_role='admin' AND r.read_at IS NULL)) GROUP BY category`, f.UserID, f.Status, f.UnreadOnly)
	if err != nil {
		return nil, err
	}
	for counts.Next() {
		var category string
		var count int64
		if err = counts.Scan(&category, &count); err != nil {
			counts.Close()
			return nil, err
		}
		result.CategoryCounts[category] = count
	}
	err = counts.Err()
	counts.Close()
	if err != nil {
		return nil, err
	}
	where := ` WHERE ($1::bigint=0 OR f.user_id=$1) AND ($2::text='' OR f.category=$2) AND ($3::text='' OR f.status=$3) AND (NOT $4::boolean OR EXISTS (SELECT 1 FROM feedback_replies r WHERE r.ticket_id=f.id AND r.author_role='admin' AND r.read_at IS NULL))`
	if err = r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM feedback_tickets f`+where, f.UserID, f.Category, f.Status, f.UnreadOnly).Scan(&result.Total); err != nil {
		return nil, err
	}
	rows, err := r.db.QueryContext(ctx, `SELECT `+feedbackColumns(f.UserID == 0)+` FROM feedback_tickets f`+where+` ORDER BY f.created_at DESC,f.id DESC LIMIT $5 OFFSET $6`, f.UserID, f.Category, f.Status, f.UnreadOnly, f.PageSize, (f.Page-1)*f.PageSize)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		item, err := scanFeedback(rows)
		if err != nil {
			return nil, err
		}
		result.Items = append(result.Items, item)
	}
	return result, rows.Err()
}

func (r *feedbackRepository) Get(ctx context.Context, id, userID int64) (*service.FeedbackTicket, error) {
	f, err := scanFeedback(r.db.QueryRowContext(ctx, `SELECT `+feedbackColumns(userID == 0)+` FROM feedback_tickets f WHERE f.id=$1 AND ($2::bigint=0 OR f.user_id=$2)`, id, userID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrFeedbackNotFound
	}
	if err != nil {
		return nil, err
	}
	rows, err := r.db.QueryContext(ctx, `SELECT id,content,created_at,read_at,author_role FROM feedback_replies WHERE ticket_id=$1 ORDER BY id`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	f.Replies = []service.FeedbackReply{}
	for rows.Next() {
		var reply service.FeedbackReply
		if err := rows.Scan(&reply.ID, &reply.Content, &reply.CreatedAt, &reply.ReadAt, &reply.AuthorRole); err != nil {
			return nil, err
		}
		f.Replies = append(f.Replies, reply)
	}
	return &f, rows.Err()
}

func (r *feedbackRepository) Summary(ctx context.Context, userID int64) (*service.FeedbackSummary, error) {
	result := &service.FeedbackSummary{}
	var err error
	if userID == 0 {
		err = r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM feedback_tickets WHERE status='open'`).Scan(&result.PendingCount)
	} else {
		err = r.db.QueryRowContext(ctx, `SELECT COUNT(DISTINCT f.id),COALESCE(MAX(r.id),0) FROM feedback_tickets f JOIN feedback_replies r ON r.ticket_id=f.id WHERE f.user_id=$1 AND r.author_role='admin' AND r.read_at IS NULL`, userID).Scan(&result.UnreadCount, &result.LatestUnreadReplyID)
	}
	return result, err
}

func (r *feedbackRepository) MarkRead(ctx context.Context, id, userID, replyID int64) error {
	// Validate ownership and the last actually displayed reply in the same statement.
	// Gating the update with this CTE also makes duplicate acknowledgements idempotent.
	var owned bool
	err := r.db.QueryRowContext(ctx, `WITH owned AS (
		SELECT f.id FROM feedback_tickets f WHERE f.id=$1 AND f.user_id=$2
		AND EXISTS (SELECT 1 FROM feedback_replies r WHERE r.ticket_id=f.id AND r.id=$3 AND r.author_role='admin')
	), marked AS (
		UPDATE feedback_replies SET read_at=NOW() WHERE ticket_id IN (SELECT id FROM owned) AND id<=$3 AND author_role='admin' AND read_at IS NULL RETURNING id
	) SELECT EXISTS (SELECT 1 FROM owned)`, id, userID, replyID).Scan(&owned)
	if err != nil {
		return err
	}
	if !owned {
		return service.ErrFeedbackNotFound
	}
	return nil
}

func (r *feedbackRepository) Reply(ctx context.Context, id, adminID int64, content string, resolved bool) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	status := "replied"
	if resolved {
		status = "resolved"
	}
	res, err := tx.ExecContext(ctx, `UPDATE feedback_tickets SET status=$2,updated_at=NOW() WHERE id=$1`, id, status)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return service.ErrFeedbackNotFound
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO feedback_replies (ticket_id,admin_id,content) VALUES ($1,$2,$3)`, id, adminID, content); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *feedbackRepository) ReplyMine(ctx context.Context, id, userID int64, content string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	// Serialize both sides on the ticket row so message order and status agree.
	var hasAdminReply bool
	err = tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM feedback_replies r WHERE r.ticket_id=f.id AND r.author_role='admin') FROM feedback_tickets f WHERE f.id=$1 AND f.user_id=$2 FOR UPDATE OF f`, id, userID).Scan(&hasAdminReply)
	if errors.Is(err, sql.ErrNoRows) {
		return service.ErrFeedbackNotFound
	}
	if err != nil {
		return err
	}
	if !hasAdminReply {
		return service.ErrFeedbackAwaitingAdmin
	}
	if _, err = tx.ExecContext(ctx, `UPDATE feedback_tickets SET status='open',updated_at=NOW() WHERE id=$1`, id); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO feedback_replies (ticket_id,user_id,author_role,content) VALUES ($1,$2,'user',$3)`, id, userID, content); err != nil {
		return err
	}
	return tx.Commit()
}

type feedbackUsageSnapshot struct {
	Model     string `json:"model"`
	GroupID   *int64 `json:"group_id"`
	GroupName string `json:"group_name"`
	Tokens    int64  `json:"tokens"`
}

type feedbackSnapshot struct {
	Username       string                  `json:"username"`
	Email          string                  `json:"email"`
	TotalRecharged float64                 `json:"total_recharged"`
	TotalTokens    int64                   `json:"total_tokens"`
	Usage          []feedbackUsageSnapshot `json:"usage"`
	WindowStart    time.Time               `json:"window_start"`
	WindowEnd      time.Time               `json:"window_end"`
	CollectedAt    time.Time               `json:"collected_at"`
}

func (r *feedbackRepository) EnrichNext(ctx context.Context) (bool, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	var id, userID int64
	var createdAt time.Time
	err = tx.QueryRowContext(ctx, `SELECT id,user_id,created_at FROM feedback_tickets WHERE enrichment_status='pending' AND next_attempt_at<=NOW() ORDER BY next_attempt_at,id LIMIT 1 FOR UPDATE SKIP LOCKED`).Scan(&id, &userID, &createdAt)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	snapshot, err := r.collectSnapshot(ctx, tx, userID, createdAt)
	if err == nil {
		var data []byte
		data, err = json.Marshal(snapshot)
		if err == nil {
			_, err = tx.ExecContext(ctx, `UPDATE feedback_tickets SET snapshot=$2,enrichment_status='ready',enrichment_attempts=enrichment_attempts+1 WHERE id=$1`, id, string(data))
		}
	}
	if err != nil {
		// Rollback releases the row lock; no partial snapshot is ever published.
		_ = tx.Rollback()
		// The request context may have timed out. Give the retry update its own
		// short lifetime so a permanently failing ticket cannot block the queue.
		retryCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
		defer cancel()
		_, retryErr := r.db.ExecContext(retryCtx, `UPDATE feedback_tickets SET enrichment_attempts=enrichment_attempts+1,enrichment_status=CASE WHEN enrichment_attempts>=7 THEN 'failed' ELSE 'pending' END,next_attempt_at=NOW()+INTERVAL '1 minute' WHERE id=$1 AND enrichment_status='pending'`, id)
		return true, errors.Join(fmt.Errorf("enrich feedback %d: %w", id, err), retryErr)
	}
	return true, tx.Commit()
}

func (r *feedbackRepository) collectSnapshot(ctx context.Context, tx *sql.Tx, userID int64, end time.Time) (*feedbackSnapshot, error) {
	s := &feedbackSnapshot{Usage: []feedbackUsageSnapshot{}, WindowStart: end.Add(-7 * 24 * time.Hour), WindowEnd: end, CollectedAt: time.Now()}
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(username,''),email,total_recharged FROM users WHERE id=$1`, userID).Scan(&s.Username, &s.Email, &s.TotalRecharged); err != nil {
		return nil, err
	}
	rows, err := tx.QueryContext(ctx, `SELECT COALESCE(NULLIF(u.requested_model,''),u.model),u.group_id,COALESCE(g.name,''),SUM(u.input_tokens::bigint+u.output_tokens::bigint+u.cache_creation_tokens::bigint+u.cache_read_tokens::bigint)::bigint FROM usage_logs u LEFT JOIN groups g ON g.id=u.group_id WHERE u.user_id=$1 AND u.created_at >= $2 AND u.created_at <= $3 GROUP BY 1,u.group_id,g.name ORDER BY 1,u.group_id`, userID, s.WindowStart, s.WindowEnd)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var usage feedbackUsageSnapshot
		if err = rows.Scan(&usage.Model, &usage.GroupID, &usage.GroupName, &usage.Tokens); err != nil {
			return nil, err
		}
		s.TotalTokens += usage.Tokens
		s.Usage = append(s.Usage, usage)
	}
	return s, rows.Err()
}
