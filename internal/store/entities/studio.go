package entities

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

type Studio struct {
	ent.Schema
}

func (Studio) Fields() []ent.Field {
	return withDefaultFields(
		field.String("name").Unique(),
		field.JSON("provider_ids", map[string]string{}).Optional(),
	)
}

func (Studio) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("items", Item.Type).Ref("studios"),
	}
}
