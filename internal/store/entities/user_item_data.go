package entities

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type UserItemData struct {
	ent.Schema
}

func (UserItemData) Fields() []ent.Field {
	return withDefaultFields(
		field.Bool("played"),
		field.Bool("is_favorite"),
		field.Int32("play_count"),
		field.Int64("playback_position_ticks"),
		field.Float("rating").Optional(),
		field.Bool("likes").Optional(),
		field.Time("last_played_at").Optional(),
	)
}

func (UserItemData) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).Ref("item_data").Unique().Required(),
		edge.From("item", Item.Type).Ref("user_data").Unique().Required(),
	}
}

func (UserItemData) Indexes() []ent.Index {
	return []ent.Index{
		index.Edges("user", "item").Unique(),
	}
}
