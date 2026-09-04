package arr

import "time"

const (
	StatusPath = "api/v3/system/status"
	TagsPath   = "api/v3/tag"
	APIKeyName = "X-Api-Key"
)

type Tag struct {
	ID    int    `json:"id"`
	Label string `json:"label"`
}

type File struct {
	Path      string    `json:"path"`
	Size      int64     `json:"size"`
	DateAdded time.Time `json:"dateAdded"`
}
