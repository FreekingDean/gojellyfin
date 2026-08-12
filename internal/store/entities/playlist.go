package entities

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

type Playlist struct {
	ent.Schema
}

func (Playlist) Fields() []ent.Field {
	return withDefaultFields(
		field.UUID("item_id", uuid.UUID{}).Unique(),
		field.UUID("owner_id", uuid.UUID{}),
		field.Bool("open_access"),
	)
}

func (Playlist) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("item", Item.Type).Ref("playlist").Unique().Required().Field("item_id"),
		edge.From("owner", User.Type).Ref("playlists").Unique().Required().Field("owner_id"),
		edge.To("entries", PlaylistEntry.Type),
		edge.To("shares", PlaylistShare.Type),
	}
}
