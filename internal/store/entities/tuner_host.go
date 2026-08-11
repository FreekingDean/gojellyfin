package entities

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
)

type TunerHost struct {
	ent.Schema
}

func (TunerHost) Fields() []ent.Field {
	return withDefaultFields(
		field.String("kind"),
		field.String("url"),
		field.String("device_id").Optional(),
		field.String("friendly_name").Optional(),
		field.String("source").Optional(),
		field.String("user_agent").Optional(),
		field.Int32("tuner_count"),
		field.Int32("fallback_max_streaming_bitrate"),
		field.Bool("import_favorites_only"),
		field.Bool("allow_hw_transcoding"),
		field.Bool("allow_fmp4_transcoding_container"),
		field.Bool("allow_stream_sharing"),
		field.Bool("enable_stream_looping"),
		field.Bool("ignore_dts"),
	)
}
