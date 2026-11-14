package service

import (
	"context"
	"time"

	apperrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const OperationsPelicanPrefix = "operations/pelican/"
const OperationsPelicanWorkers = 4
const OperationsPelicanMaxHTML = 5 << 20
const OperationsPelicanTimeout = 10 * time.Minute

var ErrOperationsNotFound = apperrors.NotFound("OPERATIONS_NOT_FOUND", "Operations resource not found")
var ErrOperationsConflict = apperrors.Conflict("OPERATIONS_CONFLICT", "Configuration changed or a round is already active")

type OperationsAccountConfig struct {
	AccountID int64  `json:"account_id"`
	ModelID   string `json:"model_id"`
}
type OperationsGroup struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}
type OperationsAccount struct {
	ID           int64             `json:"id"`
	Name         string            `json:"name"`
	Platform     string            `json:"platform"`
	Protocol     string            `json:"protocol"`
	Groups       []OperationsGroup `json:"groups"`
	HasUngrouped bool              `json:"has_ungrouped,omitempty"`
}
type OperationsTask struct {
	ID              int64                     `json:"id"`
	Name            string                    `json:"name"`
	Enabled         bool                      `json:"enabled"`
	IntervalHours   int                       `json:"interval_hours"`
	AccountConfigs  []OperationsAccountConfig `json:"account_configs"`
	ReasoningEffort string                    `json:"reasoning_effort"`
	Prompt          string                    `json:"prompt"`
	StorageSource   string                    `json:"storage_source"`
	ConfigVersion   int64                     `json:"config_version"`
	LastTriggeredAt *time.Time                `json:"last_triggered_at"`
	NextRunAt       *time.Time                `json:"next_run_at"`
	CreatedBy       int64                     `json:"created_by"`
	UpdatedBy       int64                     `json:"updated_by"`
	CreatedAt       time.Time                 `json:"created_at"`
	UpdatedAt       time.Time                 `json:"updated_at"`
	DeletedAt       *time.Time                `json:"deleted_at,omitempty"`
	Progress        map[string]int            `json:"progress,omitempty"`
}
type OperationsMetrics struct {
	FirstReasoningMs   *int64         `json:"first_reasoning_ms"`
	FirstTextMs        *int64         `json:"first_text_ms"`
	LastTextMs         *int64         `json:"last_text_ms"`
	TTFTMs             *int64         `json:"ttft_ms"`
	UpstreamDurationMs *int64         `json:"upstream_duration_ms"`
	TotalDurationMs    *int64         `json:"total_duration_ms"`
	InputTokens        *int64         `json:"input_tokens"`
	OutputTokens       *int64         `json:"output_tokens"`
	ReasoningTokens    *int64         `json:"reasoning_tokens"`
	TPS                *float64       `json:"tps"`
	UsageMetadata      map[string]any `json:"usage_metadata,omitempty"`
}
type OperationsResult struct {
	ID                 int64             `json:"id"`
	TaskID             int64             `json:"task_id"`
	RunID              string            `json:"run_id"`
	AccountID          int64             `json:"account_id"`
	TriggerType        string            `json:"trigger_type"`
	TriggerKey         string            `json:"-"`
	TriggeredBy        int64             `json:"triggered_by"`
	TaskName           string            `json:"task_name"`
	AccountName        string            `json:"account_name"`
	Groups             []OperationsGroup `json:"groups"`
	ConfigSnapshot     OperationsTask    `json:"config_snapshot"`
	Protocol           string            `json:"protocol"`
	ModelID            string            `json:"model_id"`
	UpstreamModelID    string            `json:"upstream_model_id"`
	ReasoningEffort    string            `json:"reasoning_effort"`
	EffectiveReasoning map[string]any    `json:"effective_reasoning,omitempty"`
	Status             string            `json:"status"`
	ActiveRound        bool              `json:"active_round"`
	ErrorStage         string            `json:"error_stage,omitempty"`
	ErrorCode          string            `json:"error_code,omitempty"`
	ErrorMessage       string            `json:"error_message,omitempty"`
	PoolSlot           *int              `json:"-"`
	WorkerToken        string            `json:"-"`
	HeartbeatAt        *time.Time        `json:"-"`
	DeadlineAt         *time.Time        `json:"-"`
	QueuedAt           time.Time         `json:"queued_at"`
	StartedAt          *time.Time        `json:"started_at"`
	RequestStartedAt   *time.Time        `json:"request_started_at"`
	FinishedAt         *time.Time        `json:"finished_at"`
	OperationsMetrics
	StorageSource   string    `json:"storage_source"`
	StorageEndpoint string    `json:"storage_endpoint,omitempty"`
	StorageBucket   string    `json:"storage_bucket,omitempty"`
	ObjectKey       string    `json:"object_key,omitempty"`
	StorageBinding  string    `json:"-"`
	VersionID       string    `json:"-"`
	ContentType     string    `json:"content_type,omitempty"`
	SizeBytes       int64     `json:"size_bytes,omitempty"`
	SHA256          string    `json:"content_sha256,omitempty"`
	ExpiresAt       time.Time `json:"expires_at"`
	CleanupStatus   string    `json:"cleanup_status"`
	CleanupAttempts int       `json:"cleanup_attempts"`
	CleanupError    string    `json:"cleanup_error,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}
type OperationsRound struct {
	RunID    string `json:"run_id"`
	Queued   int    `json:"queued"`
	Existing bool   `json:"existing"`
}
type OperationsResultFilter struct {
	Scope                      string
	History                    bool
	GroupByAccount             bool
	GroupID, AccountID, TaskID int64
	Status                     string
	BeforeID                   int64
	BeforeAccountID            int64
	Limit                      int
	From, To                   *time.Time
}
type OperationsWallGroup struct {
	ID       int64               `json:"id"`
	Name     string              `json:"name"`
	Accounts []OperationsAccount `json:"accounts"`
}
type OperationsWallGroups struct {
	Current []OperationsWallGroup `json:"current"`
	Past    []OperationsWallGroup `json:"past"`
}
type OperationsGroupCleanup struct {
	Removed  int `json:"removed"`
	Detached int `json:"detached"`
	Failed   int `json:"failed"`
}
type OperationsGroupCleanupBatch struct {
	Items    []*OperationsResult
	Detached int
}
type OperationsPage struct {
	Items          []*OperationsResult `json:"items"`
	HasMore        bool                `json:"has_more"`
	NextID         int64               `json:"next_id"`
	NextAccountID  int64               `json:"next_account_id,omitempty"`
	AccountHasMore map[int64]bool      `json:"account_has_more,omitempty"`
}
type OperationsStatus struct {
	Running       int `json:"running"`
	Queued        int `json:"queued"`
	CleanupFailed int `json:"cleanup_failed"`
	MaxWorkers    int `json:"max_workers"`
}

// OperationsRepository is deliberately independent from scheduled connectivity tests.
// Payload JSON contains immutable snapshots/metrics; indexed queue and storage fields
// remain relational so claiming and cleanup never require loading artifact content.
type OperationsRepository interface {
	ListTasks(context.Context) ([]*OperationsTask, error)
	GetTask(context.Context, int64) (*OperationsTask, error)
	SaveTask(context.Context, *OperationsTask, []*OperationsResult, string) (*OperationsTask, error)
	DeleteTask(context.Context, int64) error
	Enqueue(context.Context, int64, string, string, int64, []*OperationsResult, *time.Time) (*OperationsRound, error)
	Claim(context.Context) (*OperationsResult, error)
	Heartbeat(context.Context, int64, string) (bool, error)
	UpdateResult(context.Context, *OperationsResult) error
	GetResult(context.Context, int64) (*OperationsResult, error)
	ListResults(context.Context, OperationsResultFilter) (*OperationsPage, error)
	FilterSnapshots(context.Context) ([]*OperationsResult, error)
	WallGroups(context.Context) (*OperationsWallGroups, error)
	PrepareGroupCleanup(context.Context, int64) (*OperationsGroupCleanupBatch, error)
	RunResults(context.Context, string) ([]*OperationsResult, error)
	Status(context.Context) (*OperationsStatus, error)
	Recover(context.Context) error
	ClaimCleanup(context.Context, int) ([]*OperationsResult, error)
	CompleteCleanup(context.Context, int64) error
	FailCleanup(context.Context, int64, string) error
}
type OperationsStoredObject struct {
	VersionID string
	Size      int64
}
type OperationsObjectStore interface {
	Test(context.Context) error
	Put(context.Context, string, []byte) (*OperationsStoredObject, error)
	Read(context.Context, string) ([]byte, error)
	Inspect(context.Context, string) (*OperationsStoredObject, error)
	Delete(context.Context, string, string) error
}
type OperationsObjectStoreFactory func(context.Context, *BackupS3Config) (OperationsObjectStore, error)
