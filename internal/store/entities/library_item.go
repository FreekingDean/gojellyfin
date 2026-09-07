package entities

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

type LibraryItem struct {
	ent.Schema
}

func (LibraryItem) Fields() []ent.Field {
	return withDefaultFields(
		field.UUID("library_id", uuid.UUID{}),
		field.UUID("item_id", uuid.UUID{}),
		field.UUID("source_id", uuid.UUID{}),
	)
}

func (LibraryItem) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("library", Library.Type).Ref("library_items").Unique().Required().Field("library_id"),
		edge.From("item", Item.Type).Ref("libraries").Unique().Required().Field("item_id"),
		edge.From("source", Source.Type).Ref("memberships").Unique().Required().Field("source_id"),
	}
}

func (LibraryItem) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("library_id", "item_id", "source_id").Unique(),
		index.Fields("item_id"),
	}
}
