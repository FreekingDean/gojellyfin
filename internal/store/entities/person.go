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
		field.Text("overview").Optional(),
		field.Time("birth_date").Optional(),
		field.Time("death_date").Optional(),
		field.JSON("provider_ids", map[string]string{}).Optional(),
	)
}

func (Person) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("credits", Credit.Type).Annotations(cascadeOnDelete),
	}
}
