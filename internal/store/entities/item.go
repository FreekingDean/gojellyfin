package entities

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

type Item struct {
	ent.Schema
}

func (Item) Fields() []ent.Field {
	return withDefaultFields(
		field.UUID("library_id", uuid.UUID{}).Optional(),
		field.UUID("parent_id", uuid.UUID{}).Optional().Nillable(),

		field.Enum("kind").Values(
			"AggregateFolder", "Audio", "AudioBook", "BasePluginFolder", "Book",
			"BoxSet", "Channel", "ChannelFolderItem", "CollectionFolder", "Episode",
			"Folder", "Genre", "ManualPlaylistsFolder", "Movie", "LiveTvChannel",
			"LiveTvProgram", "MusicAlbum", "MusicArtist", "MusicGenre", "MusicVideo",
			"Person", "Photo", "PhotoAlbum", "Playlist", "PlaylistsFolder", "Program",
			"Recording", "Season", "Series", "Studio", "Trailer", "TvChannel",
			"TvProgram", "UserRootFolder", "UserView", "Video", "Year",
		),
		field.Enum("media_type").Values("Unknown", "Video", "Audio", "Photo", "Book").
			Default("Unknown"),

		field.String("key").Optional(),
		field.String("name"),
		field.String("sort_name").Optional(),
		field.Time("deleted_at").Optional().Nillable(),
		field.Text("overview").Optional(),

		field.Bool("is_folder").Default(false),
		field.Bool("lock_data").Default(false),
		field.Bool("has_subtitles").Default(false),

		field.Time("premiere_date").Optional().Nillable(),
		field.Time("end_date").Optional().Nillable(),
		field.Time("date_modified").Optional(),
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
		edge.From("library", Library.Type).Ref("items").Unique().Field("library_id"),
		edge.To("item_sources", ItemSource.Type).Annotations(cascadeOnDelete),
		edge.To("credits", Credit.Type).Annotations(cascadeOnDelete),
		edge.To("images", Image.Type).Annotations(cascadeOnDelete),
		edge.To("user_data", UserItemData.Type).Annotations(cascadeOnDelete),
		edge.To("activity_log_entries", ActivityLogEntry.Type),
		edge.To("playlist", Playlist.Type).Unique().Annotations(cascadeOnDelete),
		edge.To("playlist_entries", PlaylistEntry.Type).Annotations(cascadeOnDelete),
		edge.To("genres", Genre.Type),
		edge.To("studios", Studio.Type),
	}
}

func (Item) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("library_id", "key").Unique(),
		index.Fields("kind", "sort_name"),
		index.Fields("deleted_at"),
	}
}
