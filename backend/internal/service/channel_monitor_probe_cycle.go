package service

import (
	"math/rand/v2"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/domain"
)

// V1 探测重试周期（channel-monitor-v1-probe-retry）。
//
// 本文件只放周期状态与最终决策的纯逻辑：状态推进不碰 IO，落库决策通过
// lookup 回调注入参考行/连续序号查询，便于在不依赖 repo 的情况下单测
// （见 channel_monitor_probe_cycle_test.go）。探测与落库的编排分别在
// channel_monitor_runner.go（调度）与 channel_monitor_service.go（手动检测）。

// modelProbeState 单个模型在一个周期内的探测状态。
type modelProbeState struct {
	// Category 由首次结果决定（failure / degraded），决定重试预算；
	// 首次即 operational 时保持空串且 Done 立即为 true。
	Category string
	// Attempts 按时间顺序记录每次尝试的结果。
	Attempts []*CheckResult
	// Done 已拿到 operational 或预算耗尽，不再参与后续轮次。
	Done bool
	// NextAt 下一次允许尝试的时刻 = 上一轮记录时刻 + 对应退避。
	NextAt time.Time
}

// probeCategoryFailure / probeCategoryDegraded 重试策略类别。
const (
	probeCategoryFailure  = "failure"
	probeCategoryDegraded = "degraded"
)

// ProbeCycle 一次探测周期的内存状态（不落库）。
// primary 与每个 extra model 各自独立判定、独立重试（Models 按 model 键控）。
type ProbeCycle struct {
	MonitorID int64
	Trigger   string // scheduled / manual
	StartedAt time.Time
	Models    map[string]*modelProbeState

	// order 记录模型注册顺序（primary 在前），保证落库与返回结果顺序稳定。
	order []string
}

// newProbeCycle 创建周期并注册参与模型。
func newProbeCycle(monitorID int64, trigger string, startedAt time.Time, models []string) *ProbeCycle {
	c := &ProbeCycle{
		MonitorID: monitorID,
		Trigger:   trigger,
		StartedAt: startedAt,
		Models:    make(map[string]*modelProbeState, len(models)),
	}
	for _, model := range models {
		if _, exists := c.Models[model]; !exists {
			c.Models[model] = &modelProbeState{}
			c.order = append(c.order, model)
		}
	}
	return c
}

// ModelNames 按注册顺序返回参与模型列表。
func (c *ProbeCycle) ModelNames() []string {
	out := make([]string, len(c.order))
	copy(out, c.order)
	return out
}

// probeRetryBackoffs 返回该类别的重试退避序列；预算 = 首次 + len(backoffs)。
func probeRetryBackoffs(category string) []time.Duration {
	if category == probeCategoryDegraded {
		return monitorDegradedRetryBackoffs
	}
	return monitorFailureRetryBackoffs
}

// recordAttempt 记录单次尝试并推进状态（首次结果决定类别，operational 立即完成）。
// now 为本轮尝试的结束时刻，用于排程下一次退避。
func (st *modelProbeState) recordAttempt(r *CheckResult, now time.Time) {
	if st.Done || r == nil {
		return
	}
	if len(st.Attempts) == 0 {
		switch r.Status {
		case MonitorStatusDegraded:
			st.Category = probeCategoryDegraded
		case MonitorStatusOperational:
			st.Done = true // 首次即正常：不重试，1 次结束
			st.Attempts = append(st.Attempts, r)
			return
		default: // error / failed
			st.Category = probeCategoryFailure
		}
	}
	st.Attempts = append(st.Attempts, r)
	if r.Status == MonitorStatusOperational {
		st.Done = true
		return
	}
	backoffs := probeRetryBackoffs(st.Category)
	if len(st.Attempts) >= len(backoffs)+1 {
		st.Done = true // 预算耗尽
		return
	}
	st.NextAt = now.Add(backoffs[len(st.Attempts)-1])
}

// RecordRound 记录一轮探测结果（now 为该轮结束时刻）。未知模型的结果被忽略。
func (c *ProbeCycle) RecordRound(results []*CheckResult, now time.Time) {
	for _, r := range results {
		if r == nil {
			continue
		}
		if st, ok := c.Models[r.Model]; ok {
			st.recordAttempt(r, now)
		}
	}
}

// DueModels 返回 now 时刻应当探测的模型：未 Done 且退避已到点。
func (c *ProbeCycle) DueModels(now time.Time) []string {
	var due []string
	for _, model := range c.order {
		st := c.Models[model]
		if st.Done || now.Before(st.NextAt) {
			continue
		}
		due = append(due, model)
	}
	return due
}

// AllDone 所有模型都已结束（拿到 operational 或预算耗尽）。
func (c *ProbeCycle) AllDone() bool {
	for _, st := range c.Models {
		if !st.Done {
			return false
		}
	}
	return true
}

// NextWakeDelay 返回距下一次有模型到点的等待时长。
// ok=false 表示没有待探测模型（调用方应直接收尾）。
func (c *ProbeCycle) NextWakeDelay(now time.Time) (time.Duration, bool) {
	var minDelay time.Duration
	found := false
	for _, st := range c.Models {
		if st.Done {
			continue
		}
		delay := st.NextAt.Sub(now)
		if delay < 0 {
			delay = 0
		}
		if !found || delay < minDelay {
			minDelay = delay
			found = true
		}
	}
	return minDelay, found
}

// selectModelResult 最终结果选取（§3.3 的前两步）：
// 存在 operational → 第一次 operational；否则存在 degraded → latency_ms 最小的一次；
// 全部失败 → nil（由调用方走复制/真实失败决策）。
func selectModelResult(st *modelProbeState) *CheckResult {
	var bestDegraded *CheckResult
	for _, a := range st.Attempts {
		switch a.Status {
		case MonitorStatusOperational:
			return a
		case MonitorStatusDegraded:
			if bestDegraded == nil || attemptLatency(a) < attemptLatency(bestDegraded) {
				bestDegraded = a
			}
		}
	}
	return bestDegraded
}

// attemptLatency degraded 结果的排序延迟；nil 视为最差（排最后）。
func attemptLatency(r *CheckResult) int {
	if r.LatencyMs == nil {
		return int(^uint(0) >> 1) // MaxInt
	}
	return *r.LatencyMs
}

// substituteLatency 复制行的 latency_ms：参考行随机减去 10～50ms（均匀分布，
// 含两端），结果最小为 1；参考行为 NULL 时保持 NULL。
func substituteLatency(ref *int) *int {
	if ref == nil {
		return nil
	}
	v := *ref - (monitorSubstituteLatencyMin + rand.IntN(monitorSubstituteLatencyMax-monitorSubstituteLatencyMin+1))
	if v < 1 {
		v = 1
	}
	return &v
}

// historyRowFromCheckResult 把一次真实检测结果转成历史入库行。
func historyRowFromCheckResult(monitorID int64, r *CheckResult) *ChannelMonitorHistoryRow {
	return &ChannelMonitorHistoryRow{
		MonitorID:     monitorID,
		Model:         r.Model,
		Status:        r.Status,
		LatencyMs:     r.LatencyMs,
		PingLatencyMs: r.PingLatencyMs,
		Message:       r.Message,
		CheckedAt:     r.CheckedAt,
		Quota:         r.Quota,
	}
}

// buildAttemptDetails 把周期内的尝试结果转成异常记录的 JSONB 明细。
// message 已在 checker 内经过 truncateMessage + sanitizeErrorMessage。
func buildAttemptDetails(attempts []*CheckResult) []domain.MonitorProbeAttempt {
	out := make([]domain.MonitorProbeAttempt, 0, len(attempts))
	for i, a := range attempts {
		out = append(out, domain.MonitorProbeAttempt{
			Seq:           i + 1,
			Status:        a.Status,
			LatencyMs:     a.LatencyMs,
			PingLatencyMs: a.PingLatencyMs,
			Message:       a.Message,
			CheckedAt:     a.CheckedAt,
		})
	}
	return out
}

// probeModelPlan 单个模型在一个周期结束后的落库决策。
type probeModelPlan struct {
	Model   string
	Row     *ChannelMonitorHistoryRow // 最终要写的历史行（必然非 nil）
	Anomaly *ProbeAnomalyRow          // 失败策略落空时非 nil
}

// probeCycleRefLookup 为 planProbeCycle 注入参考行与连续序号查询。
// 返回 (参考行, consecutiveSeq, error)；参考行为 nil 表示无可复制行。
type probeCycleRefLookup func(model string) (*ChannelMonitorHistoryRef, int, error)

// planProbeCycle 为整个周期生成落库决策（§3.3 / §3.4 / §3.5）：
//   - 有真实 operational / degraded → 原样写该结果，无异常记录；
//   - 失败策略落空 + 有参考行 + seq ≤ 上限 → 写复制行 + substituted 异常记录；
//   - 否则 → 写最后一次真实失败 + real_written 异常记录（no_reference / too_many_consecutive）。
//
// 尚无任何尝试的模型（周期在首次探测前即被截止）跳过：不写历史也不写异常。
func planProbeCycle(cycle *ProbeCycle, monitorID int64, finishedAt time.Time, lookup probeCycleRefLookup) ([]*probeModelPlan, error) {
	plans := make([]*probeModelPlan, 0, len(cycle.order))
	for _, model := range cycle.order {
		st := cycle.Models[model]
		if len(st.Attempts) == 0 {
			continue
		}
		if final := selectModelResult(st); final != nil {
			plans = append(plans, &probeModelPlan{Model: model, Row: historyRowFromCheckResult(monitorID, final)})
			continue
		}

		ref, seq, err := lookup(model)
		if err != nil {
			return nil, err
		}
		anomaly := &ProbeAnomalyRow{
			MonitorID:       monitorID,
			Model:           model,
			Trigger:         cycle.Trigger,
			TriggerStatus:   st.Attempts[0].Status,
			ConsecutiveSeq:  seq,
			Attempts:        buildAttemptDetails(st.Attempts),
			CycleStartedAt:  cycle.StartedAt,
			CycleFinishedAt: finishedAt,
		}
		if ref != nil && seq <= monitorSubstituteMaxConsecutive {
			anomaly.Resolution = ProbeAnomalyResolutionSubstituted
			refID, refCheckedAt := ref.ID, ref.CheckedAt
			anomaly.ReferenceHistoryID = &refID
			anomaly.ReferenceCheckedAt = &refCheckedAt
			plans = append(plans, &probeModelPlan{
				Model: model,
				// 复制行（§3.4）：字段集合与普通历史行一致；latency 随机递减、
				// message 留空、checked_at 用周期首次探测开始时间（不复制参考行）。
				Row: &ChannelMonitorHistoryRow{
					MonitorID:     monitorID,
					Model:         model,
					Status:        MonitorStatusOperational,
					LatencyMs:     substituteLatency(ref.LatencyMs),
					PingLatencyMs: ref.PingLatencyMs,
					Message:       "",
					CheckedAt:     cycle.StartedAt,
				},
				Anomaly: anomaly,
			})
			continue
		}

		anomaly.Resolution = ProbeAnomalyResolutionRealWritten
		if ref == nil {
			anomaly.Reason = ProbeAnomalyReasonNoReference
		} else {
			anomaly.Reason = ProbeAnomalyReasonTooManyConsecutive
		}
		last := st.Attempts[len(st.Attempts)-1]
		plans = append(plans, &probeModelPlan{
			Model:   model,
			Row:     historyRowFromCheckResult(monitorID, last),
			Anomaly: anomaly,
		})
	}
	return plans, nil
}
