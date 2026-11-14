package entities

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

type RequestSubmission struct {
	ent.Schema
}

func (RequestSubmission) Fields() []ent.Field {
	return withDefaultFields(
		field.UUID("source_id", uuid.UUID{}),
		field.String("key"),
		field.Time("accepted_at").Optional().Nillable(),
		field.String("last_error").Optional(),
	)
}

func (RequestSubmission) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("source", Source.Type).Ref("submissions").Unique().Required().Field("source_id"),
	}
}

func (RequestSubmission) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("source_id", "key").Unique(),
		index.Fields("key"),
	}
}
