package radarr

import "github.com/FreekingDean/gojellyfin/internal/sources/arr"

const MoviesPath = "api/v3/movie"

type Movie struct {
	Title   string   `json:"title"`
	Year    int32    `json:"year"`
	Path    string   `json:"path"`
	HasFile bool     `json:"hasFile"`
	File    arr.File `json:"movieFile"`
	Tags    []int    `json:"tags"`
}
