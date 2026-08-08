package entities

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
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
		field.Enum("kind").Values(
			"AggregateFolder", "Audio", "AudioBook", "BasePluginFolder", "Book",
			"BoxSet", "Channel", "ChannelFolderItem", "CollectionFolder", "Episode",
			"Folder", "Genre", "ManualPlaylistsFolder", "Movie", "LiveTvChannel",
			"LiveTvProgram", "MusicAlbum", "MusicArtist", "MusicGenre", "MusicVideo",
			"Person", "Photo", "PhotoAlbum", "Playlist", "PlaylistsFolder", "Program",
			"Recording", "Season", "Series", "Studio", "Trailer", "TvChannel",
			"TvProgram", "UserRootFolder", "UserView", "Video", "Year",
		),
		field.Enum("media_type").Values("Unknown", "Video", "Audio", "Photo", "Book"),
		field.Enum("location_type").Values("FileSystem", "Remote", "Virtual", "Offline"),
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

		field.String("name"),
		field.String("original_title").Optional(),
		field.String("sort_name").Optional(),
		field.Bool("forced_sort_name"),
		field.String("path").Optional(),
		field.String("container").Optional(),
		field.Text("overview").Optional(),

		field.Bool("is_folder"),
		field.Bool("is_placeholder"),
		field.Bool("lock_data"),
		field.Bool("has_lyrics"),
		field.Bool("has_subtitles"),
		field.Bool("enable_media_source_display"),

		field.Time("premiere_date").Optional(),
		field.Time("end_date").Optional(),
		field.Time("last_media_added_at").Optional(),
		field.Int32("production_year").Optional(),

		field.String("official_rating").Optional(),
		field.String("custom_rating").Optional(),
		field.Float("critic_rating").Optional(),
		field.Float("community_rating").Optional(),

		field.Int64("run_time_ticks").Optional(),
		field.Int32("index_number").Optional(),
		field.Int32("index_number_end").Optional(),
		field.Int32("parent_index_number").Optional(),
		field.Int32("airs_before_season_number").Optional(),
		field.Int32("airs_after_season_number").Optional(),
		field.Int32("airs_before_episode_number").Optional(),

		field.String("status").Optional(),
		field.String("air_time").Optional(),
		field.String("display_order").Optional(),
		field.JSON("air_days", []string{}).Optional(),

		field.String("aspect_ratio").Optional(),
		field.Int32("width").Optional(),
		field.Int32("height").Optional(),
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
		edge.To("children", Item.Type).From("parent").Unique(),
		edge.From("library", Library.Type).Ref("items").Unique(),
		edge.To("media_sources", MediaSource.Type),
		edge.To("credits", Credit.Type),
		edge.To("chapters", Chapter.Type),
		edge.To("images", Image.Type),
		edge.To("user_data", UserItemData.Type),
		edge.To("display_preferences", DisplayPreferences.Type),
		edge.To("activity_log_entries", ActivityLogEntry.Type),
		edge.To("trickplays", Trickplay.Type),
		edge.To("media_segments", MediaSegment.Type),
		edge.To("genres", Genre.Type),
		edge.To("studios", Studio.Type),
	}
}
