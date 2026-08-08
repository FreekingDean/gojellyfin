package entities

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
)

type ApiKey struct {
	ent.Schema
}

func (ApiKey) Fields() []ent.Field {
	return withDefaultFields(
		field.String("access_token").Unique().Sensitive(),
		field.String("app_name"),
		field.Time("revoked_at").Optional(),
	)
}
