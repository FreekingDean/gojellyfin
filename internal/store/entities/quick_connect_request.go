package entities

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

type QuickConnectRequest struct {
	ent.Schema
}

func (QuickConnectRequest) Fields() []ent.Field {
	return withDefaultFields(
		field.String("secret").Unique().Sensitive(),
		field.String("code").Unique(),
		field.String("device_id"),
		field.String("device_name"),
		field.String("app_name"),
		field.String("app_version"),
		field.Time("expires_at"),
		field.UUID("authorized_by_id", uuid.UUID{}).Optional(),
	)
}

func (QuickConnectRequest) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("authorized_by", User.Type).Ref("quick_connect_requests").Unique().Field("authorized_by_id"),
	}
}
