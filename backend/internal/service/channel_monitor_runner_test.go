//go:build unit

package service

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// stubMonitorSvc 实现 monitorRunnerSvc，用于隔离 runner 与真实 service/repo。
type stubMonitorSvc struct {
	enabled    []*ChannelMonitor
	runCount   atomic.Int64
	runCalled  chan int64 // 每次 RunCheck 触发时 push 一次（缓冲足够大避免阻塞）
	runErr     error
	listErr    error
	runHoldFor time.Duration // RunCheck 内额外阻塞的时长（可被 ctx 取消打断）
	// runHoldUncancellable RunCheck 内不可取消的收尾时长（验证 StopAndWait 等 worker 退出）。
	runHoldUncancellable time.Duration

	// ---- 探测重试周期（probe / quota_probe 模式） ----
	// monitors 提供 LoadForProbe 的监控快照；未注册的 id 返回默认 quota 监控，
	// 让既有调度语义测试（RunCheck 路径）无需改动。
	monitors map[int64]*ChannelMonitor
	loadErr  error
	// rounds 是 ProbeModels 的轮次脚本：每次调用弹出一轮结果；耗尽后返回空。
	rounds [][]*CheckResult
	// probeModels 可选地完全接管 ProbeModels（优先于 rounds）。
	probeModels func(ctx context.Context, m *ChannelMonitor, models []string) []*CheckResult

	probeMu    sync.Mutex
	probeCalls int64
	probed     [][]string // 每轮实际探测的模型列表

	finalizeCount atomic.Int64
	finalizeErr   error
}

func (s *stubMonitorSvc) ListEnabledMonitors(_ context.Context) ([]*ChannelMonitor, error) {
	if s.listErr != nil {
		return nil, s.listErr
	}
	return s.enabled, nil
}

func (s *stubMonitorSvc) RunCheck(ctx context.Context, id int64) ([]*CheckResult, error) {
	s.runCount.Add(1)
	if s.runCalled != nil {
		select {
		case s.runCalled <- id:
		default:
		}
	}
	if s.runHoldFor > 0 {
		select {
		case <-time.After(s.runHoldFor):
		case <-ctx.Done():
		}
	}
	if s.runHoldUncancellable > 0 {
		time.Sleep(s.runHoldUncancellable)
	}
	return nil, s.runErr
}

// LoadForProbe 返回注册的监控快照；未注册的 id 返回默认 quota 监控
// （走 RunCheck 单发路径，保持既有调度测试的行为）。
func (s *stubMonitorSvc) LoadForProbe(_ context.Context, id int64) (*ChannelMonitor, error) {
	if s.loadErr != nil {
		return nil, s.loadErr
	}
	if m, ok := s.monitors[id]; ok {
		clone := *m
		return &clone, nil
	}
	return &ChannelMonitor{ID: id, CheckMode: MonitorCheckModeQuota, PrimaryModel: "quota"}, nil
}

// ProbeModels 弹出一轮脚本结果；脚本耗尽返回 nil（不再推进状态，周期停在等待）。
func (s *stubMonitorSvc) ProbeModels(ctx context.Context, m *ChannelMonitor, models []string) []*CheckResult {
	s.probeMu.Lock()
	s.probeCalls++
	s.probed = append(s.probed, append([]string{}, models...))
	var round []*CheckResult
	if len(s.rounds) > 0 {
		round, s.rounds = s.rounds[0], s.rounds[1:]
	}
	s.probeMu.Unlock()
	if s.probeModels != nil {
		return s.probeModels(ctx, m, models)
	}
	return round
}

// FinalizeProbeCycle 记录收尾次数并返回 finalizeErr。
func (s *stubMonitorSvc) FinalizeProbeCycle(_ context.Context, _ *ChannelMonitor, _ *ProbeCycle) ([]*CheckResult, error) {
	s.finalizeCount.Add(1)
	return nil, s.finalizeErr
}

func newRunnerForTest(svc monitorRunnerSvc) *ChannelMonitorRunner {
	return newChannelMonitorRunner(svc, nil)
}

// 等待 condition 在 timeout 内变 true，否则 t.Fatalf。轮询 5ms 一次。
func waitFor(t *testing.T, timeout time.Duration, msg string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	if !cond() {
		t.Fatalf("waitFor timed out: %s", msg)
	}
}

func runnerTaskCount(r *ChannelMonitorRunner) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.tasks)
}

func runnerTaskPtr(r *ChannelMonitorRunner, id int64) *scheduledMonitor {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.tasks[id]
}

// TestSchedule_AddsTaskAndFiresOnce 验证 Schedule 后立即触发一次首检测，并把任务记入 tasks 表。
func TestSchedule_AddsTaskAndFiresOnce(t *testing.T) {
	svc := &stubMonitorSvc{runCalled: make(chan int64, 4)}
	r := newRunnerForTest(svc)
	r.Start() // svc.enabled 为空，Start 立即完成

	r.Schedule(&ChannelMonitor{ID: 1, Name: "m1", Enabled: true, IntervalSeconds: 60})

	if got := runnerTaskCount(r); got != 1 {
		t.Fatalf("expected 1 scheduled task, got %d", got)
	}

	select {
	case id := <-svc.runCalled:
		if id != 1 {
			t.Fatalf("expected first fire for id=1, got %d", id)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("expected immediate first fire within 2s")
	}

	r.Stop()
}

// TestSchedule_ReplaceCancelsOldTask 验证对同一 id 二次 Schedule 会替换旧 task 实例。
// （旧 goroutine 通过 ctx 取消退出；这里以 task 指针不同 + Stop 不超时作为证据。）
func TestSchedule_ReplaceCancelsOldTask(t *testing.T) {
	svc := &stubMonitorSvc{runCalled: make(chan int64, 8)}
	r := newRunnerForTest(svc)
	r.Start()

	m := &ChannelMonitor{ID: 7, Name: "m7", Enabled: true, IntervalSeconds: 60}
	r.Schedule(m)
	first := runnerTaskPtr(r, 7)
	if first == nil {
		t.Fatal("first schedule did not register task")
	}

	r.Schedule(m)
	second := runnerTaskPtr(r, 7)
	if second == nil {
		t.Fatal("second schedule did not register task")
	}
	if first == second {
		t.Fatal("re-Schedule should create a new scheduledMonitor instance")
	}

	stoppedWithin(t, r, 3*time.Second)
}

// TestUnschedule_RemovesTask 验证 Unschedule 删除 task 并使对应 goroutine 退出。
func TestUnschedule_RemovesTask(t *testing.T) {
	svc := &stubMonitorSvc{runCalled: make(chan int64, 4)}
	r := newRunnerForTest(svc)
	r.Start()

	r.Schedule(&ChannelMonitor{ID: 3, Enabled: true, IntervalSeconds: 60})
	waitFor(t, time.Second, "task registered", func() bool { return runnerTaskCount(r) == 1 })

	r.Unschedule(3)
	if got := runnerTaskCount(r); got != 0 {
		t.Fatalf("expected tasks empty after Unschedule, got %d", got)
	}

	stoppedWithin(t, r, 3*time.Second)
}

// TestSchedule_DisabledRedirectsToUnschedule 验证 Enabled=false 等同于 Unschedule。
func TestSchedule_DisabledRedirectsToUnschedule(t *testing.T) {
	svc := &stubMonitorSvc{runCalled: make(chan int64, 4)}
	r := newRunnerForTest(svc)
	r.Start()

	r.Schedule(&ChannelMonitor{ID: 9, Enabled: true, IntervalSeconds: 60})
	waitFor(t, time.Second, "task registered", func() bool { return runnerTaskCount(r) == 1 })

	r.Schedule(&ChannelMonitor{ID: 9, Enabled: false, IntervalSeconds: 60})
	if got := runnerTaskCount(r); got != 0 {
		t.Fatalf("expected tasks empty after disabled re-Schedule, got %d", got)
	}

	stoppedWithin(t, r, 3*time.Second)
}

func TestSchedule_DecryptFailedRedirectsToUnschedule(t *testing.T) {
	svc := &stubMonitorSvc{runCalled: make(chan int64, 4)}
	r := newRunnerForTest(svc)
	r.Start()

	r.Schedule(&ChannelMonitor{ID: 10, Enabled: true, IntervalSeconds: 60})
	waitFor(t, time.Second, "task registered", func() bool { return runnerTaskCount(r) == 1 })

	r.Schedule(&ChannelMonitor{ID: 10, Enabled: true, IntervalSeconds: 60, APIKeyDecryptFailed: true})
	if got := runnerTaskCount(r); got != 0 {
		t.Fatalf("expected tasks empty after decrypt-failed re-Schedule, got %d", got)
	}

	stoppedWithin(t, r, 3*time.Second)
}

func TestSchedule_RepairedAPIKeyCanBeScheduled(t *testing.T) {
	svc := &stubMonitorSvc{runCalled: make(chan int64, 1)}
	r := newRunnerForTest(svc)
	r.Start()

	r.Schedule(&ChannelMonitor{ID: 11, Enabled: true, IntervalSeconds: 60, APIKeyDecryptFailed: true})
	if got := runnerTaskCount(r); got != 0 {
		t.Fatalf("expected no task for decrypt-failed monitor, got %d", got)
	}

	r.Schedule(&ChannelMonitor{ID: 11, Enabled: true, IntervalSeconds: 60, APIKey: "replacement-key"})
	if got := runnerTaskCount(r); got != 1 {
		t.Fatalf("expected repaired monitor to be scheduled, got %d tasks", got)
	}
	select {
	case id := <-svc.runCalled:
		if id != 11 {
			t.Fatalf("expected repaired monitor id=11 to fire, got %d", id)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("expected repaired monitor to fire immediately")
	}

	stoppedWithin(t, r, 3*time.Second)
}

func TestRunOne_DecryptFailureUnschedulesTask(t *testing.T) {
	svc := &stubMonitorSvc{
		runCalled: make(chan int64, 1),
		runErr:    ErrChannelMonitorAPIKeyDecryptFailed,
	}
	r := newRunnerForTest(svc)
	r.Start()

	r.Schedule(&ChannelMonitor{ID: 12, Enabled: true, IntervalSeconds: 60})
	select {
	case id := <-svc.runCalled:
		if id != 12 {
			t.Fatalf("expected failing monitor id=12 to fire, got %d", id)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("expected failing monitor to fire immediately")
	}
	waitFor(t, time.Second, "decrypt-failed task unscheduled", func() bool { return runnerTaskCount(r) == 0 })

	stoppedWithin(t, r, 3*time.Second)
}

// TestSchedule_InvalidIntervalSkipped 验证 IntervalSeconds<=0 不会注册任务（防御性检查）。
func TestSchedule_InvalidIntervalSkipped(t *testing.T) {
	svc := &stubMonitorSvc{}
	r := newRunnerForTest(svc)
	r.Start()

	r.Schedule(&ChannelMonitor{ID: 1, Enabled: true, IntervalSeconds: 0})
	if got := runnerTaskCount(r); got != 0 {
		t.Fatalf("expected no task for invalid interval, got %d", got)
	}
	r.Stop()
}

// TestSchedule_BeforeStartIsNoOp 验证 Start 之前调用 Schedule 不会注册任务。
func TestSchedule_BeforeStartIsNoOp(t *testing.T) {
	svc := &stubMonitorSvc{}
	r := newRunnerForTest(svc)
	// 故意不调用 Start

	r.Schedule(&ChannelMonitor{ID: 1, Enabled: true, IntervalSeconds: 60})
	if got := runnerTaskCount(r); got != 0 {
		t.Fatalf("expected no task before Start, got %d", got)
	}
	r.Stop()
}

// TestStart_LoadsAllEnabledMonitors 验证 Start 会为 ListEnabledMonitors 返回的每条记录建立任务。
func TestStart_LoadsAllEnabledMonitors(t *testing.T) {
	svc := &stubMonitorSvc{
		enabled: []*ChannelMonitor{
			{ID: 1, Enabled: true, IntervalSeconds: 60},
			{ID: 2, Enabled: true, IntervalSeconds: 60},
			{ID: 3, Enabled: true, IntervalSeconds: 60},
		},
	}
	r := newRunnerForTest(svc)
	r.Start()
	waitFor(t, 2*time.Second, "all 3 tasks scheduled", func() bool { return runnerTaskCount(r) == 3 })

	stoppedWithin(t, r, 3*time.Second)
}

func TestStart_SkipsDecryptFailedMonitor(t *testing.T) {
	svc := &stubMonitorSvc{
		enabled: []*ChannelMonitor{
			{ID: 4, Enabled: true, IntervalSeconds: 60, APIKeyDecryptFailed: true},
		},
	}
	r := newRunnerForTest(svc)
	r.Start()

	if got := runnerTaskCount(r); got != 0 {
		t.Fatalf("expected no task for decrypt-failed startup monitor, got %d", got)
	}
	if got := svc.runCount.Load(); got != 0 {
		t.Fatalf("expected decrypt-failed startup monitor not to run, got %d calls", got)
	}

	stoppedWithin(t, r, 3*time.Second)
}

// TestStop_DrainsAllGoroutines 验证 Stop 会等待所有调度 goroutine 退出（无游离）。
func TestStop_DrainsAllGoroutines(t *testing.T) {
	svc := &stubMonitorSvc{}
	r := newRunnerForTest(svc)
	r.Start()

	for id := int64(1); id <= 5; id++ {
		r.Schedule(&ChannelMonitor{ID: id, Enabled: true, IntervalSeconds: 60})
	}
	waitFor(t, 2*time.Second, "5 tasks scheduled", func() bool { return runnerTaskCount(r) == 5 })

	stoppedWithin(t, r, 3*time.Second)
}

// TestStop_WaitsForInFlightCheck 验证 Stop 会等待正在执行的 RunCheck 退出（pool.StopAndWait）。
// 注意：探测重试改造后周期 ctx 从任务 ctx 派生，Stop 会取消正在执行的检测
// （上游等待被尽早中止）；本用例的 stub 故意不响应 ctx 取消，
// 以验证 StopAndWait 仍会等 worker 真正退出而不是丢弃。
func TestStop_WaitsForInFlightCheck(t *testing.T) {
	svc := &stubMonitorSvc{
		runCalled: make(chan int64, 1),
		// runHoldFor 的等待现在会被 ctx 取消打断（新语义：重试周期必须可取消），
		// 因此这里用不可取消的短睡眠模拟「检测正在收尾、无法立即退出」。
		runHoldUncancellable: 150 * time.Millisecond,
	}
	r := newRunnerForTest(svc)
	r.Start()
	r.Schedule(&ChannelMonitor{ID: 1, Enabled: true, IntervalSeconds: 60})

	select {
	case <-svc.runCalled:
	case <-time.After(2 * time.Second):
		t.Fatal("first fire never happened")
	}

	start := time.Now()
	stoppedWithin(t, r, 3*time.Second)
	elapsed := time.Since(start)
	// Stop 必须等待 in-flight check 跑完（不可取消的 150ms 收尾），耗时下界约 75ms。
	if elapsed < 75*time.Millisecond {
		t.Fatalf("Stop returned too fast (%v); did not wait for in-flight check", elapsed)
	}
}

// TestInFlight_PoolFullReleasesSlot 直接驱动 fire 路径，模拟 pool.TrySubmit 失败时 inFlight 必须释放。
// 用一个小型 stub pool 替换 r.pool 不便（pond.Pool 是接口但 mock 麻烦），
// 改为：占满 inFlight 后直接 fire，验证不会在 inFlight 空槽时永久卡住。
func TestInFlight_AcquireReleaseSymmetric(t *testing.T) {
	svc := &stubMonitorSvc{}
	r := newRunnerForTest(svc)

	if !r.tryAcquireInFlight(42) {
		t.Fatal("first acquire should succeed")
	}
	if r.tryAcquireInFlight(42) {
		t.Fatal("second acquire (no release) must fail")
	}
	r.releaseInFlight(42)
	if !r.tryAcquireInFlight(42) {
		t.Fatal("acquire after release should succeed")
	}
	r.releaseInFlight(42)
}

// stoppedWithin 在 timeout 内并行调用 Stop，超时则 Fatal。验证 Stop 不会阻塞。
func stoppedWithin(t *testing.T, r *ChannelMonitorRunner, timeout time.Duration) {
	t.Helper()
	done := make(chan struct{})
	var once sync.Once
	go func() {
		r.Stop()
		once.Do(func() { close(done) })
	}()
	select {
	case <-done:
	case <-time.After(timeout):
		t.Fatalf("Stop did not return within %s — leaked goroutine?", timeout)
	}
}
