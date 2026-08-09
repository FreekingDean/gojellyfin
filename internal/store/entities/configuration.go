package entities

import (
	"encoding/json"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
)

type Configuration struct {
	ent.Schema
}

func (Configuration) Fields() []ent.Field {
	return withDefaultFields(
		field.String("key").Unique(),
		field.JSON("value", json.RawMessage{}),
	)
}
