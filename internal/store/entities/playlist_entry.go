package entities

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

type PlaylistEntry struct {
	ent.Schema
}

func (PlaylistEntry) Fields() []ent.Field {
	return withDefaultFields(
		field.UUID("playlist_id", uuid.UUID{}),
		field.UUID("item_id", uuid.UUID{}),
		field.Int32("sort_order"),
	)
}

func (PlaylistEntry) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("playlist", Playlist.Type).Ref("entries").Unique().Required().Field("playlist_id"),
		edge.From("item", Item.Type).Ref("playlist_entries").Unique().Required().Field("item_id"),
	}
}

func (PlaylistEntry) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("playlist_id", "sort_order"),
	}
}
