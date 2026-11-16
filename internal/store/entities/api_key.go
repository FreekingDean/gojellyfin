package entities

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

type ApiKey struct {
	ent.Schema
}

func (ApiKey) Fields() []ent.Field {
	return withDefaultFields(
		field.String("access_token").Unique().Sensitive(),
		field.String("app_name"),
		field.Time("revoked_at").Optional().Nillable(),
		field.UUID("user_id", uuid.UUID{}).Optional().Nillable(),
	)
}

func (ApiKey) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).Ref("api_keys").Unique().Field("user_id"),
		edge.To("sessions", Session.Type).Annotations(cascadeOnDelete),
	}
}

func (ApiKey) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id"),
	}
}
