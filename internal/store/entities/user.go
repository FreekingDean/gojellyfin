package entities

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type User struct {
	ent.Schema
}

func (User) Fields() []ent.Field {
	return withDefaultFields(
		field.String("name"),
		field.String("username").Unique(),
		field.String("password_hash").Sensitive(),
		field.Time("last_login_at").Optional().Nillable(),
		field.Time("last_activity_at").Optional().Nillable(),
		field.String("oidc_issuer").Optional().Nillable(),
		field.String("oidc_subject").Optional().Nillable(),
		field.Bool("can_request").Default(true),
		field.Bool("requests_need_approval").Default(false),
		field.Int("request_quota").Optional().Nillable(),
	)
}

func (User) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("oidc_issuer", "oidc_subject").Unique(),
	}
}

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
		edge.To("quick_connect_requests", QuickConnectRequest.Type).Annotations(cascadeOnDelete),
		edge.To("requests", Request.Type).Annotations(cascadeOnDelete),
		edge.To("reports", Report.Type),
		edge.To("api_keys", ApiKey.Type).Annotations(cascadeOnDelete),
	}
}
