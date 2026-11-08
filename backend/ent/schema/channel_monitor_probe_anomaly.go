package schema

import (
	"time"

	"github.com/Wei-Shaw/sub2api/internal/domain"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// ChannelMonitorProbeAnomaly holds the schema definition for the
// ChannelMonitorProbeAnomaly entity.
//
// 渠道监控 V1 探测异常记录：失败策略（首次 error/failed 且重试后未恢复）落空的
// 周期一行/模型。复制行对用户无感，管理员靠本表区分「真实结果」与「替代结果」。
// 保留 30 天（monitorProbeAnomalyRetentionDays），与历史明细保留期一致。
// 注意：列名 "trigger" 是 PG 保留字，所有原生 SQL 必须带双引号引用。
type ChannelMonitorProbeAnomaly struct {
	ent.Schema
}

func (ChannelMonitorProbeAnomaly) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "channel_monitor_probe_anomalies"},
	}
}

func (ChannelMonitorProbeAnomaly) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("monitor_id"),
		field.String("model").
			NotEmpty().
			MaxLen(200),
		// scheduled / manual
		field.String("trigger").
			MaxLen(20),
		// 首次失败状态：error / failed
		field.String("trigger_status").
			MaxLen(20),
		// substituted / real_written
		field.String("resolution").
			MaxLen(20),
		// real_written 时：no_reference / too_many_consecutive；substituted 为空串
		field.String("reason").
			Default("").
			MaxLen(50),
		field.Int("consecutive_seq").
			Comment("consecutive anomaly cycle seq, starts at 1"),
		field.Int16("attempt_count"),
		// 每次尝试明细（domain.MonitorProbeAttempt，JSONB）。
		field.JSON("attempts", []domain.MonitorProbeAttempt{}),
		field.Int64("reference_history_id").
			Optional().
			Nillable().
			Comment("copied operational history row id (substituted only)"),
		field.Time("reference_checked_at").
			Optional().
			Nillable(),
		field.Int64("written_history_id").
			Comment("history row ultimately written in this cycle"),
		field.Time("cycle_started_at"),
		field.Time("cycle_finished_at"),
		field.Time("created_at").
			Default(time.Now),
	}
}

func (ChannelMonitorProbeAnomaly) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("monitor", ChannelMonitor.Type).
			Ref("probe_anomalies").
			Field("monitor_id").
			Unique().
			Required(),
	}
}

func (ChannelMonitorProbeAnomaly) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("monitor_id", "model", "created_at"),
		index.Fields("created_at"),
		index.Fields("written_history_id"),
	}
}
