package entities

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type PlaylistShare struct {
	ent.Schema
}

func (PlaylistShare) Fields() []ent.Field {
	return withDefaultFields(
		field.Bool("can_edit"),
	)
}

func (PlaylistShare) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("playlist", Playlist.Type).Ref("shares").Unique().Required(),
		edge.From("user", User.Type).Ref("playlist_shares").Unique().Required(),
	}
}

func (PlaylistShare) Indexes() []ent.Index {
	return []ent.Index{
		index.Edges("playlist", "user").Unique(),
	}
}
