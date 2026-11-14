package schema

import (
	"encoding/json"
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type OperationsPelicanResult struct{ ent.Schema }

func (OperationsPelicanResult) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "operations_pelican_results"}}
}
func (OperationsPelicanResult) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("task_id"), field.String("run_id"), field.Int64("account_id"), field.String("trigger_key"), field.String("status"),
		field.JSON("group_snapshots", json.RawMessage{}), field.JSON("payload", json.RawMessage{}),
		field.Int("pool_slot").Optional().Nillable(), field.String("worker_token").Default(""), field.Time("heartbeat_at").Optional().Nillable(), field.Time("execution_deadline_at").Optional().Nillable(),
		field.Text("storage_binding_encrypted").Default(""), field.String("object_key").Default(""), field.String("version_id").Default(""),
		field.Time("queued_at").Default(time.Now), field.Time("expires_at"), field.String("cleanup_status").Default("active"), field.Int("cleanup_attempts").Default(0), field.Text("cleanup_error").Default(""), field.Time("cleanup_next_at").Optional().Nillable(),
		field.Time("created_at").Default(time.Now), field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}
func (OperationsPelicanResult) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("run_id", "account_id").Unique(), index.Fields("pool_slot").Unique(), index.Fields("task_id", "account_id", "id"), index.Fields("task_id", "trigger_key"), index.Fields("status", "queued_at"), index.Fields("expires_at", "cleanup_next_at"),
	}
}
