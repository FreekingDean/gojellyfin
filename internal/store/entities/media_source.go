package entities

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

type MediaSource struct {
	ent.Schema
}

func (MediaSource) Fields() []ent.Field {
	protocols := []string{"File", "Http", "Rtmp", "Rtsp", "Udp", "Rtp", "Ftp"}

	return withDefaultFields(
		field.UUID("item_id", uuid.UUID{}),

		field.Enum("protocol").Values(protocols...).Default("File"),
		field.Enum("encoder_protocol").Optional().Values(protocols...),
		field.Enum("kind").Values("Default", "Grouping", "Placeholder").Default("Default"),
		field.Enum("timestamp").Optional().Values("None", "Zero", "Valid"),
		field.Enum("video_type").Optional().Values("VideoFile", "Iso", "Dvd", "BluRay"),
		field.Enum("iso_type").Optional().Values("Dvd", "BluRay"),
		field.Enum("video_3d_format").Optional().Values(
			"HalfSideBySide", "FullSideBySide", "FullTopAndBottom",
			"HalfTopAndBottom", "MVC",
		),

		field.String("name"),
		field.String("path"),
		field.String("encoder_path").Optional(),
		field.String("container").Optional(),
		field.Int64("size").Optional(),
		field.Int64("run_time_ticks").Optional(),
		field.Int32("bitrate").Optional(),

		field.Bool("is_remote").Default(false),
		field.Bool("is_infinite_stream").Default(false),
		field.Bool("supports_transcoding").Default(true),
		field.Bool("supports_direct_stream").Default(true),
		field.Bool("supports_direct_play").Default(true),
		field.Bool("supports_probing").Default(true),
		field.Bool("read_at_native_framerate").Default(false),
		field.Bool("ignore_dts").Default(false),
		field.Bool("ignore_index").Default(false),
		field.Bool("gen_pts_input").Default(false),
		field.Bool("requires_looping").Default(false),
		field.Bool("has_segments").Default(false),

		field.Int32("default_audio_stream_index").Optional(),
		field.Int32("default_subtitle_stream_index").Optional(),

		field.JSON("formats", []string{}).Optional(),
	)
}

func (MediaSource) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("item", Item.Type).Ref("media_sources").Unique().Required().Field("item_id"),
		edge.To("streams", MediaStream.Type),
		edge.To("attachments", MediaAttachment.Type),
	}
}
