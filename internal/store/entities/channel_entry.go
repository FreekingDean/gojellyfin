package entities

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

type ChannelEntry struct {
	ent.Schema
}

func (ChannelEntry) Fields() []ent.Field {
	return withDefaultFields(
		field.UUID("channel_id", uuid.UUID{}),
		field.UUID("item_id", uuid.UUID{}),
		field.Int32("position"),
	)
}

func (ChannelEntry) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("channel", Channel.Type).Ref("entries").Unique().Required().Field("channel_id"),
		edge.From("item", Item.Type).Ref("channel_entries").Unique().Required().Field("item_id"),
	}
}

func (ChannelEntry) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("channel_id", "item_id").Unique(),
		index.Fields("item_id"),
	}
}
