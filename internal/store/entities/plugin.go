package entities

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
)

type Plugin struct {
	ent.Schema
}

func (Plugin) Fields() []ent.Field {
	return withDefaultFields(
		field.String("name"),
		field.String("version"),
		field.Text("description").Optional(),
		field.String("configuration_file_name").Optional(),
		field.Enum("status").Values(
			"Active", "Restart", "Deleted", "Superceded",
			"Malfunctioned", "NotSupported", "Disabled",
		),
		field.Bool("can_uninstall"),
		field.Bool("has_image"),
	)
}
