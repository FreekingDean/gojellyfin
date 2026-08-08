package entities

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

type MediaSource struct {
	ent.Schema
}

func (MediaSource) Fields() []ent.Field {
	protocols := []string{"File", "Http", "Rtmp", "Rtsp", "Udp", "Rtp", "Ftp"}

	return withDefaultFields(
		field.Enum("protocol").Values(protocols...),
		field.Enum("encoder_protocol").Optional().Values(protocols...),
		field.Enum("kind").Values("Default", "Grouping", "Placeholder"),
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

		field.Bool("is_remote"),
		field.Bool("is_infinite_stream"),
		field.Bool("supports_transcoding"),
		field.Bool("supports_direct_stream"),
		field.Bool("supports_direct_play"),
		field.Bool("supports_probing"),
		field.Bool("read_at_native_framerate"),
		field.Bool("ignore_dts"),
		field.Bool("ignore_index"),
		field.Bool("gen_pts_input"),
		field.Bool("requires_looping"),
		field.Bool("has_segments"),

		field.Int32("default_audio_stream_index").Optional(),
		field.Int32("default_subtitle_stream_index").Optional(),

		field.JSON("formats", []string{}).Optional(),
	)
}

func (MediaSource) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("item", Item.Type).Ref("media_sources").Unique().Required(),
		edge.To("streams", MediaStream.Type),
	}
}
