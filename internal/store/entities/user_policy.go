package entities

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

type UserPolicy struct {
	ent.Schema
}

func (UserPolicy) Fields() []ent.Field {
	languages := []string{"English", "Spanish"}
	return withDefaultFields(
		field.String("cast_receiver_id"),

		field.Enum("audio_language_preference").Values(languages...),
		field.Bool("play_default_audio_track"),
		field.Enum("subtitle_language_preference").Values(languages...),
		field.Enum("subtitle_mode").Values(
			"Default", "Always", "OnlyForced", "None", "Smart",
		),

		field.JSON("gropued_folders", []string{}),
		field.JSON("ordered_views", []string{}),
		field.JSON("latest_items_excludes", []string{}),
		field.JSON("my_media_excludes", []string{}),

		field.Bool("display_missing_episodes"),
		field.Bool("display_collections_view"),
		field.Bool("enabled_local_password"),
		field.Bool("hide_played_in_latest"),
		field.Bool("remember_audio_selections"),
		field.Bool("enable_next_episode_auto_play"),
	)
}

func (UserPolicy) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).Ref("policy").Unique(),
	}
}
