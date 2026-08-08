package entities

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type MediaStream struct {
	ent.Schema
}

func (MediaStream) Fields() []ent.Field {
	return withDefaultFields(
		field.Enum("kind").Values(
			"Audio", "Video", "Subtitle", "EmbeddedImage", "Data", "Lyric",
		),
		field.Enum("video_range").Optional().Values("Unknown", "SDR", "HDR"),
		field.Enum("video_range_type").Optional().Values(
			"Unknown", "SDR", "HDR10", "HLG", "DOVI", "DOVIWithHDR10",
			"DOVIWithHLG", "DOVIWithSDR", "HDR10Plus",
		),
		field.Enum("audio_spatial_format").Optional().Values("None", "DolbyAtmos", "DTSX"),

		field.Int32("index"),
		field.String("codec").Optional(),
		field.String("codec_tag").Optional(),
		field.String("profile").Optional(),
		field.String("language").Optional(),
		field.String("title").Optional(),
		field.String("comment").Optional(),
		field.String("path").Optional(),
		field.String("pixel_format").Optional(),
		field.String("aspect_ratio").Optional(),
		field.String("channel_layout").Optional(),
		field.String("time_base").Optional(),
		field.String("nal_length_size").Optional(),
		field.String("video_dovi_title").Optional(),

		field.String("color_range").Optional(),
		field.String("color_space").Optional(),
		field.String("color_transfer").Optional(),
		field.String("color_primaries").Optional(),

		field.Int32("dv_version_major").Optional(),
		field.Int32("dv_version_minor").Optional(),
		field.Int32("dv_profile").Optional(),
		field.Int32("dv_level").Optional(),
		field.Int32("rpu_present_flag").Optional(),
		field.Int32("el_present_flag").Optional(),
		field.Int32("bl_present_flag").Optional(),
		field.Int32("dv_bl_signal_compatibility_id").Optional(),

		field.Int32("bit_rate").Optional(),
		field.Int32("bit_depth").Optional(),
		field.Int32("ref_frames").Optional(),
		field.Int32("packet_length").Optional(),
		field.Int32("channels").Optional(),
		field.Int32("sample_rate").Optional(),
		field.Int32("width").Optional(),
		field.Int32("height").Optional(),
		field.Int32("rotation").Optional(),
		field.Int32("score").Optional(),
		field.Float("level").Optional(),
		field.Float("average_frame_rate").Optional(),
		field.Float("real_frame_rate").Optional(),
		field.Float("reference_frame_rate").Optional(),

		field.Bool("is_default"),
		field.Bool("is_forced"),
		field.Bool("is_external"),
		field.Bool("is_interlaced"),
		field.Bool("is_anamorphic"),
		field.Bool("is_avc"),
		field.Bool("is_hearing_impaired"),
	)
}

func (MediaStream) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("source", MediaSource.Type).Ref("streams").Unique().Required(),
	}
}

func (MediaStream) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("index").Edges("source").Unique(),
	}
}
