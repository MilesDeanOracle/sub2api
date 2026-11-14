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

type OperationsPelicanTask struct{ ent.Schema }

func (OperationsPelicanTask) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "operations_pelican_tasks"}}
}
func (OperationsPelicanTask) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").MaxLen(200), field.Bool("enabled").Default(false), field.Int("interval_hours").Positive(),
		field.JSON("account_configs", json.RawMessage{}), field.String("reasoning_effort").Default(""), field.Text("prompt"),
		field.String("storage_source"), field.Int64("config_version").Default(1),
		field.Time("last_triggered_at").Optional().Nillable(), field.Time("next_run_at").Optional().Nillable(),
		field.Int64("created_by"), field.Int64("updated_by"), field.Time("created_at").Default(time.Now), field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now), field.Time("deleted_at").Optional().Nillable(),
	}
}
func (OperationsPelicanTask) Indexes() []ent.Index { return []ent.Index{index.Fields("next_run_at")} }
