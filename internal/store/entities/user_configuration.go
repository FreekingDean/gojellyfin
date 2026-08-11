package entities

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

type UserConfiguration struct {
	ent.Schema
}

func (UserConfiguration) Fields() []ent.Field {
	return withDefaultFields(
		field.String("cast_receiver_id").Optional(),

		field.String("audio_language_preference").Optional(),
		field.String("subtitle_language_preference").Optional(),
		field.Bool("play_default_audio_track").Default(true),
		field.Enum("subtitle_mode").Values(
			"Default", "Always", "OnlyForced", "None", "Smart",
		).Default("OnlyForced"),

		field.JSON("grouped_folders", []uuid.UUID{}).Optional().Default([]uuid.UUID{}),
		field.JSON("ordered_views", []uuid.UUID{}).Optional().Default([]uuid.UUID{}),
		field.JSON("latest_items_excludes", []uuid.UUID{}).Optional().Default([]uuid.UUID{}),
		field.JSON("my_media_excludes", []uuid.UUID{}).Optional().Default([]uuid.UUID{}),

		field.Bool("display_missing_episodes").Default(false),
		field.Bool("display_collections_view").Default(false),
		field.Bool("enable_local_password").Default(false),
		field.Bool("hide_played_in_latest").Default(true),
		field.Bool("remember_audio_selections").Default(true),
		field.Bool("remember_subtitle_selections").Default(true),
		field.Bool("enable_next_episode_auto_play").Default(true),
	)
}

func (UserConfiguration) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).Ref("configuration").Unique(),
	}
}
