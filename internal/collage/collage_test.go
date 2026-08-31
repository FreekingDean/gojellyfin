package collage

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
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
		service:   New(records, files, stored),
		client:    client,
		items:     records,
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

	f.artworkFor(t, item.ID, painted(t, shade), time.Now().Add(-time.Duration(f.added)*time.Minute))

	return item.ID
}

func (f *fixture) artworkFor(t *testing.T, itemID uuid.UUID, body []byte, acquired time.Time) {
	t.Helper()

	ctx := context.Background()
	tag := uuid.NewString()
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

func (f *fixture) image(t *testing.T) []byte {
	t.Helper()

	body, ok := f.service.Image(context.Background(), f.libraryID)
	if !ok {
		t.Fatal("the library has no collage")
	}

	return body
}

func (f *fixture) decoded(t *testing.T, body []byte) image.Image {
	t.Helper()

	picture, _, err := image.Decode(bytes.NewReader(body))
	if err != nil {
		t.Fatalf("failed to decode the collage: %v", err)
	}

	return picture
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

func TestService_Image(t *testing.T) {
	t.Run("builds a collage sized to the card the client draws", func(t *testing.T) {
		fixture := newFixture(t)
		fixture.title(t, color.RGBA{R: 200, G: 30, B: 30, A: 255})

		bounds := fixture.decoded(t, fixture.image(t)).Bounds()
		if bounds.Dx() != width || bounds.Dy() != height {
			t.Errorf("collage = %dx%d, want %dx%d", bounds.Dx(), bounds.Dy(), width, height)
		}
	})

	t.Run("answers nothing for a library with no artwork", func(t *testing.T) {
		fixture := newFixture(t)

		if _, ok := fixture.service.Image(context.Background(), fixture.libraryID); ok {
			t.Error("a library with no posters was given a collage")
		}
	})

	t.Run("serves the second call from memory", func(t *testing.T) {
		fixture := newFixture(t)
		fixture.title(t, color.RGBA{R: 200, G: 30, B: 30, A: 255})

		first := fixture.image(t)
		fixture.title(t, color.RGBA{R: 30, G: 200, B: 30, A: 255})

		if second := fixture.image(t); !bytes.Equal(first, second) {
			t.Error("the collage was rebuilt while the entry was still held")
		}
	})

	t.Run("rebuilds once the entry has expired", func(t *testing.T) {
		fixture := newFixture(t)
		fixture.title(t, color.RGBA{R: 200, G: 30, B: 30, A: 255})

		first := fixture.image(t)
		fixture.title(t, color.RGBA{R: 30, G: 200, B: 30, A: 255})

		fixture.service.now = func() time.Time { return time.Now().Add(2 * lifetime) }

		if second := fixture.image(t); bytes.Equal(first, second) {
			t.Error("the expired collage was served again")
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
		fixture.artworkFor(t, broken.ID, []byte("not an image"), time.Now())

		if bounds := fixture.decoded(t, fixture.image(t)).Bounds(); bounds.Dx() != width {
			t.Errorf("collage = %dx%d, want a full collage", bounds.Dx(), bounds.Dy())
		}
	})
}
