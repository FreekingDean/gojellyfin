package entities

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

type ActivityLogEntry struct {
	ent.Schema
}

func (ActivityLogEntry) Fields() []ent.Field {
	return withDefaultFields(
		field.String("name"),
		field.String("kind"),
		field.Text("overview").Optional(),
		field.Text("short_overview").Optional(),
		field.Enum("severity").Values(
			"Trace", "Debug", "Information", "Warning", "Error", "Critical", "None",
		),
	)
}

func (ActivityLogEntry) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).Ref("activity_log_entries").Unique(),
		edge.From("item", Item.Type).Ref("activity_log_entries").Unique(),
	}
}
