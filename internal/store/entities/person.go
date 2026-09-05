package entities

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

type Person struct {
	ent.Schema
}

func (Person) Fields() []ent.Field {
	return withDefaultFields(
		field.String("name").Unique(),
	)
}

func (Person) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("credits", Credit.Type).Annotations(cascadeOnDelete),
	}
}
