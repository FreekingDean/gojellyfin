package entities

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

type PlaylistEntry struct {
	ent.Schema
}

func (PlaylistEntry) Fields() []ent.Field {
	return withDefaultFields(
		field.Int32("sort_order"),
	)
}

func (PlaylistEntry) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("playlist", Playlist.Type).Ref("entries").Unique().Required(),
		edge.From("item", Item.Type).Ref("playlist_entries").Unique().Required(),
	}
}
