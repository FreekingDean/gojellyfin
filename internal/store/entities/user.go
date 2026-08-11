package entities

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

type User struct {
	ent.Schema
}

func (User) Fields() []ent.Field {
	return withDefaultFields(
		field.String("name"),
		field.String("username").Unique(),
		field.String("password_hash").Sensitive(),
		field.Time("last_login_at").Optional(),
		field.Time("last_activity_at").Optional(),
	)
}

// Edges of the User.
func (User) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("configuration", UserConfiguration.Type).Unique().Annotations(cascadeOnDelete),
		edge.To("policy", UserPolicy.Type).Unique().Annotations(cascadeOnDelete),
		edge.To("sessions", Session.Type).Annotations(cascadeOnDelete),
		edge.To("item_data", UserItemData.Type).Annotations(cascadeOnDelete),
		edge.To("display_preferences", DisplayPreferences.Type).Annotations(cascadeOnDelete),
		edge.To("activity_log_entries", ActivityLogEntry.Type),
		edge.To("playlists", Playlist.Type).Annotations(cascadeOnDelete),
		edge.To("playlist_shares", PlaylistShare.Type).Annotations(cascadeOnDelete),
	}
}
