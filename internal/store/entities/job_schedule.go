package entities

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
)

type JobTrigger struct {
	Type            string  `json:"type"`
	DayOfWeek       *string `json:"day_of_week,omitempty"`
	IntervalTicks   *int64  `json:"interval_ticks,omitempty"`
	TimeOfDayTicks  *int64  `json:"time_of_day_ticks,omitempty"`
	MaxRuntimeTicks *int64  `json:"max_runtime_ticks,omitempty"`
}

type JobSchedule struct {
	ent.Schema
}

func (JobSchedule) Fields() []ent.Field {
	return withDefaultFields(
		field.String("kind").Unique(),
		field.JSON("triggers", []JobTrigger{}).Default([]JobTrigger{}),
	)
}
