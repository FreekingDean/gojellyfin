package entities

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

var syncPlayStates = []string{"Idle", "Waiting", "Paused", "Playing"}

type SyncPlayGroup struct {
	ent.Schema
}

func (SyncPlayGroup) Fields() []ent.Field {
	return withDefaultFields(
		field.String("name"),
		field.Enum("state").Values(syncPlayStates...).Default("Idle"),
		field.Int64("position_ticks").Default(0),
		field.Int64("run_time_ticks").Default(0),
		field.Time("last_activity").Default(time.Now),
		field.Int("playing_index").Default(-1),
		field.Enum("shuffle_mode").Values("Sorted", "Shuffle").Default("Sorted"),
		field.Enum("repeat_mode").Values("RepeatOne", "RepeatAll", "RepeatNone").Default("RepeatNone"),
		field.Time("queue_changed_at").Default(time.Now),
		field.Bool("resume_playing").Default(false),
		field.Enum("waiting_from").Values(syncPlayStates...).Optional().Nillable(),
		field.Bool("position_jumped").Default(false),
		field.Bool("ignore_buffering").Default(false),
		field.Time("wait_deadline").Optional().Nillable(),
	)
}

func (SyncPlayGroup) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("members", SyncPlayGroupMember.Type).Annotations(cascadeOnDelete),
		edge.To("queue", SyncPlayQueueEntry.Type).Annotations(cascadeOnDelete),
	}
}

func (SyncPlayGroup) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("wait_deadline"),
	}
}
