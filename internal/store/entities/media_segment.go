package entities

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

type MediaSegment struct {
	ent.Schema
}

func (MediaSegment) Fields() []ent.Field {
	return withDefaultFields(
		field.UUID("item_source_id", uuid.UUID{}),
		field.Enum("kind").Values("Unknown", "Commercial", "Preview", "Recap", "Outro", "Intro"),
		field.Enum("origin").Values("Chapter", "Fingerprint"),
		field.Int64("start_ticks"),
		field.Int64("end_ticks"),
	)
}

func (MediaSegment) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("source", ItemSource.Type).Ref("segments").Unique().Required().Field("item_source_id"),
	}
}

func (MediaSegment) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("item_source_id", "origin", "kind").Unique(),
	}
}
