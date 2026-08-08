package entities

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

type AccessSchedule struct {
	DayOfWeek string  `json:"day_of_week"`
	StartHour float64 `json:"start_hour"`
	EndHour   float64 `json:"end_hour"`
}

type UserPolicy struct {
	ent.Schema
}

func (UserPolicy) Fields() []ent.Field {
	ratings := []string{
		"Approved", "G", "PG", "PG-13", "R", "NC-17",
		"TV-Y", "TV-G", "TV-PG", "TV-14", "TV-MA",
	}

	return withDefaultFields(
		field.Bool("is_administrator"),
		field.Bool("is_hidden"),
		field.Bool("is_disabled"),

		field.Bool("enable_collection_management").Default(false),
		field.Bool("enable_subtitle_management").Default(false),
		field.Bool("enable_lyric_management").Default(false),
		field.Bool("enable_user_preference_access"),
		field.Bool("enable_remote_control_of_other_users"),
		field.Bool("enable_shared_device_control"),
		field.Bool("enable_remote_access"),
		field.Bool("enable_live_tv_management"),
		field.Bool("enable_live_tv_access"),
		field.Bool("enable_media_playback"),
		field.Bool("enable_audio_playback_transcoding"),
		field.Bool("enable_video_playback_transcoding"),
		field.Bool("enable_playback_remuxing"),
		field.Bool("force_remote_source_transcoding"),
		field.Bool("enable_content_deletion"),
		field.Bool("enable_content_downloading"),
		field.Bool("enable_sync_transcoding"),
		field.Bool("enable_media_conversion"),
		field.Bool("enable_public_sharing"),
		field.Bool("enable_all_devices"),
		field.Bool("enable_all_channels"),
		field.Bool("enable_all_folders"),

		field.Enum("max_parental_rating").Optional().Values(ratings...),
		field.Enum("max_parental_sub_rating").Optional().Values(ratings...),

		field.Int32("invalid_login_attempt_count"),
		field.Int32("login_attempts_before_lockout"),
		field.Int32("max_active_sessions"),
		field.Int32("remote_client_bitrate_limit"),

		field.JSON("allowed_tags", []string{}).Optional(),
		field.JSON("blocked_tags", []string{}).Optional(),
		field.JSON("access_schedules", []AccessSchedule{}).Optional(),
		field.JSON("enable_content_deletion_from_folders", []string{}).Optional(),
		field.JSON("enabled_devices", []string{}).Optional(),
		field.JSON("enabled_channels", []uuid.UUID{}).Optional(),
		field.JSON("enabled_folders", []uuid.UUID{}).Optional(),
		field.JSON("blocked_media_folders", []uuid.UUID{}).Optional(),
		field.JSON("blocked_channels", []uuid.UUID{}).Optional(),
		field.JSON("block_unrated_items", []string{}).Optional(),

		field.String("authentication_provider_id").NotEmpty(),
		field.String("password_reset_provider_id").NotEmpty(),
		field.Enum("sync_play_access").Values(
			"CreateAndJoinGroups", "JoinGroups", "None",
		),
	)
}

func (UserPolicy) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).Ref("policy").Unique(),
	}
}
