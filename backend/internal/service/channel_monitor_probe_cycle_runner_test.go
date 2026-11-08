//go:build unit

package service

import (
	"context"
	"sync"
	"testing"
	"time"
)

// captureAfterFunc 返回一个可控 afterFunc：记录 (delay, fn) 而不真实计时，
// 测试手动触发以推进重试轮次；期间不占 pond worker 是本编排的核心约束。
type captureAfterFunc struct {
	mu   sync.Mutex
	fns  []func()
	caps int
}

func (c *captureAfterFunc) after(d time.Duration, f func()) *time.Timer {
	c.mu.Lock()
	c.caps++
	c.fns = append(c.fns, f)
	c.mu.Unlock()
	return nil
}

// fire 触发最早的挂起回调。
func (c *captureAfterFunc) fire() {
	c.mu.Lock()
	if len(c.fns) == 0 {
		c.mu.Unlock()
		return
	}
	fn := c.fns[0]
	c.fns = c.fns[1:]
	c.mu.Unlock()
	fn()
}

func (c *captureAfterFunc) pending() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.fns)
}

// newProbeRunnerForTest 构造 probe 模式监控的 runner + stub 装配。
func newProbeRunnerForTest(svc *stubMonitorSvc) *ChannelMonitorRunner {
	if svc.monitors == nil {
		svc.monitors = map[int64]*ChannelMonitor{}
	}
	return newRunnerForTest(svc)
}

func registerProbeMonitor(svc *stubMonitorSvc, id int64, primary string, extras ...string) {
	if svc.monitors == nil {
		svc.monitors = map[int64]*ChannelMonitor{}
	}
	svc.monitors[id] = &ChannelMonitor{
		ID: id, Name: "probe-m", Enabled: true,
		CheckMode: MonitorCheckModeProbe, PrimaryModel: primary, ExtraModels: extras,
		IntervalSeconds: 60,
	}
}

// TestProbeCycle_RetryWaitDoesNotOccupyWorker 验证退避等待期间不占 pond worker：
// 监控 1 的周期在等待重试（afterFunc 被捕获不触发）时，监控 2 的检测仍能立即执行。
func TestProbeCycle_RetryWaitDoesNotOccupyWorker(t *testing.T) {
	svc := &stubMonitorSvc{
		runCalled: make(chan int64, 4),
		rounds: [][]*CheckResult{
			{resultAt("m1", MonitorStatusError, intPtr(100), time.Now())},
		},
	}
	r := newProbeRunnerForTest(svc)
	af := &captureAfterFunc{}
	r.afterFunc = af.after
	r.Start()

	registerProbeMonitor(svc, 1, "m1")
	r.Schedule(svc.monitors[1])
	// 第一轮已执行并进入退避等待（挂起 1 个回调）。
	waitFor(t, 2*time.Second, "first probe round done", func() bool { return svc.probeCallsSnapshot() == 1 })
	waitFor(t, time.Second, "backoff scheduled", func() bool { return af.pending() == 1 })

	// 监控 2 走 quota 路径（RunCheck）：能立即执行说明 worker 没被监控 1 的等待占用。
	registerProbeMonitor(svc, 2, "quota")
	svc.monitors[2].CheckMode = MonitorCheckModeQuota
	r.Schedule(svc.monitors[2])
	select {
	case id := <-svc.runCalled:
		if id != 2 {
			t.Fatalf("expected monitor 2 check to run, got %d", id)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("monitor 2 check starved: retry wait is occupying workers")
	}

	r.Stop()
}

// TestProbeCycle_InFlightHeldDuringRetry 验证重试期间 ticker 触发被 in-flight 跳过：
// 不产生新周期（LoadForProbe / ProbeModels 不再被调用）。
func TestProbeCycle_InFlightHeldDuringRetry(t *testing.T) {
	svc := &stubMonitorSvc{
		rounds: [][]*CheckResult{
			{resultAt("m1", MonitorStatusError, intPtr(100), time.Now())},
		},
	}
	r := newProbeRunnerForTest(svc)
	af := &captureAfterFunc{}
	r.afterFunc = af.after
	r.Start()

	registerProbeMonitor(svc, 3, "m1")
	r.Schedule(svc.monitors[3])
	waitFor(t, 2*time.Second, "first probe round done", func() bool { return svc.probeCallsSnapshot() == 1 })

	// ticker 再次触发（直接调 fire 模拟）：in-flight 被占，整轮跳过。
	task := runnerTaskPtr(r, 3)
	r.fire(context.Background(), task)
	time.Sleep(100 * time.Millisecond)
	if got := svc.probeCallsSnapshot(); got != 1 {
		t.Fatalf("in-flight skip failed: probe called %d times", got)
	}
	if got := svc.finalizeCount.Load(); got != 0 {
		t.Fatalf("cycle must not finalize during retries, got %d", got)
	}

	r.Stop()
}

// TestProbeCycle_UnscheduleMidRetryDropsCycle 验证重试中 Unschedule：
// 周期丢弃（不落库）、in-flight 释放、回调触发后也不会复活。
func TestProbeCycle_UnscheduleMidRetryDropsCycle(t *testing.T) {
	svc := &stubMonitorSvc{
		rounds: [][]*CheckResult{
			{resultAt("m1", MonitorStatusError, intPtr(100), time.Now())},
		},
	}
	r := newProbeRunnerForTest(svc)
	af := &captureAfterFunc{}
	r.afterFunc = af.after
	r.Start()

	registerProbeMonitor(svc, 4, "m1")
	r.Schedule(svc.monitors[4])
	waitFor(t, 2*time.Second, "first probe round done", func() bool { return svc.probeCallsSnapshot() == 1 })
	waitFor(t, time.Second, "backoff scheduled", func() bool { return af.pending() == 1 })

	// 任务被取消（删除 / 编辑 / 切 V2 都走这里）。
	r.Unschedule(4)
	// 到点回调触发：submitProbeRound 应发现 ctx 已取消 → 丢弃周期。
	af.fire()
	waitFor(t, time.Second, "cycle dropped without finalize", func() bool { return svc.finalizeCount.Load() == 0 })

	// in-flight 已释放：可以重新占用。
	if !r.tryAcquireInFlight(4) {
		t.Fatal("in-flight slot was not released after cycle drop")
	}
	r.releaseInFlight(4)

	stoppedWithin(t, r, 3*time.Second)
}

// TestProbeCycle_StopMidRetryDropsCycle 验证 Stop 取消等待中的周期且不落库。
func TestProbeCycle_StopMidRetryDropsCycle(t *testing.T) {
	svc := &stubMonitorSvc{
		rounds: [][]*CheckResult{
			{resultAt("m1", MonitorStatusError, intPtr(100), time.Now())},
		},
	}
	r := newProbeRunnerForTest(svc)
	af := &captureAfterFunc{}
	r.afterFunc = af.after
	r.Start()

	registerProbeMonitor(svc, 5, "m1")
	r.Schedule(svc.monitors[5])
	waitFor(t, 2*time.Second, "first probe round done", func() bool { return svc.probeCallsSnapshot() == 1 })

	stoppedWithin(t, r, 3*time.Second)
	if got := svc.finalizeCount.Load(); got != 0 {
		t.Fatalf("stopped cycle must not finalize, got %d", got)
	}
	// 真实场景下退避定时器到点后会发现 ctx 已取消并释放槽（最迟一个退避周期）；
	// 捕获式 afterFunc 需手动触发等价的到点回调。
	for af.pending() > 0 {
		af.fire()
	}
	if !r.tryAcquireInFlight(5) {
		t.Fatal("in-flight slot leaked after Stop")
	}
	r.releaseInFlight(5)
}

// TestProbeCycle_DeadlineFinalizesWithRecordedAttempts 验证周期截止时间到：
// 用已有尝试结果收尾（Finalize 被调用），而不是丢弃。
func TestProbeCycle_DeadlineFinalizesWithRecordedAttempts(t *testing.T) {
	svc := &stubMonitorSvc{
		rounds: [][]*CheckResult{
			{resultAt("m1", MonitorStatusError, intPtr(100), time.Now())},
		},
	}
	r := newProbeRunnerForTest(svc)
	r.probeCycleDeadline = 250 * time.Millisecond // 早于首个退避（3s），必然触发截止收尾
	r.Start()

	registerProbeMonitor(svc, 6, "m1")
	r.Schedule(svc.monitors[6])
	waitFor(t, 2*time.Second, "first probe round done", func() bool { return svc.probeCallsSnapshot() == 1 })

	// 截止后下一次唤醒（真实 AfterFunc，+3s）应走截止收尾。
	waitFor(t, 6*time.Second, "deadline finalize", func() bool { return svc.finalizeCount.Load() == 1 })

	stoppedWithin(t, r, 3*time.Second)
}

// TestProbeCycle_RecoveredCycleFinalizes 验证重试恢复后的正常收尾链路：
// 第 2 轮 operational → 立即 Finalize（无需等退避耗尽）→ in-flight 释放。
func TestProbeCycle_RecoveredCycleFinalizes(t *testing.T) {
	svc := &stubMonitorSvc{
		rounds: [][]*CheckResult{
			{resultAt("m1", MonitorStatusError, intPtr(100), time.Now())},
			{resultAt("m1", MonitorStatusOperational, intPtr(120), time.Now())},
		},
	}
	r := newProbeRunnerForTest(svc)
	r.afterFunc = time.AfterFunc // 真实退避（3s）
	r.Start()

	registerProbeMonitor(svc, 7, "m1")
	r.Schedule(svc.monitors[7])
	waitFor(t, 8*time.Second, "cycle finalized after recovery", func() bool { return svc.finalizeCount.Load() == 1 })
	if got := svc.probeCallsSnapshot(); got != 2 {
		t.Fatalf("expected exactly 2 probe rounds, got %d", got)
	}

	stoppedWithin(t, r, 3*time.Second)
}

// probeCallsSnapshot 线程安全读取探测轮次计数。
func (s *stubMonitorSvc) probeCallsSnapshot() int64 {
	s.probeMu.Lock()
	defer s.probeMu.Unlock()
	return s.probeCalls
}
