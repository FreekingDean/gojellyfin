package entities

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

type Playlist struct {
	ent.Schema
}

func (Playlist) Fields() []ent.Field {
	return withDefaultFields(
		field.Bool("open_access"),
	)
}

func (Playlist) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("item", Item.Type).Ref("playlist").Unique().Required(),
		edge.To("entries", PlaylistEntry.Type),
		edge.To("shares", PlaylistShare.Type),
	}
}
