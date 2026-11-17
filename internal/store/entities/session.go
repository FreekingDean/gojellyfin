package entities

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

type PlaybackPlan struct {
	Direct      bool       `json:"direct"`
	VideoDirect bool       `json:"video_direct"`
	AudioDirect bool       `json:"audio_direct"`
	Container   string     `json:"container,omitempty"`
	VideoCodec  string     `json:"video_codec,omitempty"`
	AudioCodec  string     `json:"audio_codec,omitempty"`
	Segmented   bool       `json:"segmented,omitempty"`
	Reasons     []string   `json:"reasons,omitempty"`
	StandsInFor *uuid.UUID `json:"stands_in_for,omitempty"`
}

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
		field.UUID("planned_source_id", uuid.UUID{}).Optional().Nillable(),
		field.JSON("playback_plan", &PlaybackPlan{}).Optional(),
		field.UUID("api_key_id", uuid.UUID{}).Optional().Nillable(),
		field.UUID("now_viewing_item_id", uuid.UUID{}).Optional().Nillable(),
		field.Time("viewing_at").Optional().Nillable(),
	)
}

func (Session) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).Ref("sessions").Unique().Required(),
		edge.From("device", Device.Type).Ref("sessions").Unique().Required(),
		edge.From("now_playing", Item.Type).Ref("playing_sessions").Unique().Field("now_playing_item_id"),
		edge.From("planned_source", ItemSource.Type).Ref("planning_sessions").Unique().Field("planned_source_id"),
		edge.From("api_key", ApiKey.Type).Ref("sessions").Unique().Field("api_key_id"),
		edge.From("now_viewing", Item.Type).Ref("viewing_sessions").Unique().Field("now_viewing_item_id"),
		edge.To("additional_users", User.Type),
		edge.To("sync_play_memberships", SyncPlayGroupMember.Type).Annotations(cascadeOnDelete),
	}
}

func (Session) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("now_playing_item_id"),
		index.Fields("planned_source_id"),
		index.Fields("api_key_id"),
		index.Fields("now_viewing_item_id"),
	}
}
