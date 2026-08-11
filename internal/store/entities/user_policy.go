package entities

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/consts"
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
		field.Bool("is_administrator").Default(false),
		field.Bool("is_hidden").Default(false),
		field.Bool("is_disabled").Default(false),

		field.Bool("enable_collection_management").Default(false),
		field.Bool("enable_subtitle_management").Default(true),
		field.Bool("enable_lyric_management").Default(false),
		field.Bool("enable_user_preference_access").Default(true),
		field.Bool("enable_remote_control_of_other_users").Default(false),
		field.Bool("enable_shared_device_control").Default(true),
		field.Bool("enable_remote_access").Default(true),
		field.Bool("enable_live_tv_management").Default(false),
		field.Bool("enable_live_tv_access").Default(false),
		field.Bool("enable_media_playback").Default(true),
		field.Bool("enable_audio_playback_transcoding").Default(true),
		field.Bool("enable_video_playback_transcoding").Default(true),
		field.Bool("enable_playback_remuxing").Default(true),
		field.Bool("force_remote_source_transcoding").Default(false),
		field.Bool("enable_content_deletion").Default(false),
		field.Bool("enable_content_downloading").Default(true),
		field.Bool("enable_sync_transcoding").Default(true),
		field.Bool("enable_media_conversion").Default(true),
		field.Bool("enable_public_sharing").Default(true),
		field.Bool("enable_all_devices").Default(true),
		field.Bool("enable_all_channels").Default(true),
		field.Bool("enable_all_folders").Default(true),

		field.Enum("max_parental_rating").Optional().Values(ratings...),
		field.Enum("max_parental_sub_rating").Optional().Values(ratings...),

		field.Int32("invalid_login_attempt_count").Default(0),
		field.Int32("login_attempts_before_lockout").Default(-1),
		field.Int32("max_active_sessions").Default(0),
		field.Int32("remote_client_bitrate_limit").Default(0),

		field.JSON("allowed_tags", []string{}).Optional().Default([]string{}),
		field.JSON("blocked_tags", []string{}).Optional().Default([]string{}),
		field.JSON("access_schedules", []AccessSchedule{}).Optional().Default([]AccessSchedule{}),
		field.JSON("enable_content_deletion_from_folders", []string{}).Optional().Default([]string{}),
		field.JSON("enabled_devices", []string{}).Optional().Default([]string{}),
		field.JSON("enabled_channels", []uuid.UUID{}).Optional().Default([]uuid.UUID{}),
		field.JSON("enabled_folders", []uuid.UUID{}).Optional().Default([]uuid.UUID{}),
		field.JSON("blocked_media_folders", []uuid.UUID{}).Optional().Default([]uuid.UUID{}),
		field.JSON("blocked_channels", []uuid.UUID{}).Optional().Default([]uuid.UUID{}),
		field.JSON("block_unrated_items", []string{}).Optional().Default([]string{}),

		field.String("authentication_provider_id").NotEmpty().
			Default(consts.DefaultProviderFor("Users", "DefaultAuthenticationProvider")),
		field.String("password_reset_provider_id").NotEmpty().
			Default(consts.DefaultProviderFor("Users", "DefaultPasswordResetProvider")),
		field.Enum("sync_play_access").Values(
			"CreateAndJoinGroups", "JoinGroups", "None",
		).Default("CreateAndJoinGroups"),
	)
}

func (UserPolicy) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).Ref("policy").Unique(),
	}
}
