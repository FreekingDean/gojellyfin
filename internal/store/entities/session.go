package entities

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

type Session struct {
	ent.Schema
}

func (Session) Fields() []ent.Field {
	return withDefaultFields(
		field.String("access_token").Unique().Sensitive(),
		field.String("remote_endpoint").Optional(),
		field.Time("last_activity_at").Optional(),
		field.Time("revoked_at").Optional(),
	)
}

func (Session) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).Ref("sessions").Unique().Required(),
		edge.From("device", Device.Type).Ref("sessions").Unique().Required(),
		edge.To("sync_play_memberships", SyncPlayGroupMember.Type).Annotations(cascadeOnDelete),
	}
}
