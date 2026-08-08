package entities

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

func withDefaultFields(fields ...ent.Field) []ent.Field {
	return append([]ent.Field{
		field.UUID("id", uuid.UUID{}).Annotations(
			entsql.Default("gen_random_uuid()"),
		),
		field.Time("created_at").Default(time.Now),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}, fields...)
}
