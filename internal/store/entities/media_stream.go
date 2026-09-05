package entities

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

type MediaStream struct {
	ent.Schema
}

func (MediaStream) Fields() []ent.Field {
	return withDefaultFields(
		field.UUID("source_id", uuid.UUID{}),

		field.Enum("kind").Values(
			"Audio", "Video", "Subtitle", "EmbeddedImage", "Data", "Lyric",
		),
		field.Enum("video_range_type").Optional().Values(
			"Unknown", "SDR", "HDR10", "HLG", "DOVI", "DOVIWithHDR10",
			"DOVIWithHLG", "DOVIWithSDR", "HDR10Plus",
		),

		field.Int32("index").Default(0),
		field.String("codec").Optional(),
		field.String("profile").Optional(),
		field.String("language").Optional(),
		field.String("title").Optional(),
		field.String("path").Optional(),
		field.String("pixel_format").Optional(),

		field.Int32("bit_rate").Optional(),
		field.Int32("channels").Optional(),
		field.Int32("sample_rate").Optional(),
		field.Int32("width").Optional(),
		field.Int32("height").Optional(),
		field.Float("level").Optional(),

		field.Bool("is_default").Default(false),
		field.Bool("is_forced").Default(false),
		field.Bool("is_external").Default(false),
		field.Bool("is_interlaced").Default(false),
		field.Bool("is_anamorphic").Default(false),
		field.Bool("is_hearing_impaired").Default(false),
	)
}

func (MediaStream) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("source", MediaSource.Type).Ref("streams").Unique().Required().Field("source_id"),
	}
}

func (MediaStream) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("source_id", "index").Unique(),
	}
}
