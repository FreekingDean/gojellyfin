package entities

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

type MediaSource struct {
	ent.Schema
}

func (MediaSource) Fields() []ent.Field {
	protocols := []string{"File", "Http", "Rtmp", "Rtsp", "Udp", "Rtp", "Ftp"}

	return withDefaultFields(
		field.UUID("item_id", uuid.UUID{}),
		field.UUID("library_id", uuid.UUID{}),

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
		field.Time("date_modified").Optional(),
		field.Time("probed_at").Optional(),

		field.Bool("read_at_native_framerate").Default(false),
		field.Bool("ignore_dts").Default(false),
		field.Bool("ignore_index").Default(false),
		field.Bool("gen_pts_input").Default(false),
		field.Bool("has_segments").Default(false),

		field.Int32("default_audio_stream_index").Optional(),
		field.Int32("default_subtitle_stream_index").Optional(),

		field.JSON("formats", []string{}).Optional(),
	)
}

func (MediaSource) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("item", Item.Type).Ref("media_sources").Unique().Required().Field("item_id"),
		edge.From("library", Library.Type).Ref("media_sources").Unique().Required().Field("library_id"),
		edge.To("streams", MediaStream.Type).Annotations(cascadeOnDelete),
		edge.To("attachments", MediaAttachment.Type).Annotations(cascadeOnDelete),
	}
}

func (MediaSource) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("library_id", "path").Unique(),
	}
}
