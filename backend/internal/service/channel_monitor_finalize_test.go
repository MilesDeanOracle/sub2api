//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// probeCycleRepoStub 记录 FinalizeProbeCycle 的落库决策（周期收尾）。
type probeCycleRepoStub struct {
	ChannelMonitorRepository
	reference  *ChannelMonitorHistoryRef // FindOperationalReference 返回值
	prevAnom   *ProbeAnomaly             // FindLatestProbeAnomaly 返回值
	lastHist   *ChannelMonitorHistoryEntry
	persisted  []*ProbeCyclePersist
	persistIDs []int64
}

func (r *probeCycleRepoStub) FindOperationalReference(context.Context, int64, string) (*ChannelMonitorHistoryRef, error) {
	return r.reference, nil
}

func (r *probeCycleRepoStub) FindLatestProbeAnomaly(context.Context, int64, string) (*ProbeAnomaly, error) {
	return r.prevAnom, nil
}

func (r *probeCycleRepoStub) FindLatestHistoryEntry(context.Context, int64, string) (*ChannelMonitorHistoryEntry, error) {
	return r.lastHist, nil
}

func (r *probeCycleRepoStub) PersistProbeCycle(_ context.Context, persist *ProbeCyclePersist) ([]int64, error) {
	r.persisted = append(r.persisted, persist)
	ids := make([]int64, 0, len(persist.History))
	for i := range persist.History {
		ids = append(ids, int64(1000+i))
	}
	r.persistIDs = ids
	return ids, nil
}

func newFinalizeService(repo *probeCycleRepoStub) *ChannelMonitorService {
	return NewChannelMonitorService(repo, &duplicateChannelMonitorEncryptor{})
}

// exhaustedCycle 构造 4 次全失败的单模型周期。
func exhaustedCycle(t *testing.T, startedAt time.Time) *ProbeCycle {
	cycle := newProbeCycle(7, ProbeTriggerScheduled, startedAt, []string{"m1"})
	now := startedAt
	for i := 0; i < 4; i++ {
		cycle.RecordRound([]*CheckResult{{
			Model: "m1", Status: MonitorStatusError,
			LatencyMs: latencyPtr(150), PingLatencyMs: latencyPtr(30),
			Message: "upstream HTTP 502", CheckedAt: now,
		}}, now)
		now = now.Add(time.Minute)
	}
	require.True(t, cycle.AllDone())
	return cycle
}

func TestFinalizeProbeCycle_Substituted(t *testing.T) {
	startedAt := time.Now().Add(-5 * time.Minute)
	repo := &probeCycleRepoStub{
		reference: &ChannelMonitorHistoryRef{ID: 55, LatencyMs: latencyPtr(480), PingLatencyMs: latencyPtr(60), CheckedAt: startedAt.Add(-time.Hour)},
	}
	svc := newFinalizeService(repo)
	cycle := exhaustedCycle(t, startedAt)

	results, err := svc.FinalizeProbeCycle(context.Background(), &ChannelMonitor{ID: 7, PrimaryModel: "m1"}, cycle)
	require.NoError(t, err)
	require.Len(t, repo.persisted, 1)

	persist := repo.persisted[0]
	require.Equal(t, int64(7), persist.MonitorID)
	require.Len(t, persist.History, 1)
	require.Len(t, persist.Anomalies, 1)

	row := persist.History[0]
	require.Equal(t, MonitorStatusOperational, row.Status)
	require.Equal(t, startedAt, row.CheckedAt)
	require.Nil(t, row.Quota)

	anomaly := persist.Anomalies[0]
	require.Equal(t, ProbeAnomalyResolutionSubstituted, anomaly.Resolution)
	require.Equal(t, 1, anomaly.ConsecutiveSeq)
	require.Equal(t, 0, anomaly.HistoryRowIndex, "single model → first history row")
	require.Equal(t, 4, len(anomaly.Attempts))
	require.NotNil(t, anomaly.ReferenceHistoryID)
	require.Equal(t, int64(55), *anomaly.ReferenceHistoryID)

	// 返回结果与写入行一致（供 /run 响应）。
	require.Len(t, results, 1)
	require.Equal(t, "m1", results[0].Model)
	require.Equal(t, MonitorStatusOperational, results[0].Status)
}

func TestFinalizeProbeCycle_ConsecutiveSeqComputation(t *testing.T) {
	// 上一条异常记录的 written_history_id 与最新历史行相同 → seq = prev + 1。
	startedAt := time.Now().Add(-5 * time.Minute)
	repo := &probeCycleRepoStub{
		reference: &ChannelMonitorHistoryRef{ID: 56, LatencyMs: latencyPtr(300), PingLatencyMs: latencyPtr(40)},
		prevAnom:  &ProbeAnomaly{WrittenHistoryID: 900, ConsecutiveSeq: 2},
		lastHist:  &ChannelMonitorHistoryEntry{ID: 900},
	}
	svc := newFinalizeService(repo)

	_, err := svc.FinalizeProbeCycle(context.Background(), &ChannelMonitor{ID: 7, PrimaryModel: "m1"}, exhaustedCycle(t, startedAt))
	require.NoError(t, err)
	require.Equal(t, 3, repo.persisted[0].Anomalies[0].ConsecutiveSeq)
	require.Equal(t, ProbeAnomalyResolutionSubstituted, repo.persisted[0].Anomalies[0].Resolution,
		"seq=3 is still within the substitution cap")

	// 中间有真实结果（最新历史行 id ≠ prev.written_history_id）→ seq 归 1。
	repo2 := &probeCycleRepoStub{
		reference: &ChannelMonitorHistoryRef{ID: 57, LatencyMs: latencyPtr(300), PingLatencyMs: latencyPtr(40)},
		prevAnom:  &ProbeAnomaly{WrittenHistoryID: 900, ConsecutiveSeq: 5},
		lastHist:  &ChannelMonitorHistoryEntry{ID: 1001},
	}
	svc2 := newFinalizeService(repo2)
	_, err = svc2.FinalizeProbeCycle(context.Background(), &ChannelMonitor{ID: 7, PrimaryModel: "m1"}, exhaustedCycle(t, startedAt))
	require.NoError(t, err)
	require.Equal(t, 1, repo2.persisted[0].Anomalies[0].ConsecutiveSeq)
	require.Equal(t, ProbeAnomalyResolutionSubstituted, repo2.persisted[0].Anomalies[0].Resolution)
}

func TestFinalizeProbeCycle_TooManyConsecutiveWritesRealFailure(t *testing.T) {
	startedAt := time.Now().Add(-5 * time.Minute)
	repo := &probeCycleRepoStub{
		reference: &ChannelMonitorHistoryRef{ID: 58, LatencyMs: latencyPtr(300), PingLatencyMs: latencyPtr(40)},
		prevAnom:  &ProbeAnomaly{WrittenHistoryID: 900, ConsecutiveSeq: monitorSubstituteMaxConsecutive},
		lastHist:  &ChannelMonitorHistoryEntry{ID: 900},
	}
	svc := newFinalizeService(repo)

	_, err := svc.FinalizeProbeCycle(context.Background(), &ChannelMonitor{ID: 7, PrimaryModel: "m1"}, exhaustedCycle(t, startedAt))
	require.NoError(t, err)
	anomaly := repo.persisted[0].Anomalies[0]
	require.Equal(t, ProbeAnomalyResolutionRealWritten, anomaly.Resolution)
	require.Equal(t, ProbeAnomalyReasonTooManyConsecutive, anomaly.Reason)
	require.Equal(t, monitorSubstituteMaxConsecutive+1, anomaly.ConsecutiveSeq)
	require.Equal(t, MonitorStatusError, repo.persisted[0].History[0].Status, "last real failure is written")
}

func TestFinalizeProbeCycle_NoReference(t *testing.T) {
	startedAt := time.Now().Add(-5 * time.Minute)
	repo := &probeCycleRepoStub{}
	svc := newFinalizeService(repo)

	_, err := svc.FinalizeProbeCycle(context.Background(), &ChannelMonitor{ID: 7, PrimaryModel: "m1"}, exhaustedCycle(t, startedAt))
	require.NoError(t, err)
	anomaly := repo.persisted[0].Anomalies[0]
	require.Equal(t, ProbeAnomalyResolutionRealWritten, anomaly.Resolution)
	require.Equal(t, ProbeAnomalyReasonNoReference, anomaly.Reason)
	require.Equal(t, 1, anomaly.ConsecutiveSeq, "no-reference cycles also count toward the streak")
	require.Nil(t, anomaly.ReferenceHistoryID)
}

func TestFinalizeProbeCycle_QuotaProbeAttachesSnapshotToPrimary(t *testing.T) {
	// 多模型 + quota_probe：配额快照只挂主模型行；失败模型照常走复制。
	startedAt := time.Now().Add(-5 * time.Minute)
	repo := &probeCycleRepoStub{
		reference: &ChannelMonitorHistoryRef{ID: 60, LatencyMs: latencyPtr(400), PingLatencyMs: latencyPtr(50)},
	}
	svc := newFinalizeService(repo)
	svc.SetQuotaFetcher(newQuotaModeFetcher(map[int64]*Account{}, nil))
	svc.SetRuntimeReader(channelMonitorRuntimeStub{rt: ChannelMonitorRuntime{Enabled: true, Mode: ChannelMonitorModeV1}})

	cycle := newProbeCycle(7, ProbeTriggerScheduled, startedAt, []string{"primary", "extra"})
	now := startedAt
	// primary：失败 ×4；extra：首轮 operational。
	for i := 0; i < 4; i++ {
		cycle.RecordRound([]*CheckResult{
			{Model: "primary", Status: MonitorStatusError, LatencyMs: latencyPtr(100), PingLatencyMs: latencyPtr(20), Message: "boom", CheckedAt: now},
			{Model: "extra", Status: MonitorStatusOperational, LatencyMs: latencyPtr(80), PingLatencyMs: latencyPtr(20), CheckedAt: now},
		}, now)
		now = now.Add(time.Minute)
	}
	require.True(t, cycle.AllDone())

	m := &ChannelMonitor{ID: 7, PrimaryModel: "primary", CheckMode: MonitorCheckModeQuotaProbe}
	_, err := svc.FinalizeProbeCycle(context.Background(), m, cycle)
	require.NoError(t, err)

	persist := repo.persisted[0]
	require.Len(t, persist.History, 2)
	byModel := map[string]*ChannelMonitorHistoryRow{}
	for _, h := range persist.History {
		byModel[h.Model] = h
	}
	// 未关联账号 → 错误快照仍挂主模型行（与 RunCheck quota_probe 行为一致）。
	require.NotNil(t, byModel["primary"].Quota, "quota snapshot attaches to the primary row")
	require.Nil(t, byModel["extra"].Quota)
	// primary 走复制，异常记录挂在第 0 行。
	require.Len(t, persist.Anomalies, 1)
	require.Equal(t, "primary", persist.Anomalies[0].Model)
	require.Equal(t, 0, persist.Anomalies[0].HistoryRowIndex)
}

func TestFinalizeProbeCycle_RealResultNoAnomaly(t *testing.T) {
	startedAt := time.Now().Add(-5 * time.Minute)
	repo := &probeCycleRepoStub{}
	svc := newFinalizeService(repo)

	cycle := newProbeCycle(7, ProbeTriggerScheduled, startedAt, []string{"m1"})
	cycle.RecordRound([]*CheckResult{{
		Model: "m1", Status: MonitorStatusOperational, LatencyMs: latencyPtr(90), PingLatencyMs: latencyPtr(15), CheckedAt: startedAt,
	}}, startedAt)

	results, err := svc.FinalizeProbeCycle(context.Background(), &ChannelMonitor{ID: 7, PrimaryModel: "m1"}, cycle)
	require.NoError(t, err)
	require.Len(t, repo.persisted, 1)
	require.Len(t, repo.persisted[0].History, 1)
	require.Empty(t, repo.persisted[0].Anomalies)
	require.Equal(t, MonitorStatusOperational, results[0].Status)
}
