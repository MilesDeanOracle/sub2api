//go:build unit

package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/stretchr/testify/require"
)

// probeCycleTestClock 用固定时刻驱动 RecordRound，避免依赖真实时间。
type probeCycleTestClock struct{ now time.Time }

func (c *probeCycleTestClock) advance(d time.Duration) { c.now = c.now.Add(d) }

// record 便捷封装：按当前时钟时刻记录一轮。
func (c *probeCycleTestClock) record(cycle *ProbeCycle, results ...*CheckResult) {
	cycle.RecordRound(results, c.now)
}

func resultAt(model, status string, latencyMs *int, checkedAt time.Time) *CheckResult {
	return &CheckResult{Model: model, Status: status, LatencyMs: latencyMs, CheckedAt: checkedAt}
}

func latencyPtr(v int) *int { return &v }

// --- 周期状态推进 ---

func TestProbeCycle_FirstOperationalNoRetry(t *testing.T) {
	cycle := newProbeCycle(1, ProbeTriggerScheduled, time.Now(), []string{"m1"})
	clock := &probeCycleTestClock{now: time.Now()}
	clock.record(cycle, resultAt("m1", MonitorStatusOperational, latencyPtr(800), clock.now))

	require.True(t, cycle.AllDone(), "first operational should finish the cycle")
	require.Len(t, cycle.Models["m1"].Attempts, 1)
	require.Equal(t, "", cycle.Models["m1"].Category, "operational first try needs no retry category")

	// 决策：原样写结果，无异常记录，也不触发参考行查询。
	plans, err := planProbeCycle(cycle, 1, clock.now, func(string) (*ChannelMonitorHistoryRef, int, error) {
		t.Fatal("lookup must not be called when a real result exists")
		return nil, 0, nil
	})
	require.NoError(t, err)
	require.Len(t, plans, 1)
	require.Equal(t, MonitorStatusOperational, plans[0].Row.Status)
	require.Nil(t, plans[0].Anomaly)
}

func TestProbeCycle_FailureThenOperationalWritesSecondResult(t *testing.T) {
	cycle := newProbeCycle(1, ProbeTriggerScheduled, time.Now(), []string{"m1"})
	clock := &probeCycleTestClock{now: time.Now()}

	clock.record(cycle, resultAt("m1", MonitorStatusError, latencyPtr(45001), clock.now))
	require.False(t, cycle.AllDone(), "failure should trigger retries")
	delay, ok := cycle.NextWakeDelay(clock.now)
	require.True(t, ok)
	require.Equal(t, 3*time.Second, delay, "first failure backoff should be 3s")

	// 重试 1（+3s）拿到 operational：立即停止。
	clock.advance(3 * time.Second)
	second := resultAt("m1", MonitorStatusOperational, latencyPtr(900), clock.now)
	clock.record(cycle, second)
	require.True(t, cycle.AllDone())
	require.Len(t, cycle.Models["m1"].Attempts, 2)

	plans, err := planProbeCycle(cycle, 1, clock.now, func(string) (*ChannelMonitorHistoryRef, int, error) {
		t.Fatal("lookup must not be called when retry recovered")
		return nil, 0, nil
	})
	require.NoError(t, err)
	require.Same(t, second, selectModelResult(cycle.Models["m1"]))
	require.Nil(t, plans[0].Anomaly, "recovered cycle must not write an anomaly")
}

func TestProbeCycle_FailureBackoffScheduleAndBudget(t *testing.T) {
	cycle := newProbeCycle(1, ProbeTriggerScheduled, time.Now(), []string{"m1"})
	clock := &probeCycleTestClock{now: time.Now()}

	// 4 次全部失败：退避依次 3s / 10s / 30s，第 4 次后预算耗尽。
	backoffs := []time.Duration{3 * time.Second, 10 * time.Second, 30 * time.Second}
	for i, backoff := range backoffs {
		clock.record(cycle, resultAt("m1", MonitorStatusError, latencyPtr(100), clock.now))
		require.False(t, cycle.AllDone(), "attempt %d should still allow retries", i+1)
		delay, ok := cycle.NextWakeDelay(clock.now)
		require.True(t, ok)
		require.Equal(t, backoff, delay, "backoff after attempt %d", i+1)
		clock.advance(backoff)
	}
	clock.record(cycle, resultAt("m1", MonitorStatusFailed, latencyPtr(200), clock.now))
	require.True(t, cycle.AllDone(), "4 attempts should exhaust the failure budget")
	require.Len(t, cycle.Models["m1"].Attempts, 4)
}

func TestProbeCycle_DegradedBudgetIsThreeAttempts(t *testing.T) {
	cycle := newProbeCycle(1, ProbeTriggerScheduled, time.Now(), []string{"m1"})
	clock := &probeCycleTestClock{now: time.Now()}

	// 降级策略：首次 + 2 次重试（15s / 30s），共 3 次。
	clock.record(cycle, resultAt("m1", MonitorStatusDegraded, latencyPtr(9000), clock.now))
	delay, _ := cycle.NextWakeDelay(clock.now)
	require.Equal(t, 15*time.Second, delay)

	clock.advance(15 * time.Second)
	clock.record(cycle, resultAt("m1", MonitorStatusDegraded, latencyPtr(8000), clock.now))
	delay, _ = cycle.NextWakeDelay(clock.now)
	require.Equal(t, 30*time.Second, delay)

	clock.advance(30 * time.Second)
	clock.record(cycle, resultAt("m1", MonitorStatusDegraded, latencyPtr(7000), clock.now))
	require.True(t, cycle.AllDone(), "3 attempts should exhaust the degraded budget")
	require.Len(t, cycle.Models["m1"].Attempts, 3)

	plans, err := planProbeCycle(cycle, 1, clock.now, func(string) (*ChannelMonitorHistoryRef, int, error) {
		t.Fatal("degraded cycles never consult reference rows")
		return nil, 0, nil
	})
	require.NoError(t, err)
	// 3 次都不是 operational：写延迟最低的一次 degraded（7000ms）。
	require.Equal(t, MonitorStatusDegraded, plans[0].Row.Status)
	require.Equal(t, 7000, *plans[0].Row.LatencyMs)
	require.Nil(t, plans[0].Anomaly, "degraded cycles must not write anomalies")
}

func TestProbeCycle_DegradedFirstThenFailuresKeepBestDegraded(t *testing.T) {
	cycle := newProbeCycle(1, ProbeTriggerScheduled, time.Now(), []string{"m1"})
	clock := &probeCycleTestClock{now: time.Now()}

	clock.record(cycle, resultAt("m1", MonitorStatusDegraded, latencyPtr(9000), clock.now))
	clock.advance(15 * time.Second)
	clock.record(cycle, resultAt("m1", MonitorStatusError, latencyPtr(300), clock.now))
	clock.advance(30 * time.Second)
	clock.record(cycle, resultAt("m1", MonitorStatusDegraded, latencyPtr(7500), clock.now))
	require.True(t, cycle.AllDone())

	// 重试中出现失败不影响结论：仍写最佳 degraded，无异常记录。
	plans, err := planProbeCycle(cycle, 1, clock.now, func(string) (*ChannelMonitorHistoryRef, int, error) {
		t.Fatal("no anomaly expected for degraded-first cycles")
		return nil, 0, nil
	})
	require.NoError(t, err)
	require.Equal(t, MonitorStatusDegraded, plans[0].Row.Status)
	require.Equal(t, 7500, *plans[0].Row.LatencyMs)
	require.Nil(t, plans[0].Anomaly)
}

func TestProbeCycle_FailureFirstWithMidDegradedPicksLowestLatency(t *testing.T) {
	cycle := newProbeCycle(1, ProbeTriggerScheduled, time.Now(), []string{"m1"})
	clock := &probeCycleTestClock{now: time.Now()}

	clock.record(cycle, resultAt("m1", MonitorStatusError, latencyPtr(200), clock.now))
	clock.advance(3 * time.Second)
	clock.record(cycle, resultAt("m1", MonitorStatusDegraded, latencyPtr(9500), clock.now))
	clock.advance(10 * time.Second)
	clock.record(cycle, resultAt("m1", MonitorStatusError, latencyPtr(210), clock.now))
	clock.advance(30 * time.Second)
	clock.record(cycle, resultAt("m1", MonitorStatusFailed, latencyPtr(220), clock.now))
	require.True(t, cycle.AllDone())

	// 失败首发、中途 degraded：最终写 degraded（延迟最低），无异常记录。
	plans, err := planProbeCycle(cycle, 1, clock.now, func(string) (*ChannelMonitorHistoryRef, int, error) {
		t.Fatal("degraded result exists, no anomaly expected")
		return nil, 0, nil
	})
	require.NoError(t, err)
	require.Equal(t, MonitorStatusDegraded, plans[0].Row.Status)
	require.Equal(t, 9500, *plans[0].Row.LatencyMs)
	require.Nil(t, plans[0].Anomaly)
}

func TestProbeCycle_MultipleModelsIndependentBudgets(t *testing.T) {
	cycle := newProbeCycle(1, ProbeTriggerScheduled, time.Now(), []string{"primary", "extra-a", "extra-b"})
	clock := &probeCycleTestClock{now: time.Now()}
	require.Equal(t, []string{"primary", "extra-a", "extra-b"}, cycle.ModelNames())

	// 第 1 轮：primary 失败、extra-a 降级、extra-b 正常。
	clock.record(cycle,
		resultAt("primary", MonitorStatusError, latencyPtr(100), clock.now),
		resultAt("extra-a", MonitorStatusDegraded, latencyPtr(9000), clock.now),
		resultAt("extra-b", MonitorStatusOperational, latencyPtr(500), clock.now),
	)
	require.True(t, cycle.Models["extra-b"].Done)
	require.False(t, cycle.Models["primary"].Done)
	require.False(t, cycle.Models["extra-a"].Done)

	// +3s：只有 primary 到点（extra-a 的降级退避是 15s）。
	require.Equal(t, []string{"primary"}, cycle.DueModels(clock.now.Add(3*time.Second)))
	// +15s：两个都到点。
	clock.advance(15 * time.Second)
	require.ElementsMatch(t, []string{"primary", "extra-a"}, cycle.DueModels(clock.now))

	// 第 2 轮：primary 恢复 operational；extra-a 仍降级。
	clock.record(cycle,
		resultAt("primary", MonitorStatusOperational, latencyPtr(400), clock.now),
		resultAt("extra-a", MonitorStatusDegraded, latencyPtr(7800), clock.now),
	)
	require.True(t, cycle.Models["primary"].Done)
	require.False(t, cycle.Models["extra-a"].Done, "degraded budget is 3, extra-a needs one more attempt")

	// 第 3 轮：extra-a 预算耗尽。
	clock.advance(30 * time.Second)
	clock.record(cycle, resultAt("extra-a", MonitorStatusDegraded, latencyPtr(7600), clock.now))
	require.True(t, cycle.AllDone())

	plans, err := planProbeCycle(cycle, 1, clock.now, func(string) (*ChannelMonitorHistoryRef, int, error) {
		t.Fatal("no failure-category model exhausted, no lookup expected")
		return nil, 0, nil
	})
	require.NoError(t, err)
	require.Len(t, plans, 3)
	byModel := map[string]*probeModelPlan{}
	for _, p := range plans {
		byModel[p.Model] = p
	}
	require.Equal(t, MonitorStatusOperational, byModel["primary"].Row.Status)
	require.Equal(t, 7600, *byModel["extra-a"].Row.LatencyMs)
	require.Equal(t, MonitorStatusOperational, byModel["extra-b"].Row.Status)
	for _, p := range plans {
		require.Nil(t, p.Anomaly)
	}
}

// --- 复制决策（§3.4 / §3.5） ---

// exhaustedFailureCycle 构造一个失败策略落空（4 次全失败）的单模型周期。
func exhaustedFailureCycle(t *testing.T, model string) (*ProbeCycle, *probeCycleTestClock) {
	cycle := newProbeCycle(7, ProbeTriggerScheduled, time.Now(), []string{model})
	clock := &probeCycleTestClock{now: time.Now()}
	for _, backoff := range monitorFailureRetryBackoffs {
		clock.record(cycle, resultAt(model, MonitorStatusError, latencyPtr(120), clock.now))
		clock.advance(backoff)
	}
	clock.record(cycle, resultAt(model, MonitorStatusFailed, latencyPtr(130), clock.now))
	require.True(t, cycle.AllDone())
	return cycle, clock
}

func TestProbeCycle_SubstitutedRowShape(t *testing.T) {
	cycle, clock := exhaustedFailureCycle(t, "m1")
	startedAt := cycle.StartedAt
	ref := &ChannelMonitorHistoryRef{ID: 42, LatencyMs: latencyPtr(500), PingLatencyMs: latencyPtr(120), CheckedAt: startedAt.Add(-time.Minute)}

	plans, err := planProbeCycle(cycle, 7, clock.now, func(model string) (*ChannelMonitorHistoryRef, int, error) {
		require.Equal(t, "m1", model)
		return ref, 1, nil
	})
	require.NoError(t, err)
	require.Len(t, plans, 1)
	plan := plans[0]
	require.NotNil(t, plan.Anomaly)

	// 复制行字段口径（§3.4）。
	row := plan.Row
	require.Equal(t, MonitorStatusOperational, row.Status)
	require.Equal(t, "", row.Message)
	require.Equal(t, startedAt, row.CheckedAt, "substituted row must use cycle start time")
	require.Equal(t, 120, *row.PingLatencyMs, "ping latency is copied verbatim")
	require.InDelta(t, 500-30, *row.LatencyMs, 20.5, "latency = ref - [10,50]")
	require.GreaterOrEqual(t, *row.LatencyMs, 1)

	// 异常记录：substituted，seq 与参考行信息。
	a := plan.Anomaly
	require.Equal(t, ProbeAnomalyResolutionSubstituted, a.Resolution)
	require.Equal(t, "", a.Reason)
	require.Equal(t, 1, a.ConsecutiveSeq)
	require.Equal(t, MonitorStatusError, a.TriggerStatus, "trigger status is the first failure")
	require.Len(t, a.Attempts, 4)
	require.Equal(t, 1, a.Attempts[0].Seq)
	require.Equal(t, 4, a.Attempts[3].Seq)
	require.NotNil(t, a.ReferenceHistoryID)
	require.Equal(t, int64(42), *a.ReferenceHistoryID)
	require.Equal(t, ProbeTriggerScheduled, a.Trigger)
}

func TestProbeCycle_SubstitutedLatencyClampAndNull(t *testing.T) {
	// 参考行为 NULL：保持 NULL。
	require.Nil(t, substituteLatency(nil))
	// 小参考值：扣减后最小为 1。
	for i := 0; i < 200; i++ {
		v := substituteLatency(latencyPtr(5))
		require.NotNil(t, v)
		require.Equal(t, 1, *v)
	}
	// 边界：ref=60 时结果 ∈ [10, 50]。
	for i := 0; i < 200; i++ {
		v := substituteLatency(latencyPtr(60))
		require.GreaterOrEqual(t, *v, 10)
		require.LessOrEqual(t, *v, 50)
	}
}

func TestProbeCycle_SubstitutedRowWithNullLatencyReference(t *testing.T) {
	cycle, clock := exhaustedFailureCycle(t, "m1")
	ref := &ChannelMonitorHistoryRef{ID: 43, CheckedAt: cycle.StartedAt.Add(-time.Minute)} // Latency/Ping 均为 NULL

	plans, err := planProbeCycle(cycle, 7, clock.now, func(string) (*ChannelMonitorHistoryRef, int, error) {
		return ref, 2, nil
	})
	require.NoError(t, err)
	require.Nil(t, plans[0].Row.LatencyMs, "NULL reference latency stays NULL")
	require.Nil(t, plans[0].Row.PingLatencyMs)
	require.Equal(t, ProbeAnomalyResolutionSubstituted, plans[0].Anomaly.Resolution)
}

// 参考行本身可以是上一周期的复制行（也是 operational），照常复制。
func TestProbeCycle_ChainedSubstitutionAllowed(t *testing.T) {
	for seq := 1; seq <= monitorSubstituteMaxConsecutive; seq++ {
		cycle, clock := exhaustedFailureCycle(t, "m1")
		// 参考行 ID 标记为复制行也不影响：只看 status='operational'。
		ref := &ChannelMonitorHistoryRef{ID: int64(100 + seq), LatencyMs: latencyPtr(400), PingLatencyMs: latencyPtr(90), CheckedAt: cycle.StartedAt}
		plans, err := planProbeCycle(cycle, 7, clock.now, func(string) (*ChannelMonitorHistoryRef, int, error) {
			return ref, seq, nil
		})
		require.NoError(t, err)
		require.Equal(t, ProbeAnomalyResolutionSubstituted, plans[0].Anomaly.Resolution,
			"copying from a substituted reference row is allowed (seq=%d)", seq)
	}
}

func TestProbeCycle_NoReferenceWritesRealFailure(t *testing.T) {
	cycle, clock := exhaustedFailureCycle(t, "m1")
	plans, err := planProbeCycle(cycle, 7, clock.now, func(string) (*ChannelMonitorHistoryRef, int, error) {
		return nil, 1, nil
	})
	require.NoError(t, err)
	plan := plans[0]
	require.Equal(t, ProbeAnomalyResolutionRealWritten, plan.Anomaly.Resolution)
	require.Equal(t, ProbeAnomalyReasonNoReference, plan.Anomaly.Reason)
	// 写最后一次真实失败结果。
	require.Equal(t, MonitorStatusFailed, plan.Row.Status)
	require.Equal(t, 130, *plan.Row.LatencyMs)
	require.Nil(t, plan.Anomaly.ReferenceHistoryID)
}

func TestProbeCycle_ConsecutiveGuardAndRecovery(t *testing.T) {
	// 模拟 §3.5 的周期序列：seq 由 lookup 注入（service 用异常表+历史表计算）。
	type cycleCase struct {
		seq        int
		hasRef     bool
		wantRes    string
		wantReason string
	}
	cases := []cycleCase{
		{seq: 1, hasRef: true, wantRes: ProbeAnomalyResolutionSubstituted},
		{seq: 2, hasRef: true, wantRes: ProbeAnomalyResolutionSubstituted},
		{seq: 3, hasRef: true, wantRes: ProbeAnomalyResolutionSubstituted},
		{seq: 4, hasRef: true, wantRes: ProbeAnomalyResolutionRealWritten, wantReason: ProbeAnomalyReasonTooManyConsecutive},
		{seq: 5, hasRef: true, wantRes: ProbeAnomalyResolutionRealWritten, wantReason: ProbeAnomalyReasonTooManyConsecutive},
		// 中间出现一次真实 operational（无异常记录）后，seq 归零重新从 1 开始。
		{seq: 1, hasRef: true, wantRes: ProbeAnomalyResolutionSubstituted},
	}
	for i, tc := range cases {
		cycle, clock := exhaustedFailureCycle(t, "m1")
		var ref *ChannelMonitorHistoryRef
		if tc.hasRef {
			ref = &ChannelMonitorHistoryRef{ID: 99, LatencyMs: latencyPtr(600), PingLatencyMs: latencyPtr(100), CheckedAt: cycle.StartedAt}
		}
		plans, err := planProbeCycle(cycle, 7, clock.now, func(string) (*ChannelMonitorHistoryRef, int, error) {
			return ref, tc.seq, nil
		})
		require.NoError(t, err, "cycle %d", i+1)
		require.Equal(t, tc.wantRes, plans[0].Anomaly.Resolution, "cycle %d", i+1)
		require.Equal(t, tc.wantReason, plans[0].Anomaly.Reason, "cycle %d", i+1)
	}
}

func TestProbeCycle_LookupErrorPropagates(t *testing.T) {
	cycle, clock := exhaustedFailureCycle(t, "m1")
	_, err := planProbeCycle(cycle, 7, clock.now, func(string) (*ChannelMonitorHistoryRef, int, error) {
		return nil, 0, errors.New("db down")
	})
	require.Error(t, err)
}

// --- 手动周期驱动（RunCheckWithRetry 的等待循环） ---

func TestDriveProbeCycle_StopsAtOperational(t *testing.T) {
	cycle := newProbeCycle(1, ProbeTriggerManual, time.Now(), []string{"m1"})
	calls := 0
	err := driveProbeCycle(context.Background(), cycle, func(_ context.Context, due []string) []*CheckResult {
		calls++
		require.Equal(t, []string{"m1"}, due)
		if calls == 1 {
			return []*CheckResult{resultAt("m1", MonitorStatusError, latencyPtr(1), time.Now())}
		}
		return []*CheckResult{resultAt("m1", MonitorStatusOperational, latencyPtr(2), time.Now())}
	}, nil)
	require.NoError(t, err)
	require.Equal(t, 2, calls)
	require.True(t, cycle.AllDone())
}

func TestDriveProbeCycle_CanceledContextDiscards(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cycle := newProbeCycle(1, ProbeTriggerManual, time.Now(), []string{"m1"})
	// 首轮失败后取消 ctx：周期作废（返回 ctx 错误），不再探测。
	calls := 0
	err := driveProbeCycle(ctx, cycle, func(_ context.Context, due []string) []*CheckResult {
		calls++
		cancel()
		return []*CheckResult{resultAt("m1", MonitorStatusError, latencyPtr(1), time.Now())}
	}, nil)
	require.ErrorIs(t, err, context.Canceled)
	require.Equal(t, 1, calls)
}

func TestDriveProbeCycle_ImmediateCancelBeforeFirstAttempt(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	cycle := newProbeCycle(1, ProbeTriggerManual, time.Now(), []string{"m1"})
	err := driveProbeCycle(ctx, cycle, func(context.Context, []string) []*CheckResult {
		t.Fatal("probe must not run with a dead context")
		return nil
	}, nil)
	require.ErrorIs(t, err, context.Canceled)
}

// DueModels 不包含未知模型 / 已完成模型（防御性）。
func TestProbeCycle_RecordRoundIgnoresUnknownModels(t *testing.T) {
	cycle := newProbeCycle(1, ProbeTriggerScheduled, time.Now(), []string{"m1"})
	cycle.RecordRound([]*CheckResult{resultAt("ghost", MonitorStatusError, nil, time.Now())}, time.Now())
	require.False(t, cycle.AllDone(), "unknown model results must be ignored, m1 still pending")
	require.Len(t, cycle.Models, 1)
}

func TestDriveProbeCycle_EmitsObserverEvents(t *testing.T) {
	// 压缩退避到 1ms，避免真实等待。
	savedFailure := monitorFailureRetryBackoffs
	monitorFailureRetryBackoffs = []time.Duration{time.Millisecond}
	defer func() { monitorFailureRetryBackoffs = savedFailure }()

	cycle := newProbeCycle(1, ProbeTriggerManual, time.Now(), []string{"m1", "m2"})
	var events []string
	observe := &ProbeRunEventObserver{
		OnCycleStart: func(models []string, _ time.Time) {
			events = append(events, "start:"+strings.Join(models, ","))
		},
		OnAttempt: func(model string, a domain.MonitorProbeAttempt) {
			events = append(events, fmt.Sprintf("attempt:%s#%d:%s", model, a.Seq, a.Status))
		},
		OnWait: func(delay time.Duration) {
			events = append(events, "wait")
		},
	}

	calls := 0
	err := driveProbeCycle(context.Background(), cycle, func(_ context.Context, due []string) []*CheckResult {
		calls++
		if calls == 1 {
			require.ElementsMatch(t, []string{"m1", "m2"}, due)
			return []*CheckResult{
				resultAt("m1", MonitorStatusError, latencyPtr(100), time.Now()),
				resultAt("m2", MonitorStatusOperational, latencyPtr(80), time.Now()),
			}
		}
		require.Equal(t, []string{"m1"}, due, "recovered m2 must not be probed again")
		return []*CheckResult{resultAt("m1", MonitorStatusOperational, latencyPtr(90), time.Now())}
	}, observe)
	require.NoError(t, err)

	// 事件顺序：周期开始 → 每轮各模型的尝试 → 退避等待 → 恢复后无等待直接结束。
	require.Equal(t, []string{
		"start:m1,m2",
		"attempt:m1#1:error",
		"attempt:m2#1:operational",
		"wait",
		"attempt:m1#2:operational",
	}, events)
}
