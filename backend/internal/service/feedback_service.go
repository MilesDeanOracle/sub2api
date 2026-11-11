package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	apperrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
)

var ErrFeedbackNotFound = apperrors.NotFound("FEEDBACK_NOT_FOUND", "Feedback ticket not found")
var ErrFeedbackAwaitingAdmin = apperrors.Conflict("FEEDBACK_AWAITING_ADMIN", "Wait for an administrator reply before following up")

type FeedbackInput struct {
	Title    string `json:"title"`
	Category string `json:"category"`
	Content  string `json:"content"`
	Rating   int    `json:"rating"`
}

type FeedbackTicket struct {
	ID               int64           `json:"id"`
	UserID           int64           `json:"user_id"`
	Title            string          `json:"title"`
	Category         string          `json:"category"`
	Content          string          `json:"content"`
	Rating           int             `json:"rating"`
	Status           string          `json:"status"`
	Snapshot         json.RawMessage `json:"snapshot,omitempty"`
	EnrichmentStatus string          `json:"enrichment_status"`
	ReplyCount       int             `json:"reply_count"`
	UnreadReplyCount int             `json:"unread_reply_count"`
	CreatedAt        time.Time       `json:"created_at"`
	UpdatedAt        time.Time       `json:"updated_at"`
	Replies          []FeedbackReply `json:"replies,omitempty"`
}

type FeedbackReply struct {
	AuthorRole string     `json:"author_role"`
	ID         int64      `json:"id"`
	Content    string     `json:"content"`
	CreatedAt  time.Time  `json:"created_at"`
	ReadAt     *time.Time `json:"read_at"`
}

type FeedbackFilter struct {
	UserID     int64 // Zero is reserved for authenticated admin endpoints.
	Category   string
	Status     string
	Page       int
	PageSize   int
	UnreadOnly bool
}

type FeedbackList struct {
	Items          []FeedbackTicket `json:"items"`
	Total          int64            `json:"total"`
	CategoryCounts map[string]int64 `json:"category_counts"`
	Page           int              `json:"page"`
	PageSize       int              `json:"page_size"`
}

type FeedbackRepository interface {
	Create(context.Context, int64, FeedbackInput) (int64, error)
	List(context.Context, FeedbackFilter) (*FeedbackList, error)
	Get(context.Context, int64, int64) (*FeedbackTicket, error)
	Reply(context.Context, int64, int64, string, bool) error
	ReplyMine(context.Context, int64, int64, string) error
	EnrichNext(context.Context) (bool, error)
	Summary(context.Context, int64) (*FeedbackSummary, error)
	MarkRead(context.Context, int64, int64, int64) error
}

type FeedbackSummary struct {
	PendingCount        int64 `json:"pending_count"`
	UnreadCount         int64 `json:"unread_count"`
	LatestUnreadReplyID int64 `json:"latest_unread_reply_id"`
}

type FeedbackSettings struct {
	// nil uses the localized default; an empty string intentionally hides the notice.
	ContextNotice *string `json:"context_notice"`
}

const feedbackNoticeSettingKey = "feedback_context_notice"

type FeedbackService struct {
	repo     FeedbackRepository
	settings SettingRepository
	wake     chan struct{}
	cancel   context.CancelFunc
	done     chan struct{}
	stopOnce sync.Once
}

func NewFeedbackService(repo FeedbackRepository, settings SettingRepository) *FeedbackService {
	ctx, cancel := context.WithCancel(context.Background())
	s := &FeedbackService{repo: repo, settings: settings, wake: make(chan struct{}, 1), cancel: cancel, done: make(chan struct{})}
	go s.run(ctx)
	return s
}

func ValidFeedbackCategory(category string) bool {
	switch category {
	case "usage", "suggestion", "cooperation", "other":
		return true
	}
	return false
}

func (s *FeedbackService) Create(ctx context.Context, userID int64, in FeedbackInput) (int64, error) {
	in.Title, in.Content = strings.TrimSpace(in.Title), strings.TrimSpace(in.Content)
	if userID <= 0 || in.Title == "" || utf8.RuneCountInString(in.Title) > 120 || in.Content == "" || utf8.RuneCountInString(in.Content) > 10000 || !ValidFeedbackCategory(in.Category) || in.Rating < 1 || in.Rating > 5 {
		return 0, apperrors.BadRequest("INVALID_FEEDBACK", "Please provide a title, category, content and a rating from 1 to 5")
	}
	id, err := s.repo.Create(ctx, userID, in)
	if err == nil {
		select {
		case s.wake <- struct{}{}:
		default:
		}
	}
	return id, err
}

func (s *FeedbackService) List(ctx context.Context, f FeedbackFilter) (*FeedbackList, error) {
	if f.Category != "" && !ValidFeedbackCategory(f.Category) {
		return nil, apperrors.BadRequest("INVALID_CATEGORY", "Invalid feedback category")
	}
	if f.Status != "" && f.Status != "open" && f.Status != "replied" && f.Status != "resolved" {
		return nil, apperrors.BadRequest("INVALID_STATUS", "Invalid feedback status")
	}
	if f.Page < 1 {
		f.Page = 1
	}
	if f.PageSize < 1 || f.PageSize > 100 {
		f.PageSize = 20
	}
	return s.repo.List(ctx, f)
}

func (s *FeedbackService) Get(ctx context.Context, id, userID int64) (*FeedbackTicket, error) {
	return s.repo.Get(ctx, id, userID)
}

func (s *FeedbackService) Reply(ctx context.Context, id, adminID int64, content string, resolved bool) error {
	content = strings.TrimSpace(content)
	if content == "" || utf8.RuneCountInString(content) > 10000 {
		return apperrors.BadRequest("INVALID_REPLY", "Reply must contain 1 to 10000 characters")
	}
	return s.repo.Reply(ctx, id, adminID, content, resolved)
}

func (s *FeedbackService) ReplyMine(ctx context.Context, id, userID int64, content string) error {
	content = strings.TrimSpace(content)
	if id <= 0 || userID <= 0 || content == "" || utf8.RuneCountInString(content) > 10000 {
		return apperrors.BadRequest("INVALID_REPLY", "Reply must contain 1 to 10000 characters")
	}
	return s.repo.ReplyMine(ctx, id, userID, content)
}

func (s *FeedbackService) Stop() { s.stopOnce.Do(s.cancel); <-s.done }

func (s *FeedbackService) Summary(ctx context.Context, userID int64) (*FeedbackSummary, error) {
	return s.repo.Summary(ctx, userID)
}

func (s *FeedbackService) MarkRead(ctx context.Context, id, userID, replyID int64) error {
	if userID <= 0 || replyID <= 0 {
		return apperrors.BadRequest("INVALID_REPLY_ID", "Invalid reply ID")
	}
	return s.repo.MarkRead(ctx, id, userID, replyID)
}

func (s *FeedbackService) GetSettings(ctx context.Context) (*FeedbackSettings, error) {
	value, err := s.settings.GetValue(ctx, feedbackNoticeSettingKey)
	if errors.Is(err, ErrSettingNotFound) {
		return &FeedbackSettings{}, nil
	}
	if err != nil {
		return nil, err
	}
	return &FeedbackSettings{ContextNotice: &value}, nil
}

func (s *FeedbackService) UpdateSettings(ctx context.Context, settings FeedbackSettings) error {
	if settings.ContextNotice == nil || utf8.RuneCountInString(*settings.ContextNotice) > 2000 {
		return apperrors.BadRequest("INVALID_FEEDBACK_NOTICE", "Feedback notice must contain at most 2000 characters")
	}
	return s.settings.Set(ctx, feedbackNoticeSettingKey, strings.TrimSpace(*settings.ContextNotice))
}

func (s *FeedbackService) run(ctx context.Context) {
	defer close(s.done)
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		// Bounded batches avoid an unbounded goroutine per submission. Database row
		// locks let multiple server instances safely share the persistent queue.
		for i := 0; i < 20 && ctx.Err() == nil; i++ {
			jobCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			processed, err := s.repo.EnrichNext(jobCtx)
			cancel()
			if err != nil {
				logger.LegacyPrintf("service.feedback", "feedback enrichment failed: %v", err)
				break
			}
			if !processed {
				break
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-s.wake:
		case <-ticker.C:
		}
	}
}
