package arr

import "time"

const (
	StatusPath     = "api/v3/system/status"
	TagsPath       = "api/v3/tag"
	RootFolderPath = "api/v3/rootfolder"
	ProfilesPath   = "api/v3/qualityprofile"
	CommandPath    = "api/v3/command"
	FailedPath     = "api/v3/history/failed"
	NotifyPath     = "api/v3/notification"
	NotifySchema   = "api/v3/notification/schema"
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

type QualityProfile struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type Grab struct {
	ID   int       `json:"id"`
	Date time.Time `json:"date"`
}

func Newest(grabs []Grab) (Grab, bool) {
	if len(grabs) == 0 {
		return Grab{}, false
	}

	newest := grabs[0]
	for _, grab := range grabs[1:] {
		if grab.Date.After(newest.Date) {
			newest = grab
		}
	}

	return newest, true
}

type Webhook struct {
	EventType string   `json:"eventType"`
	Movie     *Subject `json:"movie"`
	Series    *Subject `json:"series"`
}

type Subject struct {
	ID     int64 `json:"id"`
	TmdbID int   `json:"tmdbId"`
}
