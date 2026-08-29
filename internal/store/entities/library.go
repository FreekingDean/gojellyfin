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
		field.String("image_tag").Default(""),
	)
}

func (Library) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("options", LibraryOptions.Type).Unique().Annotations(cascadeOnDelete),
		edge.To("items", Item.Type).Annotations(cascadeOnDelete),
		edge.To("media_sources", MediaSource.Type).Annotations(cascadeOnDelete),
	}
}
