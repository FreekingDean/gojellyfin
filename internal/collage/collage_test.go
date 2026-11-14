package collage

import (
	"bytes"
	"context"
	"encoding/hex"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/env"
	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/libraries"
	"github.com/FreekingDean/gojellyfin/internal/shelves"
	"github.com/FreekingDean/gojellyfin/internal/store"
	imagemodel "github.com/FreekingDean/gojellyfin/internal/store/image"
	itemmodel "github.com/FreekingDean/gojellyfin/internal/store/item"
	sourcemodel "github.com/FreekingDean/gojellyfin/internal/store/source"
	"github.com/FreekingDean/gojellyfin/internal/users"
)

type fixture struct {
	service   *Service
	client    *store.Client
	libraryID uuid.UUID
	sourceID  uuid.UUID
	cdn       *httptest.Server
	posters   map[string][]byte
	fetched   atomic.Int32
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

	library, err := client.Library.Create().SetName(t.Name() + "-" + uuid.NewString()).Save(ctx)
	if err != nil {
		t.Fatalf("failed to create the library: %v", err)
	}

	downloader, err := client.Source.Create().
		SetName(t.Name() + "-" + uuid.NewString()).
		SetURL("http://" + uuid.NewString() + ".invalid").
		SetAPIKeyVariable("SOURCE_API_KEY_TEST").
		SetKind(sourcemodel.KindRadarr).
		Save(ctx)
	if err != nil {
		t.Fatalf("failed to create the source: %v", err)
	}

	f := &fixture{
		service:   newService(items.New(client), libraries.New(client), unshelved{}, everything{}, time.Now),
		client:    client,
		libraryID: library.ID,
		sourceID:  downloader.ID,
		posters:   map[string][]byte{},
	}

	f.cdn = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.fetched.Add(1)
		body, ok := f.posters[r.URL.Path]
		if !ok {
			http.NotFound(w, r)

			return
		}
		_, _ = w.Write(body)
	}))

	t.Cleanup(func() {
		f.cdn.Close()
		if _, err := client.Item.Delete().Where(itemmodel.KeyHasPrefix("collage:" + library.ID.String())).Exec(ctx); err != nil {
			t.Errorf("failed to delete the items: %v", err)
		}
		if err := client.Source.DeleteOne(downloader).Exec(ctx); err != nil {
			t.Errorf("failed to delete the source: %v", err)
		}
		if err := client.Library.DeleteOne(library).Exec(ctx); err != nil {
			t.Errorf("failed to delete the library: %v", err)
		}
		if err := connection.Stop(); err != nil {
			t.Errorf("failed to close the database: %v", err)
		}
	})

	return f
}

func (f *fixture) title(t *testing.T, poster []byte) uuid.UUID {
	t.Helper()

	ctx := context.Background()
	f.added++
	name := "Title " + uuid.NewString()

	item, err := f.client.Item.Create().
		SetKind(itemmodel.KindMovie).
		SetName(name).
		SetSortName(name).
		SetKey("collage:" + f.libraryID.String() + ":" + name).
		Save(ctx)
	if err != nil {
		t.Fatalf("failed to create the item: %v", err)
	}

	if err := f.client.LibraryItem.Create().
		SetLibraryID(f.libraryID).
		SetSourceID(f.sourceID).
		SetItemID(item.ID).
		Exec(ctx); err != nil {
		t.Fatalf("failed to place the item in the library: %v", err)
	}

	path := "/t/p/w780/" + uuid.NewString() + ".png"
	f.posters[path] = poster

	if _, err := f.client.Image.Create().
		SetItemID(item.ID).
		SetKind(imagemodel.KindPrimary).
		SetURL(f.cdn.URL + path).
		SetTag(uuid.NewString()).
		SetCreatedAt(time.Now().Add(time.Duration(f.added) * time.Minute)).
		Save(ctx); err != nil {
		t.Fatalf("failed to create the image row: %v", err)
	}

	return item.ID
}

func (f *fixture) image(t *testing.T) []byte {
	t.Helper()

	built, ok := f.service.Image(context.Background(), f.libraryID, "")
	if !ok {
		t.Fatal("the library has no collage")
	}

	return built.Body
}

func (f *fixture) tag(t *testing.T) string {
	t.Helper()

	tag, err := f.service.Tag(context.Background(), f.libraryID, uuid.Nil)
	if err != nil {
		t.Fatalf("failed to read the tag: %v", err)
	}

	return tag
}

func opened(t *testing.T, body []byte) image.Image {
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

var (
	red   = color.RGBA{R: 200, G: 30, B: 30, A: 255}
	green = color.RGBA{R: 30, G: 200, B: 30, A: 255}
)

func TestService_Image(t *testing.T) {
	t.Run("builds a collage sized to the card the client draws", func(t *testing.T) {
		fixture := newFixture(t)
		fixture.title(t, painted(t, red))

		bounds := opened(t, fixture.image(t)).Bounds()
		if bounds.Dx() != width || bounds.Dy() != height {
			t.Errorf("collage = %dx%d, want %dx%d", bounds.Dx(), bounds.Dy(), width, height)
		}
	})

	t.Run("answers nothing for a library with no posters", func(t *testing.T) {
		fixture := newFixture(t)

		if _, ok := fixture.service.Image(context.Background(), fixture.libraryID, ""); ok {
			t.Error("a library with no posters was given a collage")
		}
		if tag := fixture.tag(t); tag != "" {
			t.Errorf("tag = %q, want none", tag)
		}
	})

	t.Run("serves the same posters from memory without fetching them again", func(t *testing.T) {
		fixture := newFixture(t)
		fixture.title(t, painted(t, red))

		first := fixture.image(t)
		fetched := fixture.fetched.Load()

		if second := fixture.image(t); !bytes.Equal(first, second) {
			t.Error("the collage changed while its posters did not")
		}
		if again := fixture.fetched.Load(); again != fetched {
			t.Errorf("fetched %d posters for a collage already held", again-fetched)
		}
	})

	t.Run("rebuilds and retags once a library gains a poster", func(t *testing.T) {
		fixture := newFixture(t)
		fixture.title(t, painted(t, red))

		first, before := fixture.image(t), fixture.tag(t)
		fixture.title(t, painted(t, green))

		if after := fixture.tag(t); after == before {
			t.Error("the tag did not change, so a client would keep the old picture")
		}
		if second := fixture.image(t); bytes.Equal(first, second) {
			t.Error("the collage was not rebuilt for the new poster")
		}
	})

	t.Run("steps past a poster it cannot fetch or decode, and does not hold the result", func(t *testing.T) {
		fixture := newFixture(t)
		fixture.title(t, painted(t, red))
		fixture.title(t, []byte("not an image"))

		built, ok := fixture.service.Image(context.Background(), fixture.libraryID, "")
		if !ok {
			t.Fatal("the library has no collage")
		}
		if built.Complete {
			t.Error("a collage missing a poster was called complete")
		}
		if bounds := opened(t, built.Body).Bounds(); bounds.Dx() != width {
			t.Errorf("collage = %dx%d, want a full collage", bounds.Dx(), bounds.Dy())
		}

		fetched := fixture.fetched.Load()
		fixture.image(t)
		if fixture.fetched.Load() == fetched {
			t.Error("an incomplete collage was held, so the missing poster would never be retried")
		}
	})

	t.Run("answers nothing when no poster can be fetched", func(t *testing.T) {
		fixture := newFixture(t)
		fixture.title(t, []byte("not an image"))

		if _, ok := fixture.service.Image(context.Background(), fixture.libraryID, ""); ok {
			t.Error("a collage was built out of nothing")
		}
	})

	t.Run("fetches the posters from the urls the provider answered", func(t *testing.T) {
		fixture := newFixture(t)
		fixture.title(t, painted(t, red))

		front := opened(t, fixture.image(t)).At(850, height/2)
		r, g, b, _ := front.RGBA()
		if r>>8 < 150 || g>>8 > 80 || b>>8 > 80 {
			t.Errorf("front = %v, want the red poster", front)
		}
	})

	t.Run("retags once what the library added ages out of the week", func(t *testing.T) {
		fixture := newFixture(t)
		fixture.title(t, painted(t, red))

		now := fixture.tag(t)
		later := newService(items.New(fixture.client), libraries.New(fixture.client), unshelved{}, everything{}, func() time.Time { return time.Now().Add(shelves.FreshWindow + time.Hour) })
		aged, err := later.Tag(context.Background(), fixture.libraryID, uuid.Nil)
		if err != nil {
			t.Fatalf("failed to read the aged tag: %v", err)
		}
		if aged == "" || aged == now {
			t.Errorf("tag a week on = %q, want a new tag beside %q", aged, now)
		}
	})
}

type unshelved struct{}

func (unshelved) Card(context.Context, uuid.UUID, items.Viewer, uuid.UUID, int) (shelves.Card, bool, error) {
	return shelves.Card{}, false, nil
}

type everything struct{}

func (everything) Access(context.Context, uuid.UUID) (users.Access, error) {
	return users.Access{All: true}, nil
}

type shelved struct {
	id    uuid.UUID
	card  shelves.Card
	owner uuid.UUID
}

func (s shelved) Card(_ context.Context, id uuid.UUID, viewer items.Viewer, userID uuid.UUID, _ int) (shelves.Card, bool, error) {
	if id != s.id || !viewer.All || s.card.Personal && userID != s.owner {
		return shelves.Card{}, false, nil
	}

	return s.card, true, nil
}

func TestService_Shelf(t *testing.T) {
	fixture := newFixture(t)
	ctx := context.Background()
	fixture.title(t, painted(t, red))
	fixture.title(t, painted(t, green))

	posters, err := items.New(fixture.client).LibraryPosters(ctx, fixture.libraryID, stacked)
	if err != nil {
		t.Fatalf("failed to read the library's posters: %v", err)
	}

	shelfID := uuid.New()
	card := shelves.Card{Name: "Recently Aired", Fresh: 3, Posters: posters}
	service := newService(items.New(fixture.client), libraries.New(fixture.client), shelved{id: shelfID, card: card}, everything{}, time.Now)

	tag, err := service.Tag(ctx, shelfID, uuid.Nil)
	if err != nil {
		t.Fatalf("failed to read the shelf's tag: %v", err)
	}
	if tag == "" {
		t.Fatal("the shelf has no tag")
	}

	card.Fresh = 4
	recounted, err := newService(items.New(fixture.client), libraries.New(fixture.client), shelved{id: shelfID, card: card}, everything{}, time.Now).Tag(ctx, shelfID, uuid.Nil)
	if err != nil {
		t.Fatalf("failed to read the recounted tag: %v", err)
	}
	if recounted == tag {
		t.Error("a new count kept the old tag, so a client would keep the old card")
	}

	built, ok := service.Image(ctx, shelfID, tag)
	if !ok || !built.Complete {
		t.Fatalf("shelf card = %v complete %v, want a complete card", ok, built.Complete)
	}
	if bytes.Equal(built.Body, fixture.image(t)) {
		t.Error("the shelf's card is the library's, want one titled with the shelf's name")
	}

	drawn := opened(t, built.Body)
	front := drawn.At(850, height/2)
	if r, g, b, _ := front.RGBA(); g>>8 < 150 || r>>8 > 80 || b>>8 > 80 {
		t.Errorf("front = %v, want the newest poster, which is green", front)
	}
	corner := drawn.At(4, height-4)
	if r, g, b, _ := corner.RGBA(); r>>8 > 40 || g>>8 > 40 || b>>8 > 40 {
		t.Errorf("corner = %v, want the dark side the title sits on", corner)
	}
}

func TestService_PersonalShelf(t *testing.T) {
	fixture := newFixture(t)
	ctx := context.Background()
	fixture.title(t, painted(t, red))
	fixture.title(t, painted(t, green))

	posters, err := items.New(fixture.client).LibraryPosters(ctx, fixture.libraryID, stacked)
	if err != nil {
		t.Fatalf("failed to read the library's posters: %v", err)
	}

	shelfID := uuid.New()
	owner := uuid.New()
	card := shelves.Card{Name: "Because You Watched The Godfather Part II", Posters: posters, Personal: true}
	service := newService(items.New(fixture.client), libraries.New(fixture.client), shelved{id: shelfID, card: card, owner: owner}, everything{}, time.Now)

	tag, err := service.Tag(ctx, shelfID, owner)
	if err != nil {
		t.Fatalf("failed to read the shelf's tag: %v", err)
	}
	if !strings.HasPrefix(tag, hex.EncodeToString(owner[:])+"-") {
		t.Fatalf("tag = %q, want one naming the user it was drawn for", tag)
	}

	if anonymous, err := service.Tag(ctx, shelfID, uuid.Nil); err != nil || anonymous != "" {
		t.Errorf("tag with nobody signed in = %q, %v, want none", anonymous, err)
	}

	built, ok := service.Image(ctx, shelfID, tag)
	if !ok || !built.Complete {
		t.Fatalf("card = %v complete %v, want the user's card", ok, built.Complete)
	}

	_, fingerprint, _ := strings.Cut(tag, "-")
	other := uuid.New()
	stranger := hex.EncodeToString(other[:])
	for name, asked := range map[string]string{
		"no tag":               "",
		"the bare fingerprint": fingerprint,
		"another user's":       stranger + "-" + fingerprint,
		"a stale fingerprint":  hex.EncodeToString(owner[:]) + "-" + strings.Repeat("0", len(fingerprint)),
	} {
		if _, ok := service.Image(ctx, shelfID, asked); ok {
			t.Errorf("%s answered a card, want none since only the tag the user was handed draws theirs", name)
		}
	}
}
