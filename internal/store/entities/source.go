package entities

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

type Source struct {
	ent.Schema
}

func (Source) Fields() []ent.Field {
	return withDefaultFields(
		field.String("name").Unique(),
		field.String("url").Unique(),
		field.String("api_key_variable"),
		field.String("root_path").Optional(),
		field.String("local_path").Optional(),
		field.Enum("kind").Values("radarr", "sonarr", "lidarr", "readarr", "bazarr"),
	)
}

func (Source) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("libraries", LibrarySource.Type).Annotations(cascadeOnDelete),
		edge.To("files", ItemSource.Type).Annotations(cascadeOnDelete),
		edge.To("memberships", LibraryItem.Type).Annotations(cascadeOnDelete),
	}
}
