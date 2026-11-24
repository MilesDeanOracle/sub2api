package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
)

// OperationsPelicanAccountFlag 持久化账号级检测开关。权威 DDL 是迁移 257，
// 此 schema 仅保持 ent 代码生成与数据库结构一致（repo 走原生 SQL）。
type OperationsPelicanAccountFlag struct{ ent.Schema }

func (OperationsPelicanAccountFlag) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "operations_pelican_account_flags"}}
}
func (OperationsPelicanAccountFlag) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("account_id"), field.Bool("detect_paused").Default(false),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}
func (OperationsPelicanAccountFlag) Indexes() []ent.Index { return nil }
