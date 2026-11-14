package radarr

import "github.com/FreekingDean/gojellyfin/internal/sources/arr"

const (
	MoviesPath = "api/v3/movie"
	LookupPath = "api/v3/movie/lookup/tmdb"
	FilePath   = "api/v3/moviefile"
	GrabsPath  = "api/v3/history/movie"
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
