package entities

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

type Device struct {
	ent.Schema
}

func (Device) Fields() []ent.Field {
	return withDefaultFields(
		field.String("client_id").Unique(),
		field.String("name"),
		field.String("custom_name").Optional(),
		field.String("app_name"),
		field.String("app_version"),
		field.String("icon_url").Optional(),

		field.JSON("playable_media_types", []string{}).Optional(),
		field.JSON("supported_commands", []string{}).Optional(),
		field.JSON("profile", map[string]any{}).Optional(),

		field.Bool("supports_media_control"),
		field.Bool("supports_persistent_identifier"),

		field.Time("last_activity_at").Optional(),
	)
}

func (Device) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("sessions", Session.Type),
	}
}
