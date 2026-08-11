package entities

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

type UserItemData struct {
	ent.Schema
}

func (UserItemData) Fields() []ent.Field {
	return withDefaultFields(
		field.UUID("user_id", uuid.UUID{}),
		field.UUID("item_id", uuid.UUID{}),

		field.Bool("played").Default(false),
		field.Bool("is_favorite").Default(false),
		field.Int32("play_count").Default(0),
		field.Int64("playback_position_ticks").Default(0),
		field.Float("rating").Optional().Nillable(),
		field.Bool("likes").Optional(),
		field.Time("last_played_at").Optional().Nillable(),
	)
}

func (UserItemData) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).Ref("item_data").Unique().Required().Field("user_id"),
		edge.From("item", Item.Type).Ref("user_data").Unique().Required().Field("item_id"),
	}
}

func (UserItemData) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id", "item_id").Unique(),
	}
}
