package entities

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

type SourcePath struct {
	ent.Schema
}

func (SourcePath) Fields() []ent.Field {
	return withDefaultFields(
		field.UUID("source_id", uuid.UUID{}),
		field.String("remote_path"),
		field.String("local_path").Unique(),
	)
}

func (SourcePath) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("source", Source.Type).Ref("paths").Unique().Required().Field("source_id"),
	}
}

func (SourcePath) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("source_id", "remote_path").Unique(),
	}
}
