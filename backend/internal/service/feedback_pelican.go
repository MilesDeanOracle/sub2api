package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	apperrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	// FeedbackCategoryPelican marks one-click reports from the pelican test page.
	FeedbackCategoryPelican = "pelican"

	// PelicanReportMaxSourceBytes caps the generated HTML stored per report.
	PelicanReportMaxSourceBytes = 2 << 20
	// PelicanReportMaxNoteRunes caps the optional one-line user note.
	PelicanReportMaxNoteRunes   = 500

	pelicanReportDefaultDir   = "/data/tihucezhi"
	pelicanReportDirEnv       = "PELICAN_REPORT_DIR"
	pelicanUsageLookupRetries = 4
	pelicanUsageLookupDelay   = time.Second
)

var (
	ErrPelicanReportDuplicate   = apperrors.Conflict("PELICAN_REPORT_DUPLICATE", "该检测已上报，请勿重复提交")
	ErrPelicanUsageNotFound     = apperrors.NotFound("PELICAN_USAGE_NOT_FOUND", "未找到本次检测的用量记录：仅支持上报通过本站完成的检测，或请稍后重试")
	ErrPelicanReportSourceWrite = apperrors.InternalServer("PELICAN_REPORT_SOURCE_WRITE_FAILED", "源码保存失败，请联系管理员")
	ErrPelicanReportNoSource    = apperrors.NotFound("PELICAN_REPORT_SOURCE_NOT_FOUND", "该工单没有保存源码")

	pelicanRunIDPattern     = regexp.MustCompile(`^[A-Za-z0-9-]{8,64}$`)
	pelicanRequestIDPattern = regexp.MustCompile(`^[A-Za-z0-9._:-]{8,128}$`)
)

// PelicanReportInput is what the browser sends after a successful run.
// Usage, account and group details are never trusted from the client; they are
// looked up from usage_logs by the gateway request ID.
type PelicanReportInput struct {
	RunID           string `json:"run_id"`
	RequestID       string `json:"request_id"`
	Model           string `json:"model"`
	ReasoningEffort string `json:"reasoning_effort"`
	Note            string `json:"note"`
	Source          string `json:"source"`
	DurationMs      *int64 `json:"duration_ms"`
	TTFTMs          *int64 `json:"ttft_ms"`
	PromptVersion   string `json:"prompt_version"`
}

// PelicanUsageInfo is the server-verified record of the gateway request.
type PelicanUsageInfo struct {
	UsageLogID      int64     `json:"usage_log_id"`
	RequestID       string    `json:"request_id"`
	APIKeyID        int64     `json:"api_key_id"`
	APIKeyName      string    `json:"api_key_name"`
	GroupID         *int64    `json:"group_id"`
	GroupName       string    `json:"group_name"`
	GroupPlatform   string    `json:"group_platform"`
	AccountID       int64     `json:"account_id"`
	AccountName     string    `json:"account_name"`
	AccountPlatform string    `json:"account_platform"`
	AccountType     string    `json:"account_type"`
	RequestedModel  string    `json:"requested_model"`
	Model           string    `json:"model"`
	UpstreamModel   string    `json:"upstream_model"`
	InputTokens     int64     `json:"input_tokens"`
	OutputTokens    int64     `json:"output_tokens"`
	CacheReadTokens int64     `json:"cache_read_tokens"`
	TotalCost       float64   `json:"total_cost"`
	DurationMs      *int64    `json:"duration_ms"`
	FirstTokenMs    *int64    `json:"first_token_ms"`
	CreatedAt       time.Time `json:"created_at"`
}

// PelicanReport is persisted as feedback_tickets.pelican_report (admin only).
type PelicanReport struct {
	RunID            string           `json:"run_id"`
	RequestID        string           `json:"request_id"`
	Model            string           `json:"model"`
	ReasoningEffort  string           `json:"reasoning_effort"`
	PromptVersion    string           `json:"prompt_version"`
	Note             string           `json:"note"`
	ClientDurationMs *int64           `json:"client_duration_ms"`
	ClientTTFTMs     *int64           `json:"client_ttft_ms"`
	SourceBytes      int              `json:"source_bytes"`
	SourceTruncated  bool             `json:"source_truncated"`
	HasSource        bool             `json:"has_source"`
	Usage            PelicanUsageInfo `json:"usage"`
	ReportedAt       time.Time        `json:"reported_at"`
}

// PelicanReportSource is returned to admins for replay.
type PelicanReportSource struct {
	Source    string `json:"source"`
	Truncated bool   `json:"truncated"`
}

// PelicanReportRecord is the repository payload for one report ticket.
type PelicanReportRecord struct {
	Title     string
	Content   string
	RunID     string
	RequestID string
	Report    json.RawMessage
}

// PelicanReportRepository is implemented by the feedback repository.
type PelicanReportRepository interface {
	FindPelicanUsage(ctx context.Context, userID int64, requestID string) (*PelicanUsageInfo, error)
	// CreatePelicanReport inserts the ticket and calls writeSource with the new
	// ticket ID inside the same transaction; a failed write rolls back the ticket.
	CreatePelicanReport(ctx context.Context, userID int64, rec PelicanReportRecord, writeSource func(id int64) (string, error)) (int64, error)
	PelicanSourceFile(ctx context.Context, id int64) (string, error)
}

// PelicanReportDir returns the directory used to store reported HTML sources.
func PelicanReportDir() string {
	if dir := strings.TrimSpace(os.Getenv(pelicanReportDirEnv)); dir != "" {
		return dir
	}
	return pelicanReportDefaultDir
}

// CreatePelicanReport validates a one-click report, verifies the run against
// usage_logs and stores it as a feedback ticket with the generated source.
func (s *FeedbackService) CreatePelicanReport(ctx context.Context, userID int64, in PelicanReportInput) (int64, error) {
	repo, ok := s.repo.(PelicanReportRepository)
	if !ok {
		return 0, errors.New("feedback repository does not support pelican reports")
	}
	in.RunID = strings.TrimSpace(in.RunID)
	in.RequestID = strings.TrimSpace(in.RequestID)
	in.Model = strings.TrimSpace(in.Model)
	in.ReasoningEffort = strings.TrimSpace(in.ReasoningEffort)
	in.Note = strings.TrimSpace(in.Note)
	in.PromptVersion = strings.TrimSpace(in.PromptVersion)
	if userID <= 0 || !pelicanRunIDPattern.MatchString(in.RunID) {
		return 0, apperrors.BadRequest("INVALID_PELICAN_REPORT", "检测 ID 无效")
	}
	if !pelicanRequestIDPattern.MatchString(in.RequestID) {
		return 0, apperrors.BadRequest("PELICAN_REPORT_NOT_LOCAL", "仅支持上报通过本站完成的检测")
	}
	if in.Model == "" || len(in.Model) > 200 {
		return 0, apperrors.BadRequest("INVALID_PELICAN_REPORT", "模型名称无效")
	}
	if in.ReasoningEffort != "" && !validPelicanReasoningLevel(in.ReasoningEffort) {
		return 0, apperrors.BadRequest("INVALID_PELICAN_REPORT", "推理级别无效")
	}
	if utf8.RuneCountInString(in.Note) > PelicanReportMaxNoteRunes {
		return 0, apperrors.BadRequest("INVALID_PELICAN_REPORT", fmt.Sprintf("补充说明最多 %d 字", PelicanReportMaxNoteRunes))
	}
	if len(in.PromptVersion) > 100 {
		in.PromptVersion = in.PromptVersion[:100]
	}
	if strings.TrimSpace(in.Source) == "" {
		return 0, apperrors.BadRequest("INVALID_PELICAN_REPORT", "没有可上报的生成结果")
	}

	usage, err := s.lookupPelicanUsage(ctx, repo, userID, in.RequestID)
	if err != nil {
		return 0, err
	}

	source, truncated := truncatePelicanSource(in.Source, PelicanReportMaxSourceBytes)
	report := PelicanReport{
		RunID:            in.RunID,
		RequestID:        usage.RequestID,
		Model:            in.Model,
		ReasoningEffort:  in.ReasoningEffort,
		PromptVersion:    in.PromptVersion,
		Note:             in.Note,
		ClientDurationMs: pelicanNonNegative(in.DurationMs),
		ClientTTFTMs:     pelicanNonNegative(in.TTFTMs),
		SourceBytes:      len(source),
		SourceTruncated:  truncated,
		HasSource:        true,
		Usage:            *usage,
		ReportedAt:       time.Now(),
	}
	raw, err := json.Marshal(report)
	if err != nil {
		return 0, err
	}

	dir := PelicanReportDir()
	rec := PelicanReportRecord{
		Title:     pelicanReportTitle(in.Model),
		Content:   pelicanReportContent(report),
		RunID:     in.RunID,
		RequestID: usage.RequestID,
		Report:    raw,
	}
	id, err := repo.CreatePelicanReport(ctx, userID, rec, func(id int64) (string, error) {
		name := fmt.Sprintf("ticket-%d-%s.html", id, in.RunID)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return "", err
		}
		if err := os.WriteFile(filepath.Join(dir, name), []byte(source), 0o644); err != nil {
			return "", err
		}
		return name, nil
	})
	if err != nil {
		return 0, err
	}
	select {
	case s.wake <- struct{}{}:
	default:
	}
	return id, nil
}

// GetPelicanSource reads the stored HTML source of a report ticket (admin).
func (s *FeedbackService) GetPelicanSource(ctx context.Context, id int64) (*PelicanReportSource, error) {
	repo, ok := s.repo.(PelicanReportRepository)
	if !ok {
		return nil, ErrPelicanReportNoSource
	}
	name, err := repo.PelicanSourceFile(ctx, id)
	if err != nil {
		return nil, err
	}
	// The stored value is a bare file name; Base guards against tampered rows.
	name = filepath.Base(strings.TrimSpace(name))
	if name == "" || name == "." || name == string(filepath.Separator) {
		return nil, ErrPelicanReportNoSource
	}
	data, err := os.ReadFile(filepath.Join(PelicanReportDir(), name))
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrPelicanReportNoSource
	}
	if err != nil {
		return nil, err
	}
	return &PelicanReportSource{Source: string(data), Truncated: len(data) >= PelicanReportMaxSourceBytes}, nil
}

// lookupPelicanUsage retries briefly because usage logs are written right
// after the stream completes and may lag the user's click by a moment.
func (s *FeedbackService) lookupPelicanUsage(ctx context.Context, repo PelicanReportRepository, userID int64, requestID string) (*PelicanUsageInfo, error) {
	for attempt := 0; ; attempt++ {
		usage, err := repo.FindPelicanUsage(ctx, userID, requestID)
		if err == nil {
			return usage, nil
		}
		if !errors.Is(err, ErrPelicanUsageNotFound) || attempt >= pelicanUsageLookupRetries-1 {
			return nil, err
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(pelicanUsageLookupDelay):
		}
	}
}

func validPelicanReasoningLevel(value string) bool {
	for _, level := range PelicanReasoningLevels {
		if level == value {
			return true
		}
	}
	return false
}

func pelicanNonNegative(v *int64) *int64 {
	if v == nil || *v < 0 {
		return nil
	}
	return v
}

// truncatePelicanSource cuts s to at most limit bytes without splitting a rune.
func truncatePelicanSource(s string, limit int) (string, bool) {
	if len(s) <= limit {
		return s, false
	}
	cut := limit
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut], true
}

func pelicanReportTitle(model string) string {
	title := "鹈鹕测智上报 · " + model
	if utf8.RuneCountInString(title) > 120 {
		title = string([]rune(title)[:120])
	}
	return title
}

func pelicanReportContent(r PelicanReport) string {
	effort := r.ReasoningEffort
	if effort == "" {
		effort = "默认（不传）"
	}
	group := r.Usage.GroupName
	if group == "" {
		group = "未分组"
	}
	account := r.Usage.AccountName
	if account == "" {
		account = fmt.Sprintf("#%d", r.Usage.AccountID)
	}
	var b strings.Builder
	b.WriteString("用户从鹈鹕测智页面一键上报了一次检测。\n\n")
	fmt.Fprintf(&b, "检测 ID：%s\n", r.RunID)
	fmt.Fprintf(&b, "请求 ID：%s\n", r.RequestID)
	fmt.Fprintf(&b, "模型：%s\n", r.Model)
	fmt.Fprintf(&b, "思考强度：%s\n", effort)
	fmt.Fprintf(&b, "分组：%s\n", group)
	fmt.Fprintf(&b, "账户：%s（%s）\n", account, r.Usage.AccountPlatform)
	if r.Note != "" {
		fmt.Fprintf(&b, "\n用户说明：%s\n", r.Note)
	}
	content := b.String()
	if utf8.RuneCountInString(content) > 10000 {
		content = string([]rune(content)[:10000])
	}
	return strings.TrimSpace(content)
}
