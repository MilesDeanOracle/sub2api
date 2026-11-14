package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"

	apperrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/google/uuid"
)

type OperationsPelicanService struct {
	repo        OperationsRepository
	accounts    AccountRepository
	tests       *AccountTestService
	backup      *BackupService
	images      *ImageStorageSettingService
	encryptor   SecretEncryptor
	stores      OperationsObjectStoreFactory
	modelsGate  chan struct{}
	modelsMu    sync.Mutex
	modelsCache map[int64]operationsModelsCacheEntry
	wake        chan struct{}
	cancel      context.CancelFunc
	wg          sync.WaitGroup
	once, stop  sync.Once
}

func ProvideOperationsPelicanService(repo OperationsRepository, accounts AccountRepository, tests *AccountTestService, backup *BackupService, images *ImageStorageSettingService, encryptor SecretEncryptor, stores OperationsObjectStoreFactory) *OperationsPelicanService {
	s := NewOperationsPelicanService(repo, accounts, tests, backup, images, encryptor, stores)
	s.Start()
	return s
}

func NewOperationsPelicanService(repo OperationsRepository, accounts AccountRepository, tests *AccountTestService, backup *BackupService, images *ImageStorageSettingService, encryptor SecretEncryptor, stores OperationsObjectStoreFactory) *OperationsPelicanService {
	return &OperationsPelicanService{repo: repo, accounts: accounts, tests: tests, backup: backup, images: images, encryptor: encryptor, stores: stores, modelsGate: make(chan struct{}, 2), wake: make(chan struct{}, 4)}
}
func operationsBad(message string) error { return apperrors.BadRequest("OPERATIONS_INVALID", message) }
func operationsGroups(a *Account) []OperationsGroup {
	out := []OperationsGroup{}
	seen := map[int64]bool{}
	for _, g := range a.Groups {
		if g != nil && !seen[g.ID] {
			seen[g.ID] = true
			out = append(out, OperationsGroup{ID: g.ID, Name: g.Name})
		}
	}
	return out
}
func (s *OperationsPelicanService) Accounts(ctx context.Context) ([]OperationsAccount, error) {
	all, e := s.accounts.ListActive(ctx)
	if e != nil {
		return nil, e
	}
	out := []OperationsAccount{}
	for i := range all {
		a := &all[i]
		p, _, e := operationsAccountTarget(a, "")
		if e != nil {
			continue
		}
		out = append(out, OperationsAccount{ID: a.ID, Name: a.Name, Platform: a.Platform, Protocol: p, Groups: operationsGroups(a)})
	}
	return out, nil
}

type OperationsModels struct {
	Models    []string                         `json:"models"`
	Source    string                           `json:"source"`
	QueriedAt time.Time                        `json:"queried_at"`
	Metadata  map[string]UpstreamModelMetadata `json:"metadata"`
	Cached    bool                             `json:"cached"`
}

type operationsModelsCacheEntry struct {
	Fingerprint [32]byte
	Value       OperationsModels
}

func operationsModelsFingerprint(a *Account) [32]byte {
	// Hash rather than retain credentials in the cache key. Account edits,
	// protocol changes and proxy changes invalidate the previous live catalog.
	raw, _ := json.Marshal([]any{a.ID, a.Type, a.Platform, a.Credentials, a.Extra, a.UpdatedAt, a.ProxyID, a.Proxy})
	return sha256.Sum256(raw)
}

func (s *OperationsPelicanService) Models(ctx context.Context, id int64, refresh bool) (*OperationsModels, error) {
	a, e := s.accounts.GetByID(ctx, id)
	if e != nil {
		return nil, e
	}
	if _, _, e = operationsAccountTarget(a, ""); e != nil {
		return nil, operationsBad(e.Error())
	}
	fingerprint := operationsModelsFingerprint(a)
	s.modelsMu.Lock()
	cached, exists := s.modelsCache[id]
	s.modelsMu.Unlock()
	if !refresh && exists && cached.Fingerprint == fingerprint && time.Since(cached.Value.QueriedAt) < 5*time.Minute {
		v := cached.Value
		v.Cached = true
		return &v, nil
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	select {
	case s.modelsGate <- struct{}{}:
		defer func() { <-s.modelsGate }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	ids, e := s.operationsDiscoverModels(ctx, a)
	if e != nil {
		return nil, e
	}
	meta := map[string]UpstreamModelMetadata{}
	for _, id := range ids {
		if m, ok := a.GetUpstreamModelMetadata(id); ok {
			meta[id] = m
		}
	}
	v := OperationsModels{Models: ids, Source: "upstream", QueriedAt: time.Now().UTC(), Metadata: meta}
	s.modelsMu.Lock()
	if s.modelsCache == nil || len(s.modelsCache) >= 1000 {
		s.modelsCache = make(map[int64]operationsModelsCacheEntry)
	}
	s.modelsCache[id] = operationsModelsCacheEntry{Fingerprint: fingerprint, Value: v}
	s.modelsMu.Unlock()
	return &v, nil
}
func (s *OperationsPelicanService) storageConfig(ctx context.Context, source string) (*BackupS3Config, error) {
	var c *BackupS3Config
	var e error
	switch source {
	case "backup_s3":
		if s.backup == nil {
			return nil, errors.New("backup storage unavailable")
		}
		c, e = s.backup.loadS3Config(ctx)
	case "image_storage":
		if s.images == nil {
			return nil, errors.New("image storage unavailable")
		}
		v, err := s.images.effectiveConfig(ctx)
		if err != nil {
			return nil, err
		}
		c = &BackupS3Config{Endpoint: v.Endpoint, Region: v.Region, Bucket: v.Bucket, AccessKeyID: v.AccessKeyID, SecretAccessKey: v.SecretAccessKey, ForcePathStyle: v.ForcePathStyle}
	default:
		return nil, errors.New("select an existing object-storage connection")
	}
	if e != nil {
		return nil, e
	}
	if c == nil || !c.IsConfigured() {
		return nil, errors.New("object storage is not configured")
	}
	copy := *c
	copy.Prefix = OperationsPelicanPrefix
	return &copy, nil
}

type OperationsStorageSource struct {
	ID        string `json:"id"`
	Available bool   `json:"available"`
	Endpoint  string `json:"endpoint"`
	Bucket    string `json:"bucket"`
	Prefix    string `json:"prefix"`
	Error     string `json:"error,omitempty"`
}

func (s *OperationsPelicanService) StorageSources(ctx context.Context) []OperationsStorageSource {
	out := []OperationsStorageSource{}
	for _, source := range []string{"backup_s3", "image_storage"} {
		v := OperationsStorageSource{ID: source, Prefix: OperationsPelicanPrefix}
		c, e := s.storageConfig(ctx, source)
		if e != nil {
			v.Error = "Storage connection unavailable"
		} else {
			v.Available = true
			v.Endpoint = c.Endpoint
			v.Bucket = c.Bucket
		}
		out = append(out, v)
	}
	return out
}
func (s *OperationsPelicanService) TestStorage(ctx context.Context, source string) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	c, e := s.storageConfig(ctx, source)
	if e != nil {
		return operationsBad(e.Error())
	}
	store, e := s.stores(ctx, c)
	if e != nil {
		return operationsBad("Cannot initialize object storage")
	}
	if e = store.Test(ctx); e != nil {
		return operationsBad("Object storage write/read/delete verification failed")
	}
	return nil
}
func (s *OperationsPelicanService) prepare(ctx context.Context, t *OperationsTask, validateModels bool) ([]*OperationsResult, error) {
	canBind := s.backup != nil && s.backup.EncryptionKeyConfigured() && s.encryptor != nil
	if !canBind && validateModels {
		return nil, operationsBad("Configure a persistent encryption key before using operations storage")
	}
	c, e := s.storageConfig(ctx, t.StorageSource)
	if e != nil {
		if validateModels {
			return nil, operationsBad(e.Error())
		}
		// An already saved plan still needs durable failure records when its
		// connection is removed. Workers will fail before contacting upstream.
		c = &BackupS3Config{}
	}
	raw, e := json.Marshal(c)
	if e != nil {
		return nil, e
	}
	binding := ""
	if canBind {
		binding, e = s.encryptor.Encrypt(string(raw))
		if e != nil && validateModels {
			return nil, operationsBad("Cannot encrypt storage connection")
		}
	}
	out := []*OperationsResult{}
	seen := map[int64]bool{}
	for _, choice := range t.AccountConfigs {
		if choice.AccountID <= 0 || strings.TrimSpace(choice.ModelID) == "" || seen[choice.AccountID] {
			return nil, operationsBad("Select unique accounts and an upstream model for each")
		}
		seen[choice.AccountID] = true
		a, e := s.accounts.GetByID(ctx, choice.AccountID)
		if e != nil {
			if validateModels {
				return nil, operationsBad(fmt.Sprintf("Account %d is unavailable", choice.AccountID))
			}
			out = append(out, &OperationsResult{AccountID: choice.AccountID, AccountName: fmt.Sprintf("#%d", choice.AccountID), Groups: []OperationsGroup{}, ConfigSnapshot: *t, ModelID: choice.ModelID, ReasoningEffort: t.ReasoningEffort, StorageBinding: binding, StorageSource: t.StorageSource})
			continue
		}
		p, _, e := operationsAccountTarget(a, choice.ModelID)
		if e != nil && validateModels {
			return nil, operationsBad(fmt.Sprintf("%s: %s", a.Name, e.Error()))
		}
		if validateModels {
			catalog, e := s.Models(ctx, a.ID, false)
			if e != nil {
				return nil, operationsBad(fmt.Sprintf("Cannot discover models for account %s", a.Name))
			}
			found := false
			for _, m := range catalog.Models {
				if m == choice.ModelID {
					found = true
					break
				}
			}
			if !found {
				return nil, operationsBad(fmt.Sprintf("Model %s is not in account %s upstream catalog", choice.ModelID, a.Name))
			}
		}
		effective, e := operationsReasoning(a, p, a.GetMappedModel(choice.ModelID), t.ReasoningEffort)
		if e != nil && validateModels {
			return nil, operationsBad(fmt.Sprintf("%s: %s", a.Name, e.Error()))
		}
		out = append(out, &OperationsResult{AccountID: a.ID, AccountName: a.Name, Groups: operationsGroups(a), ConfigSnapshot: *t, Protocol: p, ModelID: choice.ModelID, UpstreamModelID: a.GetMappedModel(choice.ModelID), ReasoningEffort: t.ReasoningEffort, EffectiveReasoning: effective, StorageBinding: binding, StorageSource: t.StorageSource, StorageEndpoint: c.Endpoint, StorageBucket: c.Bucket})
	}
	return out, nil
}
func (s *OperationsPelicanService) SaveTask(ctx context.Context, t *OperationsTask) (*OperationsTask, error) {
	t.Name = strings.TrimSpace(t.Name)
	t.Prompt = strings.TrimSpace(t.Prompt)
	if t.Name == "" || len([]rune(t.Name)) > 200 || t.Prompt == "" || len(t.Prompt) > 64<<10 || t.IntervalHours < 1 || t.IntervalHours > 8760 || len(t.AccountConfigs) == 0 || len(t.AccountConfigs) > 200 {
		return nil, operationsBad("Provide a name, prompt, interval (1–8760 hours), and 1–200 accounts")
	}
	valid := t.ReasoningEffort == ""
	for _, v := range PelicanReasoningLevels {
		valid = valid || t.ReasoningEffort == v
	}
	if !valid {
		return nil, operationsBad("Invalid reasoning effort")
	}
	items, e := s.prepare(ctx, t, true)
	if e != nil {
		return nil, e
	}
	v, e := s.repo.SaveTask(ctx, t, items, "enable:"+uuid.NewString())
	if e == nil {
		s.signal()
	}
	return v, e
}
func (s *OperationsPelicanService) Enable(ctx context.Context, id int64, enabled bool, version, actor int64) (*OperationsTask, error) {
	t, e := s.repo.GetTask(ctx, id)
	if e != nil {
		return nil, e
	}
	if t.ConfigVersion != version {
		return nil, ErrOperationsConflict
	}
	t.Enabled = enabled
	t.UpdatedBy = actor
	var items []*OperationsResult
	if enabled {
		items, e = s.prepare(ctx, t, false)
		if e != nil {
			return nil, e
		}
	}
	out, e := s.repo.SaveTask(ctx, t, items, "enable:"+uuid.NewString())
	if e == nil {
		s.signal()
	}
	return out, e
}
func (s *OperationsPelicanService) Trigger(ctx context.Context, id int64, key string, actor int64, due *time.Time) (*OperationsRound, error) {
	if len(key) < 8 || len(key) > 128 {
		return nil, operationsBad("A request ID of 8–128 characters is required")
	}
	t, e := s.repo.GetTask(ctx, id)
	if e != nil {
		return nil, e
	}
	items, e := s.prepare(ctx, t, false)
	if e != nil {
		return nil, e
	}
	kind := "manual"
	if due != nil {
		kind = "scheduled"
	}
	out, e := s.repo.Enqueue(ctx, id, kind, key, actor, items, due)
	if e == nil {
		s.signal()
	}
	return out, e
}
func (s *OperationsPelicanService) Filters(ctx context.Context) ([]OperationsAccount, error) {
	current, e := s.Accounts(ctx)
	if e != nil {
		return nil, e
	}
	past, e := s.repo.FilterSnapshots(ctx)
	if e != nil {
		return nil, e
	}
	by := map[int64]*OperationsAccount{}
	for i := range current {
		a := current[i]
		a.HasUngrouped = len(a.Groups) == 0
		by[a.ID] = &a
	}
	for _, r := range past {
		a := by[r.AccountID]
		if a == nil {
			a = &OperationsAccount{ID: r.AccountID, Name: r.AccountName, Groups: []OperationsGroup{}}
			by[a.ID] = a
		}
		a.HasUngrouped = a.HasUngrouped || len(r.Groups) == 0
		for _, g := range r.Groups {
			found := false
			for _, old := range a.Groups {
				found = found || old.ID == g.ID
			}
			if !found {
				a.Groups = append(a.Groups, g)
			}
		}
	}
	out := []OperationsAccount{}
	for _, v := range by {
		out = append(out, *v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}
func (s *OperationsPelicanService) Content(ctx context.Context, id int64) (string, error) {
	r, e := s.repo.GetResult(ctx, id)
	if e != nil {
		return "", e
	}
	if !r.ExpiresAt.After(time.Now()) || r.Status != "succeeded" || r.ObjectKey == "" {
		return "", ErrOperationsNotFound
	}
	store, e := s.boundStore(ctx, r)
	if e != nil {
		return "", errors.New("artifact storage unavailable")
	}
	data, e := store.Read(ctx, r.ObjectKey)
	if e != nil {
		return "", errors.New("cannot read artifact")
	}
	sum := sha256.Sum256(data)
	if r.SHA256 != "" && hex.EncodeToString(sum[:]) != r.SHA256 {
		return "", errors.New("artifact checksum mismatch")
	}
	return string(data), nil
}
func (s *OperationsPelicanService) CleanGroup(ctx context.Context, groupID int64) (*OperationsGroupCleanup, error) {
	if groupID == 0 || groupID < -1 {
		return nil, operationsBad("Select a previously detected group")
	}
	batch, err := s.repo.PrepareGroupCleanup(ctx, groupID)
	if err != nil {
		return nil, err
	}
	out := &OperationsGroupCleanup{Detached: batch.Detached}
	for _, item := range batch.Items {
		if err := s.cleanupResult(ctx, item); err != nil {
			out.Failed++
		} else {
			out.Removed++
		}
	}
	return out, nil
}
func (s *OperationsPelicanService) cleanupResult(ctx context.Context, r *OperationsResult) error {
	var err error
	if r.ObjectKey != "" {
		var store OperationsObjectStore
		store, err = s.boundStore(ctx, r)
		if err == nil {
			err = store.Delete(ctx, r.ObjectKey, r.VersionID)
		}
	}
	if err == nil {
		err = s.repo.CompleteCleanup(ctx, r.ID)
	}
	if err != nil {
		save, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if e := s.repo.FailCleanup(save, r.ID, "Artifact deletion failed; verify storage access"); e != nil {
			slog.Warn("operations cleanup failure could not be recorded", "result_id", r.ID, "error", e)
		}
	}
	return err
}
func (s *OperationsPelicanService) boundStore(ctx context.Context, r *OperationsResult) (OperationsObjectStore, error) {
	if r.StorageBinding == "" || s.encryptor == nil {
		return nil, errors.New("storage binding unavailable")
	}
	raw, e := s.encryptor.Decrypt(r.StorageBinding)
	if e != nil {
		return nil, e
	}
	var c BackupS3Config
	if e = json.Unmarshal([]byte(raw), &c); e != nil {
		return nil, e
	}
	if !c.IsConfigured() {
		return nil, errors.New("storage binding is incomplete")
	}
	if current, err := s.storageConfig(ctx, r.StorageSource); err == nil && current.Endpoint == c.Endpoint && current.Bucket == c.Bucket {
		c = *current
	}
	return s.stores(ctx, &c)
}
func (s *OperationsPelicanService) signal() {
	for i := 0; i < 4; i++ {
		select {
		case s.wake <- struct{}{}:
		default:
		}
	}
}
func (s *OperationsPelicanService) Start() {
	s.once.Do(func() {
		ctx, cancel := context.WithCancel(context.Background())
		s.cancel = cancel
		for i := 0; i < 4; i++ {
			s.wg.Add(1)
			go s.worker(ctx)
		}
		s.wg.Add(1)
		go s.scheduler(ctx)
	})
}
func (s *OperationsPelicanService) Stop() {
	if s == nil {
		return
	}
	s.stop.Do(func() {
		if s.cancel != nil {
			s.cancel()
		}
		s.wg.Wait()
	})
}
func (s *OperationsPelicanService) worker(ctx context.Context) {
	defer s.wg.Done()
	poll := time.NewTicker(time.Second)
	defer poll.Stop()
	for {
		if ctx.Err() != nil {
			return
		}
		r, e := s.repo.Claim(ctx)
		if e != nil {
			slog.Warn("operations claim failed", "error", e)
		}
		if r != nil {
			s.execute(ctx, r)
			continue
		}
		select {
		case <-ctx.Done():
			return
		case <-s.wake:
		case <-poll.C:
		}
	}
}
func (s *OperationsPelicanService) scheduler(ctx context.Context) {
	defer s.wg.Done()
	tick := time.NewTicker(10 * time.Second)
	defer tick.Stop()
	clean := time.NewTicker(time.Hour)
	defer clean.Stop()
	s.scan(ctx)
	s.cleanup(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			s.scan(ctx)
		case <-clean.C:
			s.cleanup(ctx)
		}
	}
}
func (s *OperationsPelicanService) scan(ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if e := s.repo.Recover(ctx); e != nil {
		slog.Warn("operations recovery failed", "error", e)
	}
	tasks, e := s.repo.ListTasks(ctx)
	if e != nil {
		slog.Warn("operations schedule scan failed", "error", e)
		return
	}
	for _, t := range tasks {
		if t.Enabled && t.NextRunAt != nil && !t.NextRunAt.After(time.Now()) {
			key := fmt.Sprintf("scheduled:%d:%d", t.ID, t.NextRunAt.Unix())
			if _, e = s.Trigger(ctx, t.ID, key, 0, t.NextRunAt); e != nil {
				slog.Warn("operations scheduled trigger failed", "task_id", t.ID, "error", e)
			}
		}
	}
	s.signal()
}
func (s *OperationsPelicanService) execute(parent context.Context, r *OperationsResult) {
	ctx, cancel := context.WithDeadline(parent, *r.DeadlineAt)
	defer cancel()
	heartDone := make(chan struct{})
	defer close(heartDone)
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-heartDone:
				return
			case <-ctx.Done():
				return
			case <-ticker.C:
				hbCtx, hbCancel := context.WithTimeout(ctx, 5*time.Second)
				ok, e := s.repo.Heartbeat(hbCtx, r.ID, r.WorkerToken)
				hbCancel()
				if e != nil || !ok {
					cancel()
					return
				}
			}
		}
	}()
	fail := func(stage, code, message string) {
		now := time.Now().UTC()
		r.Status = "failed"
		r.ErrorStage = stage
		r.ErrorCode = code
		r.ErrorMessage = message
		r.FinishedAt = &now
		r.ExpiresAt = now.Add(7 * 24 * time.Hour)
		if r.StartedAt != nil {
			ms := time.Since(*r.StartedAt).Milliseconds()
			r.TotalDurationMs = &ms
		}
		save, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		if e := s.repo.UpdateResult(save, r); e != nil {
			slog.Warn("operations failed result update deferred to recovery", "result_id", r.ID, "error", e)
		}
		s.signal()
	}
	a, e := s.accounts.GetByID(ctx, r.AccountID)
	if e != nil {
		fail("account", "ACCOUNT_UNAVAILABLE", "Account is unavailable")
		return
	}
	p, _, e := operationsAccountTarget(a, r.ModelID)
	if e != nil {
		fail("account", "ACCOUNT_INVALID", "Account disabled, expired or missing credentials")
		return
	}
	r.Protocol = p
	r.UpstreamModelID = a.GetMappedModel(r.ModelID)
	r.EffectiveReasoning, e = operationsReasoning(a, p, r.UpstreamModelID, r.ReasoningEffort)
	if e != nil {
		fail("request", "REASONING_UNSUPPORTED", e.Error())
		return
	}
	store, e := s.boundStore(ctx, r)
	if e != nil {
		fail("storage", "STORAGE_UNAVAILABLE", "Cannot initialize artifact storage")
		return
	}
	html, e := s.generate(ctx, a, r)
	if e != nil {
		fail("upstream", "GENERATION_FAILED", e.Error())
		return
	}
	sum := sha256.Sum256([]byte(html))
	r.SHA256 = hex.EncodeToString(sum[:])
	r.SizeBytes = int64(len(html))
	r.ContentType = "text/html; charset=utf-8"
	r.ObjectKey = fmt.Sprintf("%s%s/%d/%s/%d/%d.html", OperationsPelicanPrefix, r.CreatedAt.UTC().Format("2006/01/02"), r.TaskID, r.RunID, r.AccountID, r.ID)
	r.Status = "uploading"
	if e = s.repo.UpdateResult(ctx, r); e != nil {
		fail("database", "LOCATION_SAVE_FAILED", "Could not persist object location")
		return
	}
	obj, e := store.Put(ctx, r.ObjectKey, []byte(html))
	if e != nil {
		// A timed-out PUT may have committed; verify the exact bytes before declaring failure.
		checkCtx, stop := context.WithTimeout(ctx, 15*time.Second)
		data, readErr := store.Read(checkCtx, r.ObjectKey)
		if readErr == nil && string(data) == html {
			obj, e = store.Inspect(checkCtx, r.ObjectKey)
		}
		stop()
	}
	if e != nil {
		fail("upload", "UPLOAD_FAILED", "HTML generated, but artifact upload failed")
		return
	}
	r.VersionID = obj.VersionID
	now := time.Now().UTC()
	r.Status = "succeeded"
	r.FinishedAt = &now
	r.ExpiresAt = now.Add(7 * 24 * time.Hour)
	ms := time.Since(*r.StartedAt).Milliseconds()
	r.TotalDurationMs = &ms
	if e = s.repo.UpdateResult(ctx, r); e != nil {
		slog.Warn("operations upload committed; result awaits recovery", "result_id", r.ID, "error", e)
	}
	s.signal()
}
func (s *OperationsPelicanService) cleanup(ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	for ctx.Err() == nil {
		items, e := s.repo.ClaimCleanup(ctx, 50)
		if e != nil {
			slog.Warn("operations cleanup scan failed", "error", e)
			return
		}
		if len(items) == 0 {
			return
		}
		for _, r := range items {
			_ = s.cleanupResult(ctx, r)
		}
	}
}
