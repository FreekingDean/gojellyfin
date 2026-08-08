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
		field.Bool("enabled"),
		field.Bool("enable_photos"),
		field.Bool("enable_realtime_monitor"),
		field.Bool("enable_lufs_scan"),
		field.Bool("enable_chapter_image_extraction"),
		field.Bool("extract_chapter_images_during_library_scan"),
		field.Bool("enable_trickplay_image_extraction"),
		field.Bool("extract_trickplay_images_during_library_scan"),
		field.Bool("save_local_metadata"),
		field.Bool("enable_internet_providers"),
		field.Bool("enable_automatic_series_grouping"),
		field.Bool("enable_embedded_titles"),
		field.Bool("enable_embedded_extras_titles"),
		field.Bool("enable_embedded_episode_infos"),
		field.Bool("skip_subtitles_if_embedded_subtitles_present"),
		field.Bool("skip_subtitles_if_audio_track_matches"),
		field.Bool("require_perfect_subtitle_match"),
		field.Bool("save_subtitles_with_media"),
		field.Bool("save_lyrics_with_media"),
		field.Bool("save_trickplay_with_media"),
		field.Bool("prefer_nonstandard_artists_tag"),
		field.Bool("use_custom_tag_delimiters"),
		field.Bool("automatically_add_to_collection"),

		field.Int32("automatic_refresh_interval_days"),
		field.String("preferred_metadata_language"),
		field.String("metadata_country_code"),
		field.String("season_zero_display_name"),

		field.Enum("allow_embedded_subtitles").Values(
			"AllowAll", "AllowText", "AllowImage", "AllowNone",
		),

		field.JSON("metadata_savers", []string{}).Optional(),
		field.JSON("disabled_local_metadata_readers", []string{}).Optional(),
		field.JSON("local_metadata_reader_order", []string{}).Optional(),
		field.JSON("disabled_subtitle_fetchers", []string{}).Optional(),
		field.JSON("subtitle_fetcher_order", []string{}).Optional(),
		field.JSON("disabled_media_segment_providers", []string{}).Optional(),
		field.JSON("media_segment_provider_order", []string{}).Optional(),
		field.JSON("subtitle_download_languages", []string{}).Optional(),
		field.JSON("disabled_lyric_fetchers", []string{}).Optional(),
		field.JSON("lyric_fetcher_order", []string{}).Optional(),
		field.JSON("custom_tag_delimiters", []string{}).Optional(),
		field.JSON("delimiter_whitelist", []string{}).Optional(),
		field.JSON("type_options", []TypeOptions{}).Optional(),
	)
}

func (LibraryOptions) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("library", Library.Type).Ref("options").Unique(),
	}
}
