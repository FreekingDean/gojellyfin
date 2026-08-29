package collage

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"io"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/artwork"
	"github.com/FreekingDean/gojellyfin/internal/env"
	"github.com/FreekingDean/gojellyfin/internal/filesystem"
	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/libraries"
	"github.com/FreekingDean/gojellyfin/internal/store"
	imagemodal "github.com/FreekingDean/gojellyfin/internal/store/image"
	itemmodal "github.com/FreekingDean/gojellyfin/internal/store/item"
	librarymodal "github.com/FreekingDean/gojellyfin/internal/store/library"
)

type fixture struct {
	service   *Service
	client    *store.Client
	items     *items.Service
	libraries *libraries.Service
	artwork   artwork.Store
	libraryID uuid.UUID
	added     int
}

func newFixture(t *testing.T) *fixture {
	t.Helper()

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

	ctx := context.Background()
	client := connection.Client()
	catalogue := libraries.New(client)

	library, err := catalogue.CreateLibrary(
		ctx,
		t.Name()+"-"+uuid.NewString(),
		librarymodal.CollectionTypeMovies,
		[]string{"/" + uuid.NewString()},
	)
	if err != nil {
		t.Fatalf("failed to create the library: %v", err)
	}

	stored := artwork.New(client)
	records := items.New(client)
	files := filesystem.New(env.Config{MediaDirectories: []string{filesystem.Root}})

	t.Cleanup(func() {
		if err := catalogue.DeleteLibrary(ctx, library.ID); err != nil {
			t.Errorf("failed to delete the library: %v", err)
		}
		if err := connection.Stop(); err != nil {
			t.Errorf("failed to close the database: %v", err)
		}
	})

	return &fixture{
		service:   New(catalogue, records, files, stored),
		client:    client,
		items:     records,
		libraries: catalogue,
		artwork:   stored,
		libraryID: library.ID,
	}
}

func (f *fixture) title(t *testing.T, shade color.RGBA) uuid.UUID {
	t.Helper()

	ctx := context.Background()
	f.added++
	name := "Title " + uuid.NewString()

	item, err := f.client.Item.Create().
		SetLibraryID(f.libraryID).
		SetKind(itemmodal.KindMovie).
		SetName(name).
		SetSortName(name).
		SetKey("collage:" + name).
		Save(ctx)
	if err != nil {
		t.Fatalf("failed to create the item: %v", err)
	}

	f.artworkFor(t, item.ID, uuid.NewString(), painted(t, shade), time.Now().Add(-time.Duration(f.added)*time.Minute))

	return item.ID
}

func (f *fixture) artworkFor(t *testing.T, itemID uuid.UUID, tag string, body []byte, acquired time.Time) {
	t.Helper()

	ctx := context.Background()
	key := "items/" + itemID.String() + "/Primary/" + tag + ".png"
	if err := f.artwork.Put(ctx, key, bytes.NewReader(body)); err != nil {
		t.Fatalf("failed to store the poster: %v", err)
	}

	_, err := f.client.Image.Create().
		SetItemID(itemID).
		SetKind(imagemodal.KindPrimary).
		SetSource(imagemodal.SourceRemote).
		SetPath(key).
		SetTag(tag).
		SetCreatedAt(acquired).
		Save(ctx)
	if err != nil {
		t.Fatalf("failed to create the image row: %v", err)
	}
}

func (f *fixture) library(t *testing.T) *libraries.Library {
	t.Helper()

	library, err := f.libraries.Library(context.Background(), f.libraryID)
	if err != nil {
		t.Fatalf("failed to read the library: %v", err)
	}

	return library
}

func (f *fixture) build(t *testing.T) {
	t.Helper()

	if err := f.service.BuildLibraryImage(context.Background(), f.libraryID); err != nil {
		t.Fatalf("failed to build the collage: %v", err)
	}
}

func (f *fixture) collage(t *testing.T) image.Image {
	t.Helper()

	library := f.library(t)
	if library.ImageTag == "" {
		t.Fatal("the library has no image tag")
	}

	body, _, found, err := f.artwork.Open(context.Background(), libraries.ImageKey(f.libraryID, library.ImageTag))
	if err != nil || !found {
		t.Fatalf("failed to open the collage: found %v, %v", found, err)
	}
	defer func() { _ = body.Close() }()

	decoded, _, err := image.Decode(body)
	if err != nil {
		t.Fatalf("failed to decode the collage: %v", err)
	}

	return decoded
}

func (f *fixture) stored(t *testing.T, key string) bool {
	t.Helper()

	body, _, found, err := f.artwork.Open(context.Background(), key)
	if err != nil {
		t.Fatalf("failed to look for %q: %v", key, err)
	}
	if found {
		_ = body.Close()
	}

	return found
}

func (f *fixture) bytes(t *testing.T, key string) []byte {
	t.Helper()

	body, _, found, err := f.artwork.Open(context.Background(), key)
	if err != nil || !found {
		t.Fatalf("failed to open %q: found %v, %v", key, found, err)
	}
	defer func() { _ = body.Close() }()

	read, err := io.ReadAll(body)
	if err != nil {
		t.Fatalf("failed to read %q: %v", key, err)
	}

	return read
}

func painted(t *testing.T, shade color.RGBA) []byte {
	t.Helper()

	poster := image.NewRGBA(image.Rect(0, 0, 200, 300))
	for x := range 200 {
		for y := range 300 {
			poster.SetRGBA(x, y, shade)
		}
	}

	written := &bytes.Buffer{}
	if err := png.Encode(written, poster); err != nil {
		t.Fatalf("failed to encode the poster: %v", err)
	}

	return written.Bytes()
}

func TestService_BuildLibraryImage(t *testing.T) {
	t.Run("builds a collage sized to the card the client draws", func(t *testing.T) {
		fixture := newFixture(t)
		fixture.title(t, color.RGBA{R: 200, G: 30, B: 30, A: 255})
		fixture.build(t)

		bounds := fixture.collage(t).Bounds()
		if bounds.Dx() != width || bounds.Dy() != height {
			t.Errorf("collage = %dx%d, want %dx%d", bounds.Dx(), bounds.Dy(), width, height)
		}
	})

	t.Run("leaves a library with no artwork without an image", func(t *testing.T) {
		fixture := newFixture(t)
		fixture.build(t)

		if tag := fixture.library(t).ImageTag; tag != "" {
			t.Errorf("image tag = %q, want it empty", tag)
		}
	})

	t.Run("writes nothing a second time when the posters have not changed", func(t *testing.T) {
		fixture := newFixture(t)
		fixture.title(t, color.RGBA{R: 200, G: 30, B: 30, A: 255})
		fixture.build(t)

		library := fixture.library(t)
		key := libraries.ImageKey(fixture.libraryID, library.ImageTag)
		untouched := []byte("the bytes the first run wrote")
		if err := fixture.artwork.Put(context.Background(), key, bytes.NewReader(untouched)); err != nil {
			t.Fatalf("failed to mark the collage: %v", err)
		}

		fixture.build(t)

		if got := fixture.bytes(t, key); !bytes.Equal(got, untouched) {
			t.Error("the collage was written again for a selection that had not changed")
		}
		if again := fixture.library(t).ImageTag; again != library.ImageTag {
			t.Errorf("image tag = %q, want it to stay %q", again, library.ImageTag)
		}
	})

	t.Run("rebuilds and drops the old bytes when a poster changes", func(t *testing.T) {
		fixture := newFixture(t)
		itemID := fixture.title(t, color.RGBA{R: 200, G: 30, B: 30, A: 255})
		fixture.build(t)

		first := fixture.library(t).ImageTag
		_, err := fixture.client.Image.Update().
			Where(imagemodal.ItemID(itemID)).
			SetTag(uuid.NewString()).
			Save(context.Background())
		if err != nil {
			t.Fatalf("failed to change the poster: %v", err)
		}

		fixture.build(t)

		second := fixture.library(t).ImageTag
		if second == first {
			t.Fatalf("image tag stayed %q after the poster changed", first)
		}
		if fixture.stored(t, libraries.ImageKey(fixture.libraryID, first)) {
			t.Error("the replaced collage is still stored")
		}
		if !fixture.stored(t, libraries.ImageKey(fixture.libraryID, second)) {
			t.Error("the new collage was not stored")
		}
	})

	t.Run("clears the image when the last title goes away", func(t *testing.T) {
		fixture := newFixture(t)
		itemID := fixture.title(t, color.RGBA{R: 200, G: 30, B: 30, A: 255})
		fixture.build(t)

		first := fixture.library(t).ImageTag
		if err := fixture.items.DeleteItem(context.Background(), itemID); err != nil {
			t.Fatalf("failed to delete the item: %v", err)
		}

		fixture.build(t)

		if tag := fixture.library(t).ImageTag; tag != "" {
			t.Errorf("image tag = %q, want it empty", tag)
		}
		if fixture.stored(t, libraries.ImageKey(fixture.libraryID, first)) {
			t.Error("the collage bytes outlived the library's artwork")
		}
	})

	t.Run("steps past a poster it cannot decode", func(t *testing.T) {
		fixture := newFixture(t)
		fixture.title(t, color.RGBA{R: 200, G: 30, B: 30, A: 255})

		broken, err := fixture.client.Item.Create().
			SetLibraryID(fixture.libraryID).
			SetKind(itemmodal.KindMovie).
			SetName("Broken").
			SetSortName("Broken").
			SetKey("collage:broken").
			Save(context.Background())
		if err != nil {
			t.Fatalf("failed to create the item: %v", err)
		}
		fixture.artworkFor(t, broken.ID, uuid.NewString(), []byte("not an image"), time.Now())

		fixture.build(t)

		if bounds := fixture.collage(t).Bounds(); bounds.Dx() != width {
			t.Errorf("collage = %dx%d, want a full collage", bounds.Dx(), bounds.Dy())
		}
	})

	t.Run("survives the sweep that deletes the artwork the scan wrote", func(t *testing.T) {
		fixture := newFixture(t)
		fixture.title(t, color.RGBA{R: 200, G: 30, B: 30, A: 255})
		fixture.build(t)

		library := fixture.library(t)
		if err := fixture.items.DeleteImagesNotInPaths(context.Background(), fixture.libraryID, nil); err != nil {
			t.Fatalf("failed to sweep: %v", err)
		}

		if again := fixture.library(t).ImageTag; again != library.ImageTag {
			t.Errorf("image tag = %q, want it to stay %q", again, library.ImageTag)
		}
		if !fixture.stored(t, libraries.ImageKey(fixture.libraryID, library.ImageTag)) {
			t.Error("the sweep took the collage bytes with it")
		}
	})
}
