package lidarr

import "github.com/FreekingDean/gojellyfin/internal/sources/arr"

const (
	ArtistsPath = "artist"
	AlbumsPath  = "album"
	TracksPath  = "track"
	FilesPath   = "trackfile"
)

type Image struct {
	CoverType string `json:"coverType"`
	RemoteURL string `json:"remoteUrl"`
}

type Artist struct {
	ID              int      `json:"id"`
	ForeignArtistID string   `json:"foreignArtistId"`
	ArtistName      string   `json:"artistName"`
	Tags            []int    `json:"tags"`
	Genres          []string `json:"genres"`
	Images          []Image  `json:"images"`
}

type Album struct {
	ID             int      `json:"id"`
	ForeignAlbumID string   `json:"foreignAlbumId"`
	Title          string   `json:"title"`
	ReleaseDate    string   `json:"releaseDate"`
	Genres         []string `json:"genres"`
	Images         []Image  `json:"images"`
}

type Track struct {
	ID                  int64  `json:"id"`
	AlbumID             int    `json:"albumId"`
	ForeignTrackID      string `json:"foreignTrackId"`
	TrackFileID         int64  `json:"trackFileId"`
	TrackNumber         string `json:"trackNumber"`
	AbsoluteTrackNumber int32  `json:"absoluteTrackNumber"`
	MediumNumber        int32  `json:"mediumNumber"`
	Title               string `json:"title"`
	HasFile             bool   `json:"hasFile"`
}

type File = arr.File
