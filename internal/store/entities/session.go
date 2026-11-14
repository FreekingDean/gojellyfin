package entities

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

type Session struct {
	ent.Schema
}

func (Session) Fields() []ent.Field {
	return withDefaultFields(
		field.String("access_token").Unique().Sensitive(),
		field.String("remote_endpoint").Optional(),
		field.Time("last_activity_at").Optional().Nillable(),
		field.Time("revoked_at").Optional().Nillable(),
		field.UUID("now_playing_item_id", uuid.UUID{}).Optional().Nillable(),
		field.Int64("position_ticks").Default(0),
		field.String("play_method").Optional(),
		field.Bool("is_paused").Default(false),
		field.Time("playing_at").Optional().Nillable(),
	)
}

func (Session) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).Ref("sessions").Unique().Required(),
		edge.From("device", Device.Type).Ref("sessions").Unique().Required(),
		edge.From("now_playing", Item.Type).Ref("playing_sessions").Unique().Field("now_playing_item_id"),
		edge.To("sync_play_memberships", SyncPlayGroupMember.Type).Annotations(cascadeOnDelete),
	}
}
