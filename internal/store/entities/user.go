package entities

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

type User struct {
	ent.Schema
}

func (User) Fields() []ent.Field {
	return withDefaultFields(
		field.String("name"),
		field.String("username"),
		field.String("password_hash").Sensitive(),
	)
}

// Edges of the User.
func (User) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("configuration", UserConfiguration.Type),
		edge.To("policy", UserPolicy.Type),
		edge.To("sessions", Session.Type),
		edge.To("item_data", UserItemData.Type),
		edge.To("display_preferences", DisplayPreferences.Type),
	}
}
