package entities

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

type SyncPlayGroup struct {
	ent.Schema
}

func (SyncPlayGroup) Fields() []ent.Field {
	return withDefaultFields(
		field.String("name"),
		field.Enum("state").Values("Idle", "Waiting", "Paused", "Playing").Default("Idle"),
	)
}

func (SyncPlayGroup) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("members", SyncPlayGroupMember.Type).Annotations(cascadeOnDelete),
	}
}
