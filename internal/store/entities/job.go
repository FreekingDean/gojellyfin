package entities

import (
	"encoding/json"
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type Job struct {
	ent.Schema
}

func (Job) Fields() []ent.Field {
	return withDefaultFields(
		field.String("kind"),
		field.Enum("state").Values(
			"Queued", "Running", "Succeeded", "Failed", "Cancelled",
		).Default("Queued"),
		field.JSON("payload", json.RawMessage{}).Optional(),
		field.String("dedupe_key").Optional().Unique(),
		field.Time("run_at").Default(time.Now),
		field.Int("attempt").Default(0),
		field.Int("max_attempts").Default(3),
		field.Float("progress").Default(0),
		field.Bool("cancel_requested").Default(false),
		field.String("worker").Optional(),
		field.Time("lease_expires_at").Optional(),
		field.Time("started_at").Optional(),
		field.Time("finished_at").Optional(),
		field.Text("error_message").Optional(),
	)
}

func (Job) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("state", "run_at"),
		index.Fields("kind", "state"),
	}
}
