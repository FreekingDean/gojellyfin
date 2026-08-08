package entities

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

type Trickplay struct {
	ent.Schema
}

func (Trickplay) Fields() []ent.Field {
	return withDefaultFields(
		field.Int32("width"),
		field.Int32("height"),
		field.Int32("tile_width"),
		field.Int32("tile_height"),
		field.Int32("thumbnail_count"),
		field.Int32("interval"),
		field.Int32("bandwidth"),
	)
}

func (Trickplay) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("item", Item.Type).Ref("trickplays").Unique().Required(),
	}
}
