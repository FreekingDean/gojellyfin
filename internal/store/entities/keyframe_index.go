package entities

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

type KeyframeIndex struct {
	ent.Schema
}

func (KeyframeIndex) Fields() []ent.Field {
	return withDefaultFields(
		field.UUID("item_source_id", uuid.UUID{}).Unique(),
		field.JSON("ticks", []int64{}),
	)
}

func (KeyframeIndex) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("source", ItemSource.Type).Ref("keyframes").Unique().Required().Field("item_source_id"),
	}
}
