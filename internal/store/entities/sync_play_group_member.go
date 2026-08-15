package entities

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

type SyncPlayGroupMember struct {
	ent.Schema
}

func (SyncPlayGroupMember) Fields() []ent.Field {
	return withDefaultFields(
		field.UUID("group_id", uuid.UUID{}),
		field.UUID("session_id", uuid.UUID{}),
	)
}

func (SyncPlayGroupMember) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("group", SyncPlayGroup.Type).Ref("members").Unique().Required().Field("group_id"),
		edge.From("session", Session.Type).Ref("sync_play_memberships").Unique().Required().Field("session_id"),
	}
}

func (SyncPlayGroupMember) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("session_id").Unique(),
	}
}
