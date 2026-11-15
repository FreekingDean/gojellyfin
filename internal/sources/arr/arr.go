package arr

import "time"

const (
	StatusPath     = "system/status"
	TagsPath       = "tag"
	RootFolderPath = "rootfolder"
	ProfilesPath   = "qualityprofile"
	CommandPath    = "command"
	FailedPath     = "history/failed"
	NotifyPath     = "notification"
	NotifySchema   = "notification/schema"
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
	Artist    *Subject `json:"artist"`
}

type Subject struct {
	ID     int64  `json:"id"`
	TmdbID int    `json:"tmdbId"`
	MBID   string `json:"mbId"`
}
