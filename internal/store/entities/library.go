package entities

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

type Library struct {
	ent.Schema
}

func (Library) Fields() []ent.Field {
	return withDefaultFields(
		field.String("name").Unique(),
		field.Enum("collection_type").Values(
			"movies", "tvshows", "music", "musicvideos",
			"homevideos", "boxsets", "books", "mixed",
		).Default("mixed"),
		field.JSON("locations", []string{}).Default([]string{}),
	)
}

func (Library) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("options", LibraryOptions.Type).Unique(),
		edge.To("items", Item.Type),
	}
}
