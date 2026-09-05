package entities

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

type LibrarySource struct {
	ent.Schema
}

func (LibrarySource) Fields() []ent.Field {
	return withDefaultFields(
		field.UUID("library_id", uuid.UUID{}),
		field.UUID("source_id", uuid.UUID{}),
		field.String("tag_filter").Optional(),
		field.String("source_path").Optional(),
		field.String("target_path").Optional(),
	)
}

func (LibrarySource) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("library", Library.Type).Ref("sources").Unique().Required().Field("library_id"),
		edge.From("source", Source.Type).Ref("libraries").Unique().Required().Field("source_id"),
	}
}

func (LibrarySource) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("library_id", "source_id").Unique(),
		index.Fields("source_id"),
	}
}
