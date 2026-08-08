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
		field.Bool("play_default_audio_track"),
		field.Enum("subtitle_mode").Values(
			"Default", "Always", "OnlyForced", "None", "Smart",
		),

		field.JSON("grouped_folders", []uuid.UUID{}).Optional(),
		field.JSON("ordered_views", []uuid.UUID{}).Optional(),
		field.JSON("latest_items_excludes", []uuid.UUID{}).Optional(),
		field.JSON("my_media_excludes", []uuid.UUID{}).Optional(),

		field.Bool("display_missing_episodes"),
		field.Bool("display_collections_view"),
		field.Bool("enable_local_password"),
		field.Bool("hide_played_in_latest"),
		field.Bool("remember_audio_selections"),
		field.Bool("remember_subtitle_selections"),
		field.Bool("enable_next_episode_auto_play"),
	)
}

func (UserConfiguration) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).Ref("configuration").Unique(),
	}
}
