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

// APIKeySubPoolBinding is the append-only history of API key to sub-pool
// bindings. Attribution after an incident needs to know which pool a key was in
// at the time, not just where it sits now.
type APIKeySubPoolBinding struct {
	ent.Schema
}

func (APIKeySubPoolBinding) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "api_key_sub_pool_bindings"},
	}
}

func (APIKeySubPoolBinding) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("api_key_id"),
		field.Int64("sub_pool_id"),
		field.Int64("group_id"),
		field.Time("bound_at").
			Default(time.Now).
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("unbound_at").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.String("reason").
			MaxLen(40).
			Comment("initial | probe_graduation | cooling_migration | admin_manual | pool_removed"),
		field.String("operator").
			MaxLen(64).
			Default("system").
			Comment("'system' or 'admin:<user_id>'"),
		field.String("note").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "text"}),
	}
}

func (APIKeySubPoolBinding) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("api_key_id", "bound_at"),
		index.Fields("sub_pool_id", "bound_at"),
	}
}
