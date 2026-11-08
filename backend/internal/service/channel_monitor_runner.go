package service

import (
	"context"
	"errors"
	"log/slog"
	"math/rand/v2"
	"sync"
	"sync/atomic"
	"time"

	"github.com/alitto/pond/v2"
)

// MonitorScheduler 调度器接口，供 ChannelMonitorService 在 CRUD 时回调，
// 用 setter 注入避免 service ↔ runner 的 wire 依赖环。
type MonitorScheduler interface {
	// Schedule 为指定监控创建（或重置）独立定时任务。
	// 当 m.Enabled=false 时等同于 Unschedule(m.ID)。
	Schedule(m *ChannelMonitor)
	// Unschedule 取消指定监控的定时任务（若存在）。
	Unschedule(id int64)
}

// monitorRunnerSvc 抽出 runner 实际依赖的 service 方法：
//   - 启动时加载 enabled monitor
//   - quota 模式：单次检测即落库（不重试，保持原行为）
//   - probe / quota_probe：探测重试周期（加载快照 → 单轮探测 → 周期收尾落库）
//
// 用接口而非 *ChannelMonitorService 是为了让 runner 单元测试可注入轻量 stub，
// 避免依赖完整的 repo + encryptor 链路。生产实现 *ChannelMonitorService 自然满足。
type monitorRunnerSvc interface {
	ListEnabledMonitors(ctx context.Context) ([]*ChannelMonitor, error)
	RunCheck(ctx context.Context, id int64) ([]*CheckResult, error)
	LoadForProbe(ctx context.Context, id int64) (*ChannelMonitor, error)
	ProbeModels(ctx context.Context, m *ChannelMonitor, models []string) []*CheckResult
	FinalizeProbeCycle(ctx context.Context, m *ChannelMonitor, cycle *ProbeCycle) ([]*CheckResult, error)
}

// ChannelMonitorRunner 渠道监控调度器。
//
// 设计：
//   - 每个 enabled monitor 对应一个独立 goroutine + ticker（按各自 IntervalSeconds）
//   - Start 时一次性加载所有 enabled monitor 并为每个建立任务
//   - Service 在 Create/Update/Delete 后通过 MonitorScheduler 接口回调，
//     即时重建/取消对应任务（无需轮询 DB）
//   - 实际 HTTP 检测交给 pond 池（容量 monitorWorkerConcurrency），
//     防止突发并发拖垮上游
//
// 历史清理与日聚合维护由 OpsCleanupService 的 cron 触发
// ChannelMonitorService.RunDailyMaintenance（复用 leader lock + heartbeat），
// 不在 runner 职责内。
type ChannelMonitorRunner struct {
	svc            monitorRunnerSvc
	settingService *SettingService

	pool         pond.Pool
	parentCtx    context.Context
	parentCancel context.CancelFunc

	mu      sync.Mutex
	tasks   map[int64]*scheduledMonitor
	wg      sync.WaitGroup
	started bool
	stopped bool

	// inFlight 跟踪正在执行的 monitor.ID。fire 调度前会检查避免重复提交，
	// 防止单次检测耗时 > interval 时同一 monitor 被并发执行。
	// 探测重试周期会占满整个重试过程（含退避等待），期间的常规触发被跳过。
	inFlight   map[int64]struct{}
	inFlightMu sync.Mutex

	// afterFunc 重试等待的调度器，默认 time.AfterFunc；测试注入可控实现，
	// 避免真实 sleep（等待期间不占 pond worker 是本设计的核心约束）。
	afterFunc func(d time.Duration, f func()) *time.Timer
	// probeCycleDeadline 单个探测周期的总截止时间（默认 monitorProbeCycleDeadline），
	// 独立字段仅为测试可覆写。
	probeCycleDeadline time.Duration
}

// scheduledMonitor 单个监控的运行时上下文。
type scheduledMonitor struct {
	id       int64
	name     string
	interval time.Duration
	jitter   time.Duration // 每轮 ± [0, jitter] 的均匀随机偏移；0 = 固定间隔
	cancel   context.CancelFunc
}

// nextDelay 计算下一次触发的等待时长：interval ± [0, jitter] 的均匀随机偏移。
// 校验链路已保证 interval - jitter >= monitorMinIntervalSeconds，
// 这里仍 clamp 一次下限，兜底数据库中违反约束的脏数据。
func (t *scheduledMonitor) nextDelay() time.Duration {
	if t.jitter <= 0 {
		return t.interval
	}
	offset := time.Duration(rand.Int64N(int64(2*t.jitter) + 1)) // [0, 2*jitter]
	d := t.interval - t.jitter + offset
	if floor := monitorMinIntervalSeconds * time.Second; d < floor {
		d = floor
	}
	return d
}

// NewChannelMonitorRunner 构造调度器。Start 在 wire 中调用一次。
// settingService 用于在每次 fire 前读取功能开关；传 nil 时视为总是启用（兼容测试）。
//
// pool 在构造时即建好：避免 Start 在 mu 内赋值、fire/Stop 在 mu 外读取的竞态隐患，
// 且 pond.NewPool 创建本身近似零开销，提前建池不会浪费资源。
func NewChannelMonitorRunner(svc *ChannelMonitorService, settingService *SettingService) *ChannelMonitorRunner {
	return newChannelMonitorRunner(svc, settingService)
}

// newChannelMonitorRunner 内部构造，接受最小化接口，便于单元测试注入 stub。
func newChannelMonitorRunner(svc monitorRunnerSvc, settingService *SettingService) *ChannelMonitorRunner {
	ctx, cancel := context.WithCancel(context.Background())
	return &ChannelMonitorRunner{
		svc:                svc,
		settingService:     settingService,
		pool:               pond.NewPool(monitorWorkerConcurrency),
		parentCtx:          ctx,
		parentCancel:       cancel,
		tasks:              make(map[int64]*scheduledMonitor),
		inFlight:           make(map[int64]struct{}),
		afterFunc:          time.AfterFunc,
		probeCycleDeadline: monitorProbeCycleDeadline,
	}
}

// Start 加载所有 enabled monitor 并为每个建立独立定时任务。
// 调用方需保证只调一次（wire ProvideChannelMonitorRunner 内只调一次）。
func (r *ChannelMonitorRunner) Start() {
	if r == nil || r.svc == nil {
		return
	}
	r.mu.Lock()
	if r.started || r.stopped {
		r.mu.Unlock()
		return
	}
	r.started = true
	r.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), monitorStartupLoadTimeout)
	defer cancel()
	enabled, err := r.svc.ListEnabledMonitors(ctx)
	if err != nil {
		slog.Error("channel_monitor: load enabled monitors failed at startup", "error", err)
		return
	}
	for _, m := range enabled {
		r.Schedule(m)
	}
	slog.Info("channel_monitor: runner started", "scheduled_tasks", len(enabled))
}

// Schedule 为指定监控创建（或重置）独立定时任务。
//   - m.Enabled=false 或 APIKeyDecryptFailed=true → 等同于 Unschedule(m.ID)
//   - 已存在的任务会先被取消再重建（适用于 IntervalSeconds 变更场景）
//   - 新任务立即触发首次检测，之后按 IntervalSeconds 周期触发
func (r *ChannelMonitorRunner) Schedule(m *ChannelMonitor) {
	if r == nil || m == nil {
		return
	}
	if !m.Enabled || m.APIKeyDecryptFailed {
		r.Unschedule(m.ID)
		return
	}
	interval := time.Duration(m.IntervalSeconds) * time.Second
	if interval <= 0 {
		// Create/Update 已通过 validateInterval 校验区间，正常路径不可能到这里。
		// 真触发说明数据库中存在违反约束的数据或校验链路有 bug，记 Error 暴露问题。
		slog.Error("channel_monitor: skip schedule for invalid interval",
			"monitor_id", m.ID, "interval_seconds", m.IntervalSeconds)
		return
	}
	jitter := time.Duration(m.JitterSeconds) * time.Second
	if jitter < 0 {
		jitter = 0
	}

	r.mu.Lock()
	if r.stopped {
		r.mu.Unlock()
		return
	}
	if !r.started {
		// Start 之前调用 Schedule 通常意味着 wire 顺序错乱：
		// 当前 wire 顺序是 SetScheduler → Start，CRUD 钩子最早也只能在请求到达时触发，
		// 此时 Start 早已完成。出现此分支时把 monitor 信息打出来便于排查，
		// 不入队、不缓存——交给运维通过重启或修复 wire 解决。
		r.mu.Unlock()
		slog.Warn("channel_monitor: schedule before runner started, skip",
			"monitor_id", m.ID, "name", m.Name)
		return
	}
	if existing, ok := r.tasks[m.ID]; ok {
		existing.cancel()
	}
	ctx, cancel := context.WithCancel(r.parentCtx)
	task := &scheduledMonitor{
		id:       m.ID,
		name:     m.Name,
		interval: interval,
		jitter:   jitter,
		cancel:   cancel,
	}
	r.tasks[m.ID] = task
	r.wg.Add(1)
	r.mu.Unlock()

	go r.runScheduled(ctx, task)
}

// Unschedule 取消指定监控的定时任务（若存在）。
// 已经在执行中的检测会通过 ctx 取消信号传递。
func (r *ChannelMonitorRunner) Unschedule(id int64) {
	if r == nil {
		return
	}
	r.mu.Lock()
	task, ok := r.tasks[id]
	if ok {
		delete(r.tasks, id)
	}
	r.mu.Unlock()
	if ok {
		task.cancel()
	}
}

// Stop 优雅停止：取消所有任务、关闭池。
func (r *ChannelMonitorRunner) Stop() {
	if r == nil {
		return
	}
	r.mu.Lock()
	if r.stopped {
		r.mu.Unlock()
		return
	}
	r.stopped = true
	r.parentCancel()
	r.tasks = nil
	r.mu.Unlock()

	r.wg.Wait()
	r.pool.StopAndWait()
}

// runScheduled 单个监控的循环：立即触发首次（满足"新建/启用即跑"），
// 之后按 interval ± jitter 周期触发；ctx 取消即退出。
// 用 timer 而非 ticker：jitter > 0 时每轮等待时长都需要重新随机化。
func (r *ChannelMonitorRunner) runScheduled(ctx context.Context, task *scheduledMonitor) {
	defer r.wg.Done()

	r.fire(ctx, task)

	timer := time.NewTimer(task.nextDelay())
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			r.fire(ctx, task)
			timer.Reset(task.nextDelay())
		}
	}
}

// fire 提交一次检测到 worker 池。功能开关关闭时跳过本次（不取消任务，
// 重新启用时立即恢复）；池满或重复在飞时也跳过。
// 提交的是「探测周期」：quota 模式等价于原来的单次 RunCheck；
// probe / quota_probe 走重试周期（runCycle）。
func (r *ChannelMonitorRunner) fire(ctx context.Context, task *scheduledMonitor) {
	if r.settingService != nil {
		rt := r.settingService.GetChannelMonitorRuntime(ctx)
		if !rt.ActiveProbesAllowed() {
			return
		}
	}
	if !r.tryAcquireInFlight(task.id) {
		slog.Debug("channel_monitor: skip already in-flight",
			"monitor_id", task.id, "name", task.name)
		return
	}
	if _, ok := r.pool.TrySubmit(func() {
		r.runCycle(ctx, task)
	}); !ok {
		// 池满：丢弃本次检测，但必须释放已占用的 inFlight 槽，否则该 monitor 会被永久卡住。
		r.releaseInFlight(task.id)
		slog.Warn("channel_monitor: worker pool full, skip submission",
			"monitor_id", task.id, "name", task.name)
	}
}

// tryAcquireInFlight 原子地占用 monitor 的 in-flight 槽。
// 已被占用返回 false（调用方应跳过本次提交）。
func (r *ChannelMonitorRunner) tryAcquireInFlight(id int64) bool {
	r.inFlightMu.Lock()
	defer r.inFlightMu.Unlock()
	if _, exists := r.inFlight[id]; exists {
		return false
	}
	r.inFlight[id] = struct{}{}
	return true
}

// releaseInFlight 释放 in-flight 槽。runOne 完成（含 panic recover）后必须调用。
func (r *ChannelMonitorRunner) releaseInFlight(id int64) {
	r.inFlightMu.Lock()
	delete(r.inFlight, id)
	r.inFlightMu.Unlock()
}

// runOne 执行单个监控的检测（quota 等价的单发路径，保留给不需要周期编排的调用方）。
// 普通错误只记日志；API key 解密失败会撤销任务。
func (r *ChannelMonitorRunner) runOne(ctx context.Context, id int64, name string) {
	defer func() {
		if rec := recover(); rec != nil {
			slog.Error("channel_monitor: runner panic",
				"monitor_id", id, "name", name, "panic", rec)
		}
	}()

	if _, err := r.svc.RunCheck(ctx, id); err != nil {
		if errors.Is(err, ErrChannelMonitorAPIKeyDecryptFailed) {
			r.Unschedule(id)
		}
		slog.Warn("channel_monitor: run check failed",
			"monitor_id", id, "name", name, "error", err)
	}
}

// ---------- V1 探测重试周期编排（channel-monitor-v1-probe-retry） ----------
//
// 执行模型（§4.2）：
//   - fire 占 in-flight 槽后提交第一轮到 pond worker；
//   - 每轮 worker 探测到点模型、写入 ProbeCycle 状态，然后通过 AfterFunc
//     排程下一轮——退避等待不占 worker，故障渠道不会占满池子；
//   - 周期 ctx 从任务 ctx 派生（Unschedule / Schedule 重建 / Stop / 切换 V2
//     都会取消它），取消即丢弃周期、不落库；周期截止时间到则用已有结果收尾；
//   - 周期结束（落库或丢弃）后才释放 in-flight 槽并更新 last_checked_at，
//     重试期间的常规 ticker 触发因 in-flight 被跳过。

// probeCycleState 单个探测周期的运行时状态（runner 内存，不落库）。
type probeCycleState struct {
	runner  *ChannelMonitorRunner
	ctx     context.Context // 周期 ctx：deadline 受控 + 随任务 ctx 取消
	task    *scheduledMonitor
	m       *ChannelMonitor // 周期开始时的配置快照，重试期间不变
	cycle   *ProbeCycle
	cancel  context.CancelFunc // 周期 ctx 的释放由 endCycle 负责（runCycle 提交首轮后即返回）
	ended   atomic.Bool        // 收尾标记（回调排程前的快速短路）
	endOnce sync.Once          // 保证收尾（落库/丢弃 + 取消 ctx + 释放 in-flight）恰好一次
}

// runCycle 在 pond worker 中启动一次检测：
//   - quota 模式：等价原 runOne（单次 RunCheck，不重试）；
//   - probe / quota_probe：进入探测重试周期。
//
// 任务结束时（含 panic recover）必须释放 in-flight 槽（由周期收尾负责）。
func (r *ChannelMonitorRunner) runCycle(taskCtx context.Context, task *scheduledMonitor) {
	st := &probeCycleState{runner: r, task: task}
	defer func() {
		if rec := recover(); rec != nil {
			slog.Error("channel_monitor: runner panic",
				"monitor_id", task.id, "name", task.name, "panic", rec)
			r.endCycle(st, false)
		}
	}()

	// 注意不能 defer cancel：runCycle 提交首轮探测后即返回（worker 空闲出来），
	// 周期 ctx 必须活到周期结束，cancel 的所有权移交 endCycle。
	ctx, cancel := context.WithTimeout(taskCtx, r.probeCycleDeadline)
	st.ctx = ctx
	st.cancel = cancel

	m, err := r.svc.LoadForProbe(ctx, task.id)
	if err != nil {
		// 周期尚未开始（无探测结果）：丢弃并释放 in-flight；解密失败撤销任务。
		if errors.Is(err, ErrChannelMonitorAPIKeyDecryptFailed) {
			r.Unschedule(task.id)
		}
		slog.Warn("channel_monitor: load monitor for probe failed",
			"monitor_id", task.id, "name", task.name, "error", err)
		r.endCycle(st, false)
		return
	}
	st.m = m

	if defaultCheckMode(m.CheckMode) == MonitorCheckModeQuota {
		r.runOne(ctx, task.id, task.name)
		r.endCycle(st, false) // runOne 内已落库并更新 last_checked_at，这里只收尾释放槽
		return
	}

	st.cycle = newProbeCycle(m.ID, ProbeTriggerScheduled, time.Now(), probeModelsOf(m))
	r.submitProbeRound(st)
}

// submitProbeRound 尝试把下一轮探测提交到 worker 池。
// ctx 已取消 / 探活被停用 → 丢弃周期；截止 → 用已有结果收尾；
// 池满 → 每隔 monitorRetryResubmitDelay 重试提交（不计为一次尝试）。
func (r *ChannelMonitorRunner) submitProbeRound(st *probeCycleState) {
	if err := st.ctx.Err(); err != nil {
		r.endCycle(st, errors.Is(err, context.DeadlineExceeded))
		return
	}
	if r.settingService != nil {
		if rt := r.settingService.GetChannelMonitorRuntime(st.ctx); !rt.ActiveProbesAllowed() {
			slog.Info("channel_monitor: drop probe cycle, active probes not allowed",
				"monitor_id", st.task.id, "name", st.task.name)
			r.endCycle(st, false)
			return
		}
	}
	if st.cycle.AllDone() {
		r.endCycle(st, true)
		return
	}
	if _, ok := r.pool.TrySubmit(func() {
		r.probeRoundWorker(st)
	}); !ok {
		slog.Warn("channel_monitor: worker pool full, retry submission later",
			"monitor_id", st.task.id, "name", st.task.name,
			"retry_in", monitorRetryResubmitDelay.String())
		r.scheduleCycleCallback(st, monitorRetryResubmitDelay, func() { r.submitProbeRound(st) })
	}
}

// probeRoundWorker 在 worker 中执行一轮探测（单次尝试超时独立于周期截止派生）。
func (r *ChannelMonitorRunner) probeRoundWorker(st *probeCycleState) {
	defer func() {
		if rec := recover(); rec != nil {
			slog.Error("channel_monitor: probe round panic",
				"monitor_id", st.task.id, "name", st.task.name, "panic", rec)
			r.endCycle(st, true) // 已有结果收尾，尽量不丢周期
		}
	}()

	attemptCtx, cancel := context.WithTimeout(st.ctx, monitorRequestTimeout+monitorPingTimeout+monitorRunOneBuffer)
	defer cancel()
	if attemptCtx.Err() != nil {
		r.endCycle(st, errors.Is(st.ctx.Err(), context.DeadlineExceeded))
		return
	}

	due := st.cycle.DueModels(time.Now())
	if len(due) == 0 {
		// 所有未完成模型都还在退避中：排程下一次到点，不占 worker。
		r.scheduleNextWake(st)
		return
	}
	results := st.runner.svc.ProbeModels(attemptCtx, st.m, due)
	if st.ctx.Err() != nil {
		// 尝试被周期取消 / 截止截断：截断产生的失败不算尝试，按结束原因收尾。
		r.endCycle(st, errors.Is(st.ctx.Err(), context.DeadlineExceeded))
		return
	}
	st.cycle.RecordRound(results, time.Now())
	if st.cycle.AllDone() {
		r.endCycle(st, true)
		return
	}
	r.scheduleNextWake(st)
}

// scheduleNextWake 排程下一个到点模型的退避唤醒（AfterFunc，不占 worker）。
func (r *ChannelMonitorRunner) scheduleNextWake(st *probeCycleState) {
	delay, ok := st.cycle.NextWakeDelay(time.Now())
	if !ok {
		r.endCycle(st, true)
		return
	}
	r.scheduleCycleCallback(st, delay, func() { r.submitProbeRound(st) })
}

// scheduleCycleCallback 经 afterFunc 排程回调；周期已结束时忽略（防止结束后复活）。
func (r *ChannelMonitorRunner) scheduleCycleCallback(st *probeCycleState, delay time.Duration, fn func()) {
	if st.ctx.Err() != nil || st.ended.Load() {
		return
	}
	r.afterFunc(delay, fn)
}

// endCycle 周期收尾，恰好执行一次：
// finalize=true 时落库（FinalizeProbeCycle，内部事务写历史 + 异常 + MarkChecked），
// 否则丢弃周期（不落库、不写异常记录）；两种情况都释放 in-flight 槽。
func (r *ChannelMonitorRunner) endCycle(st *probeCycleState, finalize bool) {
	st.endOnce.Do(func() {
		st.ended.Store(true)
		if st.cancel != nil {
			st.cancel()
		}
		if finalize && st.cycle != nil && st.m != nil {
			// 落库用独立 ctx：周期 ctx 此刻可能已因截止到期，不能再用它写库。
			persistCtx, cancel := context.WithTimeout(context.Background(), monitorRequestTimeout)
			defer cancel()
			if _, err := r.svc.FinalizeProbeCycle(persistCtx, st.m, st.cycle); err != nil {
				slog.Warn("channel_monitor: finalize probe cycle failed",
					"monitor_id", st.task.id, "name", st.task.name, "error", err)
			}
		}
		r.releaseInFlight(st.task.id)
	})
}
