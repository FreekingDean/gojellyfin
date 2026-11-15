package sonarr

import "github.com/FreekingDean/gojellyfin/internal/sources/arr"

const (
	SeriesPath   = "series"
	EpisodesPath = "episode"
	LookupPath   = "series/lookup"
	FilePath     = "episodefile"
	GrabsPath    = "history"
	Search       = "EpisodeSearch"
	GrabbedEvent = "1"
)

type Series struct {
	ID     int    `json:"id"`
	TmdbID int    `json:"tmdbId"`
	Title  string `json:"title"`
	Year   int32  `json:"year"`
	Path   string `json:"path"`
	Tags   []int  `json:"tags"`
}

type Episode struct {
	ID            int64    `json:"id"`
	SeasonNumber  int32    `json:"seasonNumber"`
	EpisodeNumber int32    `json:"episodeNumber"`
	Title         string   `json:"title"`
	HasFile       bool     `json:"hasFile"`
	File          arr.File `json:"episodeFile"`
}

type Grabs struct {
	Records []arr.Grab `json:"records"`
}
