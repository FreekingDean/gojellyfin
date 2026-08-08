package entities

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

type Chapter struct {
	ent.Schema
}

func (Chapter) Fields() []ent.Field {
	return withDefaultFields(
		field.String("name").Optional(),
		field.Int64("start_position_ticks"),
		field.String("image_path").Optional(),
		field.Time("image_modified_at").Optional(),
	)
}

func (Chapter) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("item", Item.Type).Ref("chapters").Unique().Required(),
	}
}
