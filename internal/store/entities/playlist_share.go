package entities

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

type PlaylistShare struct {
	ent.Schema
}

func (PlaylistShare) Fields() []ent.Field {
	return withDefaultFields(
		field.UUID("playlist_id", uuid.UUID{}),
		field.UUID("user_id", uuid.UUID{}),
		field.Bool("can_edit"),
	)
}

func (PlaylistShare) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("playlist", Playlist.Type).Ref("shares").Unique().Required().Field("playlist_id"),
		edge.From("user", User.Type).Ref("playlist_shares").Unique().Required().Field("user_id"),
	}
}

func (PlaylistShare) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("playlist_id", "user_id").Unique(),
	}
}
