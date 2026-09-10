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

// APIKeyReputation materialises a per-key reputation score derived from
// moderation and prompt-audit hits.
//
// The row is not the source of truth: the score is recomputed from the event
// tables on every sweep. What only lives here is the sanction already applied,
// which is what stops the job from re-banning the same key every cycle.
type APIKeyReputation struct {
	ent.Schema
}

func (APIKeyReputation) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "api_key_reputation"},
	}
}

func (APIKeyReputation) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("api_key_id").Unique(),
		field.Int("score").
			Default(100).
			Comment("100 = clean, 0 = worst"),
		field.Int("severe_hits").Default(0),
		field.Int("total_hits").Default(0),
		field.Time("last_event_at").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("scored_at").
			Default(time.Now).
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.String("sanction").
			MaxLen(20).
			Default("none").
			Comment("none | demoted | disabled"),
		field.Time("sanctioned_at").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.String("sanction_reason").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.Time("created_at").
			Default(time.Now).
			Immutable().
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("updated_at").
			Default(time.Now).
			UpdateDefault(time.Now).
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (APIKeyReputation) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("score"),
	}
}
