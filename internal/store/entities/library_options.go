package entities

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

type TypeOptions struct {
	Type                 string   `json:"type"`
	MetadataFetchers     []string `json:"metadata_fetchers"`
	MetadataFetcherOrder []string `json:"metadata_fetcher_order"`
	ImageFetchers        []string `json:"image_fetchers"`
	ImageFetcherOrder    []string `json:"image_fetcher_order"`
}

type LibraryOptions struct {
	ent.Schema
}

func (LibraryOptions) Fields() []ent.Field {
	return withDefaultFields(
		field.Bool("enabled").Default(true),
		field.Bool("enable_photos").Default(true),
		field.Bool("enable_realtime_monitor").Default(true),
		field.Bool("enable_lufs_scan").Default(true),
		field.Bool("enable_chapter_image_extraction").Default(false),
		field.Bool("extract_chapter_images_during_library_scan").Default(false),
		field.Bool("enable_trickplay_image_extraction").Default(false),
		field.Bool("extract_trickplay_images_during_library_scan").Default(false),
		field.Bool("save_local_metadata").Default(false),
		field.Bool("enable_internet_providers").Default(true),
		field.Bool("enable_automatic_series_grouping").Default(false),
		field.Bool("enable_embedded_titles").Default(false),
		field.Bool("enable_embedded_extras_titles").Default(false),
		field.Bool("enable_embedded_episode_infos").Default(false),
		field.Bool("skip_subtitles_if_embedded_subtitles_present").Default(false),
		field.Bool("skip_subtitles_if_audio_track_matches").Default(false),
		field.Bool("require_perfect_subtitle_match").Default(true),
		field.Bool("save_subtitles_with_media").Default(true),
		field.Bool("save_lyrics_with_media").Default(false),
		field.Bool("save_trickplay_with_media").Default(false),
		field.Bool("prefer_nonstandard_artists_tag").Default(false),
		field.Bool("use_custom_tag_delimiters").Default(false),
		field.Bool("automatically_add_to_collection").Default(false),

		field.Int32("automatic_refresh_interval_days").Default(0),
		field.String("preferred_metadata_language").Default(""),
		field.String("metadata_country_code").Default(""),
		field.String("season_zero_display_name").Default("Specials"),

		field.Enum("allow_embedded_subtitles").Values(
			"AllowAll", "AllowText", "AllowImage", "AllowNone",
		).Default("AllowAll"),

		field.JSON("metadata_savers", []string{}).Optional().Default([]string{}),
		field.JSON("disabled_local_metadata_readers", []string{}).Optional().Default([]string{}),
		field.JSON("local_metadata_reader_order", []string{}).Optional().Default([]string{}),
		field.JSON("disabled_subtitle_fetchers", []string{}).Optional().Default([]string{}),
		field.JSON("subtitle_fetcher_order", []string{}).Optional().Default([]string{}),
		field.JSON("disabled_media_segment_providers", []string{}).Optional().Default([]string{}),
		field.JSON("media_segment_provider_order", []string{}).Optional().Default([]string{}),
		field.JSON("subtitle_download_languages", []string{}).Optional().Default([]string{}),
		field.JSON("disabled_lyric_fetchers", []string{}).Optional().Default([]string{}),
		field.JSON("lyric_fetcher_order", []string{}).Optional().Default([]string{}),
		field.JSON("custom_tag_delimiters", []string{}).Optional().Default([]string{}),
		field.JSON("delimiter_whitelist", []string{}).Optional().Default([]string{}),
		field.JSON("type_options", []TypeOptions{}).Optional().Default([]TypeOptions{}),
	)
}

func (LibraryOptions) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("library", Library.Type).Ref("options").Unique(),
	}
}
