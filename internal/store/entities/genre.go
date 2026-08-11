package entities

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

type Genre struct {
	ent.Schema
}

func (Genre) Fields() []ent.Field {
	return withDefaultFields(
		field.String("name").Unique(),
	)
}

func (Genre) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("items", Item.Type).Ref("genres"),
	}
}
