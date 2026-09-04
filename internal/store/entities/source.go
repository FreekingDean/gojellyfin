package entities

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
)

type Source struct {
	ent.Schema
}

type SourcePathMapping struct {
	SourcePath string `json:"source_path"`
	TargetPath string `json:"target_path"`
}

type SourceLibrary struct {
	Name      string `json:"name"`
	ID        string `json:"id"`
	TagFilter string `json:"tag_filter"`
}

func (Source) Fields() []ent.Field {
	return withDefaultFields(
		field.String("name").Unique(),
		field.String("url").Unique(),
		field.String("api_key"),
		field.JSON("path_mappings", []SourcePathMapping{}).Optional(),
		field.JSON("libraries", []SourceLibrary{}).Optional(),
		field.Enum("kind").Values("radarr", "sonarr", "lidarr", "readarr", "bazarr"),
	)
}

func (Source) Edges() []ent.Edge {
	return []ent.Edge{}
}
