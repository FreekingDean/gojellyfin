package entities

import (
	"fmt"
	"strings"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

type Item struct {
	ent.Schema
}

const playlistKind = "Playlist"

var itemKinds = []string{"Movie", "Series", "Season", "Episode", playlistKind, "MusicArtist", "MusicAlbum", "Audio"}

func (Item) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Checks(map[string]string{
			"item_kind_known":          fmt.Sprintf("kind IN ('%s')", strings.Join(itemKinds, "', '")),
			"item_key_unless_playlist": fmt.Sprintf("kind = '%s' OR key IS NOT NULL", playlistKind),
		}),
	}
}

func (Item) Fields() []ent.Field {
	return withDefaultFields(
		field.UUID("parent_id", uuid.UUID{}).Optional().Nillable(),

		field.Enum("kind").Values(itemKinds...),

		field.String("key").Optional(),
		field.String("name"),
		field.String("sort_name").Optional(),
		field.Time("deleted_at").Optional().Nillable(),
		field.Text("overview").Optional(),

		field.Bool("lock_data").Default(false),

		field.Time("premiere_date").Optional().Nillable(),
		field.Time("end_date").Optional().Nillable(),
		field.Time("date_modified").Optional().Nillable(),
		field.Int32("production_year").Optional().Nillable(),

		field.String("official_rating").Optional(),
		field.Float("community_rating").Optional().Nillable(),

		field.Int64("run_time_ticks").Optional().Nillable(),
		field.Int32("index_number").Optional().Nillable(),
		field.Int32("parent_index_number").Optional().Nillable(),

		field.String("status").Optional(),

		field.JSON("provider_ids", map[string]string{}).Optional(),
		field.JSON("tags", []string{}).Optional(),
		field.JSON("taglines", []string{}).Optional(),
		field.JSON("locked_fields", []string{}).Optional(),
	)
}

func (Item) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("children", Item.Type).Annotations(cascadeOnDelete).From("parent").Unique().Field("parent_id"),
		edge.To("libraries", LibraryItem.Type).Annotations(cascadeOnDelete),
		edge.To("item_sources", ItemSource.Type).Annotations(cascadeOnDelete),
		edge.To("reports", Report.Type).Annotations(cascadeOnDelete),
		edge.To("credits", Credit.Type).Annotations(cascadeOnDelete),
		edge.To("images", Image.Type).Annotations(cascadeOnDelete),
		edge.To("user_data", UserItemData.Type).Annotations(cascadeOnDelete),
		edge.To("activity_log_entries", ActivityLogEntry.Type),
		edge.To("playing_sessions", Session.Type),
		edge.To("viewing_sessions", Session.Type),
		edge.To("playlist", Playlist.Type).Unique().Annotations(cascadeOnDelete),
		edge.To("playlist_entries", PlaylistEntry.Type).Annotations(cascadeOnDelete),
		edge.To("sync_play_entries", SyncPlayQueueEntry.Type).Annotations(cascadeOnDelete),
		edge.To("channel_entries", ChannelEntry.Type).Annotations(cascadeOnDelete),
		edge.To("genres", Genre.Type),
		edge.To("studios", Studio.Type),
	}
}

func (Item) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("key").Unique(),
		index.Fields("kind", "sort_name"),
		index.Fields("deleted_at"),
		index.Fields("parent_id"),
	}
}
