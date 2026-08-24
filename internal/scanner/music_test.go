package scanner

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/env"
	"github.com/FreekingDean/gojellyfin/internal/filesystem"
	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/libraries"
	"github.com/FreekingDean/gojellyfin/internal/store"
	itemmodal "github.com/FreekingDean/gojellyfin/internal/store/item"
	sourcemodal "github.com/FreekingDean/gojellyfin/internal/store/mediasource"
)

func TestParseTrack(t *testing.T) {
	tests := []struct {
		in    string
		disc  int32
		track int32
		title string
	}{
		{"01 - Bohemian Rhapsody.mp3", 0, 1, "Bohemian Rhapsody"},
		{"07. Love of My Life.flac", 0, 7, "Love of My Life"},
		{"2-04 The Show Must Go On.m4a", 2, 4, "The Show Must Go On"},
		{"Bohemian Rhapsody.mp3", 0, 0, "Bohemian Rhapsody"},
		{"1979.mp3", 0, 0, "1979"},
	}

	for _, test := range tests {
		disc, track, title := parseTrack(test.in)
		if title != test.title {
			t.Errorf("parseTrack(%q) title = %q, want %q", test.in, title, test.title)
		}
		if test.track == 0 {
			if track != nil {
				t.Errorf("parseTrack(%q) track = %d, want nil", test.in, *track)
			}
			continue
		}
		if track == nil || *track != test.track {
			t.Errorf("parseTrack(%q) track = %v, want %d", test.in, track, test.track)
		}
		if test.disc == 0 {
			if disc != nil {
				t.Errorf("parseTrack(%q) disc = %d, want nil", test.in, *disc)
			}
			continue
		}
		if disc == nil || *disc != test.disc {
			t.Errorf("parseTrack(%q) disc = %v, want %d", test.in, disc, test.disc)
		}
	}
}

func TestParseDisc(t *testing.T) {
	tests := []struct {
		in     string
		number int32
		ok     bool
	}{
		{"Disc 2", 2, true},
		{"CD1", 1, true},
		{"disk_3", 3, true},
		{"Bonus Material", 0, false},
	}

	for _, test := range tests {
		number, ok := parseDisc(test.in)
		if ok != test.ok {
			t.Errorf("parseDisc(%q) ok = %v, want %v", test.in, ok, test.ok)
			continue
		}
		if ok && *number != test.number {
			t.Errorf("parseDisc(%q) = %d, want %d", test.in, *number, test.number)
		}
	}
}

func TestIsAudio(t *testing.T) {
	for _, name := range []string{"song.mp3", "Song.FLAC", "song.opus"} {
		if !isAudio(name) {
			t.Errorf("isAudio(%q) = false, want true", name)
		}
	}
	for _, name := range []string{"movie.mkv", "cover.jpg", "song"} {
		if isAudio(name) {
			t.Errorf("isAudio(%q) = true, want false", name)
		}
	}
}

func TestScanMusicBuildsTheArtistAlbumTrackChain(t *testing.T) {
	ctx := context.Background()

	config, err := env.Load()
	if err != nil {
		t.Fatalf("failed to read the environment: %v", err)
	}

	connection, err := store.NewStore(config)
	if err != nil {
		t.Fatalf("failed to open the database: %v", err)
	}
	if err := connection.Start(); err != nil {
		t.Fatalf("failed to reach the database, set DATABASE_URL: %v", err)
	}

	client := connection.Client()
	library, err := client.Library.Create().SetName(t.Name() + "-" + uuid.NewString()).Save(ctx)
	if err != nil {
		t.Fatalf("failed to create the library: %v", err)
	}

	t.Cleanup(func() {
		owned := itemmodal.LibraryID(library.ID)
		if _, err := client.MediaSource.Delete().Where(sourcemodal.HasItemWith(owned)).Exec(ctx); err != nil {
			t.Errorf("failed to delete the media sources: %v", err)
		}
		if _, err := client.Item.Delete().Where(owned).Exec(ctx); err != nil {
			t.Errorf("failed to delete the items: %v", err)
		}
		if err := client.Library.DeleteOne(library).Exec(ctx); err != nil {
			t.Errorf("failed to delete the library: %v", err)
		}
		if err := connection.Stop(); err != nil {
			t.Errorf("failed to close the database: %v", err)
		}
	})

	root := t.TempDir()
	for _, name := range []string{
		"Queen/A Night at the Opera (1975)/01 - Death on Two Legs.mp3",
		"Queen/A Night at the Opera (1975)/Disc 2/03 - Love of My Life.mp3",
		"Queen/Loose Track.flac",
		"Root Level.mp3",
	} {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatalf("failed to create %q: %v", name, err)
		}
		if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
			t.Fatalf("failed to write %q: %v", name, err)
		}
	}

	service := items.New(client)
	found := &seen{}
	if err := New(service, nil, filesystem.New()).scanMusic(ctx, &libraries.Library{ID: library.ID}, root, found); err != nil {
		t.Fatalf("failed to scan the music: %v", err)
	}
	if len(found.keys) != 6 {
		t.Fatalf("titles = %d, want 6 (one artist, one album, four tracks)", len(found.keys))
	}
	if len(found.paths) != 4 {
		t.Fatalf("files = %d, want 4", len(found.paths))
	}

	artist, err := service.ItemByName(ctx, itemmodal.KindMusicArtist, "Queen")
	if err != nil {
		t.Fatalf("failed to find the artist: %v", err)
	}
	if artist.ParentID != nil {
		t.Errorf("artist parent = %v, want nil", artist.ParentID)
	}

	album, err := service.ItemByName(ctx, itemmodal.KindMusicAlbum, "A Night at the Opera")
	if err != nil {
		t.Fatalf("failed to find the album: %v", err)
	}
	if album.ParentID == nil || *album.ParentID != artist.ID {
		t.Errorf("album parent = %v, want the artist", album.ParentID)
	}
	if album.ProductionYear == nil || *album.ProductionYear != 1975 {
		t.Errorf("album year = %v, want 1975", album.ProductionYear)
	}

	track, err := service.ItemByName(ctx, itemmodal.KindAudio, "Love of My Life")
	if err != nil {
		t.Fatalf("failed to find the track: %v", err)
	}
	if track.ParentID == nil || *track.ParentID != album.ID {
		t.Errorf("track parent = %v, want the album", track.ParentID)
	}
	if track.IndexNumber == nil || *track.IndexNumber != 3 {
		t.Errorf("track number = %v, want 3", track.IndexNumber)
	}
	if track.ParentIndexNumber == nil || *track.ParentIndexNumber != 2 {
		t.Errorf("disc number = %v, want 2", track.ParentIndexNumber)
	}
	if track.MediaType != itemmodal.MediaTypeAudio {
		t.Errorf("media type = %s, want Audio", track.MediaType)
	}
	if track.IsFolder {
		t.Error("a track is not a folder")
	}

	loose, err := service.ItemByName(ctx, itemmodal.KindAudio, "Loose Track")
	if err != nil {
		t.Fatalf("failed to find the album-less track: %v", err)
	}
	if loose.ParentID == nil || *loose.ParentID != artist.ID {
		t.Errorf("album-less track parent = %v, want the artist", loose.ParentID)
	}

	orphan, err := service.ItemByName(ctx, itemmodal.KindAudio, "Root Level")
	if err != nil {
		t.Fatalf("failed to find the root level track: %v", err)
	}
	if orphan.ParentID != nil {
		t.Errorf("root level track parent = %v, want nil", orphan.ParentID)
	}
}

// Two encodes of one track are one entry with two files, which is what the key
// exists to collapse. Two different songs are not, even when they sit in the
// same album.
func TestScanMusicFoldsTwoEncodesOfOneTrack(t *testing.T) {
	ctx := context.Background()

	config, err := env.Load()
	if err != nil {
		t.Fatalf("failed to read the environment: %v", err)
	}

	connection, err := store.NewStore(config)
	if err != nil {
		t.Fatalf("failed to open the database: %v", err)
	}
	if err := connection.Start(); err != nil {
		t.Fatalf("failed to reach the database, set DATABASE_URL: %v", err)
	}

	client := connection.Client()
	library, err := client.Library.Create().SetName(t.Name() + "-" + uuid.NewString()).Save(ctx)
	if err != nil {
		t.Fatalf("failed to create the library: %v", err)
	}

	t.Cleanup(func() {
		owned := itemmodal.LibraryID(library.ID)
		if _, err := client.MediaSource.Delete().Where(sourcemodal.HasItemWith(owned)).Exec(ctx); err != nil {
			t.Errorf("failed to delete the media sources: %v", err)
		}
		if _, err := client.Item.Delete().Where(owned).Exec(ctx); err != nil {
			t.Errorf("failed to delete the items: %v", err)
		}
		if err := client.Library.DeleteOne(library).Exec(ctx); err != nil {
			t.Errorf("failed to delete the library: %v", err)
		}
		if err := connection.Stop(); err != nil {
			t.Errorf("failed to close the database: %v", err)
		}
	})

	root := t.TempDir()
	for _, name := range []string{
		"Nirvana/Nevermind/01 - Smells Like Teen Spirit.flac",
		"Nirvana/Nevermind/01 - Smells Like Teen Spirit.mp3",
		"Nirvana/Nevermind/02 - In Bloom.mp3",
	} {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatalf("failed to create %q: %v", name, err)
		}
		if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
			t.Fatalf("failed to write %q: %v", name, err)
		}
	}

	service := items.New(client)
	found := &seen{}
	if err := New(service, nil, filesystem.New()).scanMusic(ctx, &libraries.Library{ID: library.ID}, root, found); err != nil {
		t.Fatalf("failed to scan the music: %v", err)
	}

	tracks, _, err := service.QueryItems(ctx, items.ItemQuery{
		LibraryID: &library.ID,
		Kinds:     []items.Kind{itemmodal.KindAudio},
	})
	if err != nil {
		t.Fatalf("failed to query the tracks: %v", err)
	}
	if len(tracks) != 2 {
		t.Fatalf("tracks = %d, want 2", len(tracks))
	}

	byName := make(map[string]*items.Item, len(tracks))
	for _, track := range tracks {
		byName[track.Name] = track
	}

	folded, ok := byName["Smells Like Teen Spirit"]
	if !ok {
		t.Fatalf("the folded track is missing, got %v", byName)
	}

	sources, err := service.MediaSources(ctx, folded.ID)
	if err != nil {
		t.Fatalf("failed to query the sources: %v", err)
	}
	if len(sources) != 2 {
		t.Errorf("sources = %d, want 2, so both encodes stay playable", len(sources))
	}
}
