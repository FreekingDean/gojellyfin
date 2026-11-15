package entities

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

type ReportAttempt struct {
	UserID        *uuid.UUID    `json:"user_id,omitempty"`
	UserName      string        `json:"user_name,omitempty"`
	DeviceID      string        `json:"device_id,omitempty"`
	DeviceName    string        `json:"device_name,omitempty"`
	Client        string        `json:"client,omitempty"`
	ClientVersion string        `json:"client_version,omitempty"`
	RemoteAddress string        `json:"remote_address,omitempty"`
	Playback      *PlaybackPlan `json:"playback,omitempty"`
	StartTicks    int64         `json:"start_ticks,omitempty"`
	Build         string        `json:"build,omitempty"`
}

type Report struct {
	ent.Schema
}

func (Report) Fields() []ent.Field {
	return withDefaultFields(
		field.UUID("item_id", uuid.UUID{}),
		field.UUID("item_source_id", uuid.UUID{}).Optional().Nillable(),
		field.UUID("user_id", uuid.UUID{}).Optional().Nillable(),
		field.Enum("kind").Values("Unplayable", "Video", "Audio", "Subtitles", "Other"),
		field.Enum("origin").Values("User", "Delete", "Probe", "Playback"),
		field.String("note").Default(""),
		field.Enum("status").Values("Open", "Resolved", "Dismissed").Default("Open"),
		field.JSON("attempt", &ReportAttempt{}).Optional(),
	)
}

func (Report) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("item", Item.Type).Ref("reports").Unique().Required().Field("item_id"),
		edge.From("file", ItemSource.Type).Ref("reports").Unique().Field("item_source_id"),
		edge.From("user", User.Type).Ref("reports").Unique().Field("user_id"),
	}
}

func (Report) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("status", "created_at"),
		index.Fields("item_id"),
	}
}
