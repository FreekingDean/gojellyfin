package entities

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

type Report struct {
	ent.Schema
}

func (Report) Fields() []ent.Field {
	return withDefaultFields(
		field.UUID("item_id", uuid.UUID{}),
		field.UUID("item_source_id", uuid.UUID{}).Optional().Nillable(),
		field.UUID("user_id", uuid.UUID{}).Optional().Nillable(),
		field.Enum("kind").Values("Unplayable", "Video", "Audio", "Subtitles", "Other"),
		field.Enum("origin").Values("User", "Delete", "Probe", "Playback"),
		field.String("note").Default(""),
		field.Enum("status").Values("Open", "Resolved", "Dismissed").Default("Open"),
	)
}

func (Report) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("item", Item.Type).Ref("reports").Unique().Required().Field("item_id"),
		edge.From("file", ItemSource.Type).Ref("reports").Unique().Field("item_source_id"),
		edge.From("user", User.Type).Ref("reports").Unique().Field("user_id"),
	}
}

func (Report) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("status", "created_at"),
		index.Fields("item_id"),
	}
}
