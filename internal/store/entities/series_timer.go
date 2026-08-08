package entities

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

type SeriesTimer struct {
	ent.Schema
}

func (SeriesTimer) Fields() []ent.Field {
	return withDefaultFields(
		field.String("name"),
		field.Text("overview").Optional(),
		field.String("service_name").Optional(),
		field.String("external_id").Optional(),
		field.String("channel_id").Optional(),
		field.String("external_channel_id").Optional(),
		field.String("program_id").Optional(),
		field.String("external_program_id").Optional(),

		field.Time("start_date"),
		field.Time("end_date"),
		field.Int32("priority"),
		field.Int32("pre_padding_seconds"),
		field.Int32("post_padding_seconds"),
		field.Bool("is_pre_padding_required"),
		field.Bool("is_post_padding_required"),

		field.Enum("keep_until").Values(
			"UntilDeleted", "UntilSpaceNeeded", "UntilWatched", "UntilDate",
		),
		field.Int32("keep_up_to"),
		field.Bool("record_any_time"),
		field.Bool("record_any_channel"),
		field.Bool("record_new_only"),
		field.Bool("skip_episodes_in_library"),

		field.JSON("days", []string{}).Optional(),
		field.Enum("day_pattern").Optional().Values("Daily", "Weekdays", "Weekends"),
	)
}

func (SeriesTimer) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("timers", Timer.Type),
	}
}
