package entities

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

type SyncPlayQueueEntry struct {
	ent.Schema
}

func (SyncPlayQueueEntry) Fields() []ent.Field {
	return withDefaultFields(
		field.UUID("group_id", uuid.UUID{}),
		field.UUID("item_id", uuid.UUID{}),
		field.Int("sorted_index"),
		field.Int("shuffled_index").Optional().Nillable(),
	)
}

func (SyncPlayQueueEntry) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("group", SyncPlayGroup.Type).Ref("queue").Unique().Required().Field("group_id"),
		edge.From("item", Item.Type).Ref("sync_play_entries").Unique().Required().Field("item_id"),
	}
}

func (SyncPlayQueueEntry) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("group_id", "sorted_index"),
		index.Fields("item_id"),
	}
}
