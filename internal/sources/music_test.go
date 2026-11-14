package sources

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/FreekingDean/gojellyfin/internal/env"
	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/store"
	itemmodel "github.com/FreekingDean/gojellyfin/internal/store/item"
)

func lidarrServer(t *testing.T, asked *[]string) *httptest.Server {
	t.Helper()

	answers := map[string]any{
		"/api/v1/tag": []map[string]any{{"id": 4, "label": "kids"}},
		"/api/v1/artist": []map[string]any{
			{
				"id": 1, "foreignArtistId": "A-ONE", "artistName": "The Band", "tags": []int{4},
				"images": []map[string]any{
					{"coverType": "poster", "remoteUrl": "https://art/poster.jpg"},
					{"coverType": "poster", "remoteUrl": "https://art/second.jpg"},
					{"coverType": "fanart", "remoteUrl": "https://art/fanart.jpg"},
				},
			},
			{"id": 2, "foreignArtistId": "", "artistName": "Unknown"},
			{"id": 3, "foreignArtistId": "A-EMPTY", "artistName": "Nothing On Disk"},
		},
		"/api/v1/album": []map[string]any{
			{
				"id": 10, "foreignAlbumId": "R-ONE", "title": "First Album", "releaseDate": "2019-05-10T00:00:00Z",
				"images": []map[string]any{{"coverType": "cover", "remoteUrl": "https://art/cover.jpg"}},
			},
			{"id": 11, "foreignAlbumId": "R-WANTED", "title": "Not Downloaded"},
		},
		"/api/v1/track": []map[string]any{
			{"id": 100, "albumId": 10, "foreignTrackId": "T-ONE", "trackFileId": 1000, "trackNumber": "1", "mediumNumber": 1, "title": "Opener", "hasFile": true},
			{"id": 101, "albumId": 10, "foreignTrackId": "T-TWO", "trackFileId": 1001, "trackNumber": "A2", "absoluteTrackNumber": 2, "mediumNumber": 1, "title": "Opener", "hasFile": true},
			{"id": 102, "albumId": 11, "foreignTrackId": "T-THREE", "title": "Wanted", "hasFile": false},
		},
		"/api/v1/trackfile": []map[string]any{
			{"id": 1000, "path": "/music/The Band/First Album/01.flac", "dateAdded": "2024-01-01T00:00:00Z"},
			{"id": 1001, "path": "/music/The Band/First Album/02.flac", "dateAdded": "2024-01-02T00:00:00Z"},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*asked = append(*asked, r.URL.Path+"?"+r.URL.RawQuery)
		if r.Header.Get("X-Api-Key") != "key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		answer, ok := answers[r.URL.Path]
		if !ok || (r.URL.Path != "/api/v1/artist" && r.URL.Path != "/api/v1/tag" && r.URL.Query().Get("artistId") != "1") {
			_ = json.NewEncoder(w).Encode([]any{})
			return
		}

		_ = json.NewEncoder(w).Encode(answer)
	}))
	t.Cleanup(server.Close)

	return server
}

func TestTitles_Lidarr(t *testing.T) {
	var asked []string
	server := lidarrServer(t, &asked)
	service := New(nil, env.Config{SourceAPIKeys: map[string]string{"SOURCE_API_KEY_TEST": "key"}}, nil, nil)

	binding := Binding{
		Source: Source{
			Name: "lidarr", Kind: KindLidarr, URL: server.URL, APIKeyVariable: "SOURCE_API_KEY_TEST",
			Paths: []*store.SourcePathModel{{RemotePath: "/music", LocalPath: "/media/music"}},
		},
		Library: LibrarySource{TagFilter: "kids"},
	}

	titles, err := service.Titles(context.Background(), binding)
	if err != nil {
		t.Fatalf("failed to read the instance: %v", err)
	}

	byKey := map[string]Title{}
	order := make([]string, 0, len(titles))
	for _, title := range titles {
		byKey[title.Key] = title
		order = append(order, title.Key)
	}

	want := []string{"artist:mb:a-one", "album:mb:r-one", "track:mb:t-one", "track:mb:t-two"}
	if len(order) != len(want) {
		t.Fatalf("titles = %v, want %v", order, want)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Errorf("title %d = %q, want %q, each parent ahead of what hangs off it", i, order[i], want[i])
		}
	}

	artist := byKey["artist:mb:a-one"]
	if artist.Kind != itemmodel.KindMusicArtist || !artist.Tagged || artist.ParentKey != "" {
		t.Errorf("artist = %+v, want a tagged top-level MusicArtist", artist)
	}
	if len(artist.Images) != 2 || artist.Images[0].URL != "https://art/poster.jpg" || artist.Images[1].Kind != items.ImageKindBackdrop {
		t.Errorf("artist images = %+v, want the first poster and the fanart", artist.Images)
	}

	album := byKey["album:mb:r-one"]
	if album.Kind != itemmodel.KindMusicAlbum || album.ParentKey != artist.Key || album.Year == nil || *album.Year != 2019 {
		t.Errorf("album = %+v, want a 2019 MusicAlbum under the artist", album)
	}

	first, second := byKey["track:mb:t-one"], byKey["track:mb:t-two"]
	if first.Kind != itemmodel.KindAudio || first.ParentKey != album.Key {
		t.Errorf("track = %+v, want Audio under the album", first)
	}
	if len(first.Files) != 1 || first.Files[0].Path != "/media/music/The Band/First Album/01.flac" {
		t.Errorf("files = %+v, want the file at this server's path", first.Files)
	}
	if first.Files[0].ExternalID == nil || *first.Files[0].ExternalID != 100 || *first.Files[0].ExternalFileID != 1000 {
		t.Errorf("file = %+v, want the instance's track and file ids", first.Files[0])
	}
	if first.Name != second.Name || first.Key == second.Key {
		t.Error("two tracks sharing a title were folded into one")
	}
	if second.Index == nil || *second.Index != 2 || second.ParentIndex == nil || *second.ParentIndex != 1 {
		t.Errorf("track number = %v disc %v, want the absolute number 2 on disc 1 for a side-lettered track", second.Index, second.ParentIndex)
	}

	for _, path := range asked {
		if path[:8] != "/api/v1/" {
			t.Errorf("asked %q, want everything under Lidarr's v1 api", path)
		}
	}
}
