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
		field.Enum("kind").Values("radarr", "sonarr"),
		field.String("version").Optional(),
		field.Time("contacted_at").Optional().Nillable(),
		field.String("last_error").Optional(),
		field.Int("request_profile_id").Optional().Nillable(),
		field.String("request_root_folder").Optional(),
		field.String("request_tag").Optional(),
	)
}

func (Source) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("paths", SourcePath.Type).Annotations(cascadeOnDelete),
		edge.To("libraries", LibrarySource.Type).Annotations(cascadeOnDelete),
		edge.To("files", ItemSource.Type).Annotations(cascadeOnDelete),
		edge.To("memberships", LibraryItem.Type).Annotations(cascadeOnDelete),
		edge.To("submissions", RequestSubmission.Type).Annotations(cascadeOnDelete),
	}
}
