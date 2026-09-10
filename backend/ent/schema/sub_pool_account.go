package schema

import (
	"time"

	"github.com/Wei-Shaw/sub2api/internal/domain"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// SubPoolAccount holds the edge schema for the sub_pool_accounts relationship.
// group_id is denormalised so an account can be constrained to at most one
// sub-pool per group.
type SubPoolAccount struct {
	ent.Schema
}

func (SubPoolAccount) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "sub_pool_accounts"},
		// Composite primary key: (sub_pool_id, account_id).
		field.ID("sub_pool_id", "account_id"),
	}
}

func (SubPoolAccount) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("sub_pool_id"),
		field.Int64("account_id"),
		field.Int64("group_id"),
		field.String("role").
			MaxLen(20).
			Default(domain.SubPoolAccountRolePrimary).
			Comment("primary | standby"),
		field.Time("created_at").
			Immutable().
			Default(time.Now).
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (SubPoolAccount) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("sub_pool", SubPool.Type).
			Unique().
			Required().
			Field("sub_pool_id"),
		edge.To("account", Account.Type).
			Unique().
			Required().
			Field("account_id"),
	}
}

func (SubPoolAccount) Indexes() []ent.Index {
	return []ent.Index{
		// 一个账号在同一分组内最多归属一个子池。
		index.Fields("group_id", "account_id").Unique(),
		index.Fields("account_id"),
	}
}
