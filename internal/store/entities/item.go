package entities

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

type ExternalUrl struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

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
		field.Enum("location_type").Values("FileSystem", "Remote", "Virtual", "Offline").
			Default("FileSystem"),
		field.Enum("extra_type").Optional().Values(
			"Unknown", "Clip", "Trailer", "BehindTheScenes", "DeletedScene",
			"Interview", "Scene", "Sample", "ThemeSong", "ThemeVideo",
			"Featurette", "Short",
		),
		field.Enum("video_type").Optional().Values("VideoFile", "Iso", "Dvd", "BluRay"),
		field.Enum("iso_type").Optional().Values("Dvd", "BluRay"),
		field.Enum("video_3d_format").Optional().Values(
			"HalfSideBySide", "FullSideBySide", "FullTopAndBottom",
			"HalfTopAndBottom", "MVC",
		),

		field.String("key").Optional(),
		field.String("name"),
		field.String("original_title").Optional(),
		field.String("sort_name").Optional(),
		field.Bool("forced_sort_name").Default(false),
		field.Time("deleted_at").Optional().Nillable(),
		field.String("container").Optional(),
		field.Text("overview").Optional(),

		field.Bool("is_folder").Default(false),
		field.Bool("is_placeholder").Default(false),
		field.Bool("lock_data").Default(false),
		field.Bool("has_lyrics").Default(false),
		field.Bool("has_subtitles").Default(false),
		field.Bool("enable_media_source_display").Default(true),

		field.Time("premiere_date").Optional().Nillable(),
		field.Time("end_date").Optional().Nillable(),
		field.Time("last_media_added_at").Optional().Nillable(),
		field.Time("date_modified").Optional(),
		field.Time("probed_at").Optional(),
		field.Int32("production_year").Optional().Nillable(),

		field.String("official_rating").Optional(),
		field.String("custom_rating").Optional(),
		field.Float("critic_rating").Optional().Nillable(),
		field.Float("community_rating").Optional().Nillable(),

		field.Int64("run_time_ticks").Optional().Nillable(),
		field.Int32("index_number").Optional().Nillable(),
		field.Int32("index_number_end").Optional().Nillable(),
		field.Int32("parent_index_number").Optional().Nillable(),
		field.Int32("airs_before_season_number").Optional().Nillable(),
		field.Int32("airs_after_season_number").Optional().Nillable(),
		field.Int32("airs_before_episode_number").Optional().Nillable(),

		field.String("status").Optional(),
		field.String("air_time").Optional(),
		field.String("display_order").Optional(),
		field.JSON("air_days", []string{}).Optional(),

		field.String("aspect_ratio").Optional(),
		field.Int32("width").Optional().Nillable(),
		field.Int32("height").Optional().Nillable(),
		field.Float("normalization_gain").Optional(),

		field.String("preferred_metadata_language").Optional(),
		field.String("preferred_metadata_country_code").Optional(),

		field.JSON("provider_ids", map[string]string{}).Optional(),
		field.JSON("tags", []string{}).Optional(),
		field.JSON("taglines", []string{}).Optional(),
		field.JSON("production_locations", []string{}).Optional(),
		field.JSON("locked_fields", []string{}).Optional(),
		field.JSON("external_urls", []ExternalUrl{}).Optional(),
	)
}

func (Item) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("children", Item.Type).Annotations(cascadeOnDelete).From("parent").Unique().Field("parent_id"),
		edge.From("library", Library.Type).Ref("items").Unique().Field("library_id"),
		edge.To("media_sources", MediaSource.Type).Annotations(cascadeOnDelete),
		edge.To("credits", Credit.Type).Annotations(cascadeOnDelete),
		edge.To("chapters", Chapter.Type).Annotations(cascadeOnDelete),
		edge.To("images", Image.Type).Annotations(cascadeOnDelete),
		edge.To("user_data", UserItemData.Type).Annotations(cascadeOnDelete),
		edge.To("display_preferences", DisplayPreferences.Type).Annotations(cascadeOnDelete),
		edge.To("activity_log_entries", ActivityLogEntry.Type),
		edge.To("trickplays", Trickplay.Type).Annotations(cascadeOnDelete),
		edge.To("media_segments", MediaSegment.Type).Annotations(cascadeOnDelete),
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
