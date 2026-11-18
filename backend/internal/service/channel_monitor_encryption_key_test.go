//go:build unit

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

type monitorEncryptionKeyRepoStub struct {
	ChannelMonitorRepository
	source  *ChannelMonitor
	created []*ChannelMonitor
	updated []*ChannelMonitor
}

func (r *monitorEncryptionKeyRepoStub) GetByID(_ context.Context, id int64) (*ChannelMonitor, error) {
	if r.source == nil || r.source.ID != id {
		return nil, ErrChannelMonitorNotFound
	}
	return r.source, nil
}

func (r *monitorEncryptionKeyRepoStub) Create(_ context.Context, monitor *ChannelMonitor) error {
	cloned := *monitor
	r.created = append(r.created, &cloned)
	return nil
}

func (r *monitorEncryptionKeyRepoStub) Update(_ context.Context, monitor *ChannelMonitor) error {
	cloned := *monitor
	r.updated = append(r.updated, &cloned)
	return nil
}

func (r *monitorEncryptionKeyRepoStub) FindByDuplicateOperationID(_ context.Context, _ string) (*ChannelMonitor, error) {
	return nil, nil
}

type countingChannelMonitorEncryptor struct {
	*duplicateChannelMonitorEncryptor
	encryptCalls int
}

func (e *countingChannelMonitorEncryptor) Encrypt(plaintext string) (string, error) {
	e.encryptCalls++
	return e.duplicateChannelMonitorEncryptor.Encrypt(plaintext)
}

func newProbeMonitorForEncryptionKeyTests() *ChannelMonitor {
	return &ChannelMonitor{
		ID:              42,
		Name:            "primary",
		Provider:        MonitorProviderOpenAI,
		APIMode:         MonitorAPIModeChatCompletions,
		Endpoint:        "https://8.8.8.8",
		APIKey:          "OLD:sk-live",
		PrimaryModel:    "gpt-4o-mini",
		IntervalSeconds: 60,
		Enabled:         true,
		CheckMode:       MonitorCheckModeProbe,
	}
}

func TestCreateChannelMonitorRejectsEphemeralEncryptionKey(t *testing.T) {
	repo := &monitorEncryptionKeyRepoStub{}
	enc := &countingChannelMonitorEncryptor{duplicateChannelMonitorEncryptor: &duplicateChannelMonitorEncryptor{}}
	svc := NewChannelMonitorService(repo, enc)

	_, err := svc.Create(context.Background(), ChannelMonitorCreateParams{
		Name:            "primary",
		Provider:        MonitorProviderOpenAI,
		Endpoint:        "https://8.8.8.8",
		APIKey:          "sk-live",
		PrimaryModel:    "gpt-4o-mini",
		IntervalSeconds: 60,
		Enabled:         true,
	})

	require.ErrorIs(t, err, ErrChannelMonitorEncryptionKeyNotConfigured)
	require.Empty(t, repo.created)
	require.Zero(t, enc.encryptCalls)
}

func TestEnsureEncryptionKeyDurableAllowsEmptyKey(t *testing.T) {
	svc := NewChannelMonitorService(nil, &duplicateChannelMonitorEncryptor{})
	require.NoError(t, svc.ensureEncryptionKeyDurable(""))
	require.NoError(t, svc.ensureEncryptionKeyDurable("   "))
	require.ErrorIs(t, svc.ensureEncryptionKeyDurable("sk-live"), ErrChannelMonitorEncryptionKeyNotConfigured)
}

func TestUpdateChannelMonitorNameOnlyWithEphemeralEncryptionKey(t *testing.T) {
	repo := &monitorEncryptionKeyRepoStub{source: newProbeMonitorForEncryptionKeyTests()}
	svc := NewChannelMonitorService(repo, &duplicateChannelMonitorEncryptor{})

	name := "renamed"
	updated, err := svc.Update(context.Background(), 42, ChannelMonitorUpdateParams{Name: &name})

	require.NoError(t, err)
	require.Equal(t, "renamed", updated.Name)
	require.Len(t, repo.updated, 1)
	require.Equal(t, "OLD:sk-live", repo.updated[0].APIKey, "blank key updates must keep the existing ciphertext")
	require.False(t, updated.APIKeyDecryptFailed)
}

func TestUpdateChannelMonitorRejectsNewKeyWithEphemeralEncryptionKey(t *testing.T) {
	repo := &monitorEncryptionKeyRepoStub{source: newProbeMonitorForEncryptionKeyTests()}
	svc := NewChannelMonitorService(repo, &duplicateChannelMonitorEncryptor{})

	newKey := "sk-new"
	_, err := svc.Update(context.Background(), 42, ChannelMonitorUpdateParams{APIKey: &newKey})

	require.ErrorIs(t, err, ErrChannelMonitorEncryptionKeyNotConfigured)
	require.Empty(t, repo.updated)
}

func TestDuplicateChannelMonitorRejectsEphemeralEncryptionKey(t *testing.T) {
	source := newProbeMonitorForEncryptionKeyTests()
	repo := &monitorEncryptionKeyRepoStub{source: source}
	enc := &countingChannelMonitorEncryptor{duplicateChannelMonitorEncryptor: &duplicateChannelMonitorEncryptor{}}
	svc := NewChannelMonitorService(repo, enc)

	_, err := svc.Duplicate(context.Background(), source.ID, 77, "admin:77", "op-key")

	require.ErrorIs(t, err, ErrChannelMonitorEncryptionKeyNotConfigured)
	require.Empty(t, repo.created)
	require.Zero(t, enc.encryptCalls)
}

func TestDuplicateChannelMonitorAllowsEmptyQuotaKeyWithEphemeralEncryptionKey(t *testing.T) {
	source := newProbeMonitorForEncryptionKeyTests()
	source.CheckMode = MonitorCheckModeQuota
	source.APIKey = "OLD:"
	repo := &monitorEncryptionKeyRepoStub{source: source}
	svc := NewChannelMonitorService(repo, &duplicateChannelMonitorEncryptor{})

	duplicate, err := svc.Duplicate(context.Background(), source.ID, 77, "admin:77", "op-key")

	require.NoError(t, err)
	require.Empty(t, duplicate.APIKey)
	require.Len(t, repo.created, 1)
	require.Equal(t, "NEW:", repo.created[0].APIKey)
}

func TestCreateChannelMonitorWithDurableEncryptionKey(t *testing.T) {
	repo := &monitorEncryptionKeyRepoStub{}
	svc := NewChannelMonitorService(repo, &duplicateChannelMonitorEncryptor{})
	svc.SetEncryptionKeyDurable(true)

	monitor, err := svc.Create(context.Background(), ChannelMonitorCreateParams{
		Name:            "primary",
		Provider:        MonitorProviderOpenAI,
		Endpoint:        "https://8.8.8.8",
		APIKey:          "sk-live",
		PrimaryModel:    "gpt-4o-mini",
		IntervalSeconds: 60,
		Enabled:         true,
	})

	require.NoError(t, err)
	require.Equal(t, "sk-live", monitor.APIKey)
	require.Len(t, repo.created, 1)
	require.Equal(t, "NEW:sk-live", repo.created[0].APIKey)
}

func TestDecryptInPlaceIgnoresUndecryptableQuotaKey(t *testing.T) {
	svc := NewChannelMonitorService(nil, &duplicateChannelMonitorEncryptor{decryptErr: errors.New("wrong encryption key")})

	quota := &ChannelMonitor{ID: 1, CheckMode: MonitorCheckModeQuota, APIKey: "OLD:broken"}
	svc.decryptInPlace(quota)
	require.Empty(t, quota.APIKey)
	require.False(t, quota.APIKeyDecryptFailed)

	probe := &ChannelMonitor{ID: 2, CheckMode: MonitorCheckModeProbe, APIKey: "OLD:broken"}
	svc.decryptInPlace(probe)
	require.Empty(t, probe.APIKey)
	require.True(t, probe.APIKeyDecryptFailed)
}
