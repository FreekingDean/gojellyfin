package arr

import "time"

const (
	StatusPath     = "api/v3/system/status"
	TagsPath       = "api/v3/tag"
	RootFolderPath = "api/v3/rootfolder"
	APIKeyName     = "X-Api-Key"
)

type Status struct {
	Version string `json:"version"`
}

type RootFolder struct {
	Path string `json:"path"`
}

type Tag struct {
	ID    int    `json:"id"`
	Label string `json:"label"`
}

type File struct {
	ID        int64     `json:"id"`
	Path      string    `json:"path"`
	Size      int64     `json:"size"`
	DateAdded time.Time `json:"dateAdded"`
}
