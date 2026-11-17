package sources

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/FreekingDean/gojellyfin/internal/env"
	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/store"
	itemmodel "github.com/FreekingDean/gojellyfin/internal/store/item"
)

var lidarrAnswers = map[string]any{
	"/api/v1/tag": []map[string]any{{"id": 4, "label": "kids"}},
	"/api/v1/artist": []map[string]any{
		{
			"id": 1, "foreignArtistId": "A-ONE", "artistName": "The Band", "tags": []int{4}, "genres": []string{"Rock"},
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

func lidarrServer(t *testing.T, asked *[]string) *httptest.Server {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*asked = append(*asked, r.URL.Path+"?"+r.URL.RawQuery)
		if r.Header.Get("X-Api-Key") != "key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		if r.URL.Path == "/api/v1/artist/1" {
			_ = json.NewEncoder(w).Encode(lidarrAnswers["/api/v1/artist"].([]map[string]any)[0])
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/v1/artist/") {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		answer, ok := lidarrAnswers[r.URL.Path]
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
	if len(artist.Genres) != 1 || artist.Genres[0] != "Rock" {
		t.Errorf("artist genres = %v, want Lidarr's", artist.Genres)
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

func TestReported_Lidarr(t *testing.T) {
	var asked []string
	server := lidarrServer(t, &asked)
	service := New(nil, env.Config{SourceAPIKeys: map[string]string{"SOURCE_API_KEY_TEST": "key"}}, nil, nil)
	source := Source{
		Name: "lidarr", Kind: KindLidarr, URL: server.URL, APIKeyVariable: "SOURCE_API_KEY_TEST",
		Paths: []*store.SourcePathModel{{RemotePath: "/music", LocalPath: "/media/music"}},
	}
	ctx := context.Background()

	t.Run("an import reads the artist and everything on disk beneath it", func(t *testing.T) {
		held, err := service.reported(ctx, source, Hooked{ID: 1})
		if err != nil {
			t.Fatalf("failed to read the artist: %v", err)
		}
		if !reflect.DeepEqual(held.scope(), items.ArtistScope("A-ONE")) {
			t.Errorf("scope = %+v, want the artist's tree", held.scope())
		}

		for _, test := range []struct {
			name   string
			tagged func([]int) bool
		}{
			{"a binding taking the artist's tag", func(tags []int) bool { return len(tags) == 1 && tags[0] == 4 }},
			{"a binding taking another tag", func([]int) bool { return false }},
		} {
			titles, err := held.titles(source, test.tagged)
			if err != nil {
				t.Fatalf("failed to build the titles: %v", err)
			}
			if len(titles) != 4 {
				t.Fatalf("%s: titles = %d, want the artist, its album and two tracks", test.name, len(titles))
			}
			for _, title := range titles {
				if title.Tagged != test.tagged([]int{4}) {
					t.Errorf("%s: %s tagged = %v, want the artist's answer", test.name, title.Key, title.Tagged)
				}
			}
		}
	})

	t.Run("a deleted artist sweeps the tree the event named", func(t *testing.T) {
		held, err := service.reported(ctx, source, Hooked{ID: 9, MusicBrainzID: "GONE"})
		if err != nil {
			t.Fatalf("failed to read the artist: %v", err)
		}
		titles, _ := held.titles(source, func([]int) bool { return true })
		if !held.known() || !reflect.DeepEqual(held.scope(), items.ArtistScope("GONE")) || len(titles) != 0 {
			t.Errorf("report = %+v, want nothing reported under the deleted artist's scope", held)
		}
	})

	t.Run("a deleted artist the event did not name is left for the next refresh", func(t *testing.T) {
		held, err := service.reported(ctx, source, Hooked{ID: 9})
		if err != nil || held.known() {
			t.Errorf("report = %+v, err = %v, want nothing known", held, err)
		}
	})
}
