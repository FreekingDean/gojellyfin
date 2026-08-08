package entities

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

type Timer struct {
	ent.Schema
}

func (Timer) Fields() []ent.Field {
	return withDefaultFields(
		field.String("name"),
		field.Text("overview").Optional(),
		field.String("service_name").Optional(),
		field.String("external_id").Optional(),
		field.String("channel_id").Optional(),
		field.String("external_channel_id").Optional(),
		field.String("program_id").Optional(),
		field.String("external_program_id").Optional(),
		field.String("external_series_timer_id").Optional(),

		field.Time("start_date"),
		field.Time("end_date"),
		field.Int64("run_time_ticks").Optional(),
		field.Int32("priority"),
		field.Int32("pre_padding_seconds"),
		field.Int32("post_padding_seconds"),
		field.Bool("is_pre_padding_required"),
		field.Bool("is_post_padding_required"),

		field.Enum("keep_until").Values(
			"UntilDeleted", "UntilSpaceNeeded", "UntilWatched", "UntilDate",
		),
		field.Enum("status").Values(
			"New", "InProgress", "Completed", "Cancelled",
			"ConflictedOk", "ConflictedNotOk", "Error",
		),
	)
}

func (Timer) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("series_timer", SeriesTimer.Type).Ref("timers").Unique(),
	}
}
