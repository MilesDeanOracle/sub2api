package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// RechargeQuickButton 充值页支付方式后面的快捷跳转按钮配置。
//
// 读取路径只走进程内缓存（启动时加载、修改配置后写穿更新），
// 该表仅在缓存预热与后台刷新时被查询，删除策略为硬删除。
type RechargeQuickButton struct {
	ent.Schema
}

func (RechargeQuickButton) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "recharge_quick_buttons"},
	}
}

func (RechargeQuickButton) Fields() []ent.Field {
	return []ent.Field{
		field.String("label").
			MaxLen(50).
			Default(""),
		field.String("icon_url").
			SchemaType(map[string]string{dialect.Postgres: "text"}).
			Default(""),
		field.String("url").
			SchemaType(map[string]string{dialect.Postgres: "text"}).
			NotEmpty(),
		field.String("open_method").
			MaxLen(20).
			Default("new_tab"),
		field.Bool("enabled").
			Default(true),
		field.Int("sort_order").
			Default(0),
		field.Time("created_at").
			Immutable().
			Default(time.Now).
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("updated_at").
			Default(time.Now).
			UpdateDefault(time.Now).
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (RechargeQuickButton) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("enabled"),
	}
}
