package schema

import (
	"github.com/Wei-Shaw/sub2api/ent/schema/mixins"
	"github.com/Wei-Shaw/sub2api/internal/domain"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// SubPool holds the schema definition for the SubPool entity.
//
// A sub-pool is an internal isolation unit under a user-visible group: it binds
// a small number of API keys to a small number of upstream accounts so that one
// abusive key only burns its own pool and stays attributable.
type SubPool struct {
	ent.Schema
}

func (SubPool) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "sub_pools"},
	}
}

func (SubPool) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.TimeMixin{},
		mixins.SoftDeleteMixin{},
	}
}

func (SubPool) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("group_id"),
		// 唯一约束通过部分索引实现（WHERE deleted_at IS NULL），与 groups 一致。
		field.String("name").
			MaxLen(100).
			NotEmpty(),
		field.String("description").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.String("kind").
			MaxLen(20).
			Default(domain.SubPoolKindFormal).
			Comment("formal | probe (probe pools use disposable accounts)"),
		field.String("status").
			MaxLen(20).
			Default(domain.SubPoolStatusHealthy).
			Comment("healthy | cooling | closed"),
		field.Int("key_soft_limit").
			Default(domain.SubPoolDefaultKeySoftLimit).
			Comment("Soft cap on bound API keys; 0 means unlimited"),
		field.Time("cooling_until").
			Optional().
			Nillable(),
		field.String("cooling_reason").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.Int("sort_order").
			Default(0),
	}
}

func (SubPool) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("api_keys", APIKey.Type),
		// accounts: 挂在此子池上的上游账号（多对多，经 sub_pool_accounts）。
		// 子池是 owner，因此中间表主键顺序为 (sub_pool_id, account_id)。
		edge.To("accounts", Account.Type).
			Through("sub_pool_accounts", SubPoolAccount.Type),
	}
}

func (SubPool) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("group_id", "status", "kind"),
	}
}
