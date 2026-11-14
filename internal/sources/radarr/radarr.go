package radarr

import "github.com/FreekingDean/gojellyfin/internal/sources/arr"

const (
	MoviesPath = "movie"
	LookupPath = "movie/lookup/tmdb"
	FilePath   = "moviefile"
	GrabsPath  = "history/movie"
	Search     = "MoviesSearch"

	GrabbedEvent = "grabbed"
)

type Movie struct {
	ID      int64    `json:"id"`
	TmdbID  int      `json:"tmdbId"`
	Title   string   `json:"title"`
	Year    int32    `json:"year"`
	Path    string   `json:"path"`
	HasFile bool     `json:"hasFile"`
	File    arr.File `json:"movieFile"`
	Tags    []int    `json:"tags"`
}
