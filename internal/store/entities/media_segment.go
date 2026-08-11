package entities

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

type MediaSegment struct {
	ent.Schema
}

func (MediaSegment) Fields() []ent.Field {
	return withDefaultFields(
		field.Enum("kind").Values(
			"Unknown", "Commercial", "Preview", "Recap", "Outro", "Intro",
		),
		field.Int64("start_ticks"),
		field.Int64("end_ticks"),
	)
}

func (MediaSegment) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("item", Item.Type).Ref("media_segments").Unique().Required(),
	}
}
