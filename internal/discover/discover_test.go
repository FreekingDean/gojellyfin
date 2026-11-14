package discover

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/FreekingDean/gojellyfin/internal/items"
	imagemodel "github.com/FreekingDean/gojellyfin/internal/store/image"
	itemmodel "github.com/FreekingDean/gojellyfin/internal/store/item"
)

var errUnreachable = errors.New("the provider is unreachable")

type stubProvider struct {
	disabled bool
	trending []int
	failing  bool
	asked    []string
}

func (s *stubProvider) Enabled() bool { return !s.disabled }

func summary(tmdbID int) items.Metadata {
	name := "Title " + strconv.Itoa(tmdbID)
	ids := map[string]string{providerKey: strconv.Itoa(tmdbID)}

	return items.Metadata{
		Name:        &name,
		ProviderIds: &ids,
		Images:      []items.RemoteImage{{Kind: imagemodel.KindPrimary, URL: "https://image.invalid/" + strconv.Itoa(tmdbID) + ".jpg"}},
	}
}

func (s *stubProvider) detail(what string, tmdbID int) (items.Metadata, bool, error) {
	s.asked = append(s.asked, fmt.Sprintf("%s:%d", what, tmdbID))
	if s.failing {
		return items.Metadata{}, false, errUnreachable
	}
	if tmdbID == 404 {
		return items.Metadata{}, false, nil
	}

	metadata := summary(tmdbID)
	overview := "The whole record"
	metadata.Overview = &overview

	return metadata, true, nil
}

func (s *stubProvider) Movie(_ context.Context, tmdbID int) (items.Metadata, bool, error) {
	return s.detail("movie", tmdbID)
}

func (s *stubProvider) Series(_ context.Context, tmdbID int) (items.Metadata, bool, error) {
	return s.detail("series", tmdbID)
}

func (s *stubProvider) Trending(_ context.Context, kind items.Kind) ([]items.Metadata, error) {
	s.asked = append(s.asked, "trending:"+string(kind))
	if s.failing {
		return nil, errUnreachable
	}

	titles := make([]items.Metadata, 0, len(s.trending)+1)
	for _, tmdbID := range s.trending {
		titles = append(titles, summary(tmdbID))
	}

	return append(titles, items.Metadata{}), nil
}

func (s *stubProvider) Search(_ context.Context, kind items.Kind, term string) ([]items.Metadata, error) {
	s.asked = append(s.asked, "search:"+string(kind)+":"+term)
	if s.failing {
		return nil, errUnreachable
	}

	titles := make([]items.Metadata, 0, len(s.trending))
	for _, tmdbID := range s.trending {
		titles = append(titles, summary(tmdbID))
	}

	return titles, nil
}

type held []string

func (h held) HeldKeys(_ context.Context, keys []string) ([]string, error) {
	owned := []string{}
	for _, key := range keys {
		if slices.Contains(h, key) {
			owned = append(owned, key)
		}
	}

	return owned, nil
}

type clock struct{ at time.Time }

func (c *clock) now() time.Time { return c.at }

func ids(titles []Title) []uint32 {
	found := make([]uint32, 0, len(titles))
	for _, title := range titles {
		found = append(found, title.TmdbID)
	}

	return found
}

func TestService_Trending(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name  string
		held  held
		limit int
		want  []uint32
	}{
		{name: "everything the library lacks, in the provider's order", want: []uint32{603, 604, 605}},
		{name: "without a title the library holds", held: held{"movie:tmdb:604"}, want: []uint32{603, 605}},
		{name: "a series sharing the id does not hide the movie", held: held{"series:tmdb:604"}, want: []uint32{603, 604, 605}},
		{name: "the limit counts what is left after the library's own", held: held{"movie:tmdb:603"}, limit: 2, want: []uint32{604, 605}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			provider := &stubProvider{trending: []int{603, 604, 605}}
			service := newService(provider, test.held, time.Now)

			titles, err := service.Trending(ctx, itemmodel.KindMovie, test.limit)
			if err != nil {
				t.Fatalf("failed to read what is trending: %v", err)
			}
			if got := ids(titles); !slices.Equal(got, test.want) {
				t.Errorf("trending = %v, want %v", got, test.want)
			}
			for _, title := range titles {
				if title.ID != ID(itemmodel.KindMovie, title.TmdbID) || title.Key != items.MovieKey(int(title.TmdbID)) {
					t.Errorf("title %d carries id %s and key %q", title.TmdbID, title.ID, title.Key)
				}
			}
		})
	}
}

func TestService_Trending_AsksOncePerHour(t *testing.T) {
	ctx := context.Background()
	provider := &stubProvider{trending: []int{603}}
	clock := &clock{at: time.Date(2026, time.November, 14, 12, 0, 0, 0, time.UTC)}
	service := newService(provider, held{}, clock.now)

	for range 3 {
		if _, err := service.Trending(ctx, itemmodel.KindMovie, 0); err != nil {
			t.Fatalf("failed to read what is trending: %v", err)
		}
	}
	if got := provider.asked; !slices.Equal(got, []string{"trending:Movie"}) {
		t.Fatalf("asked = %v, want one request", got)
	}

	clock.at = clock.at.Add(trendingFor)
	provider.trending = []int{700}
	titles, err := service.Trending(ctx, itemmodel.KindMovie, 0)
	if err != nil {
		t.Fatalf("failed to read what is trending: %v", err)
	}
	if got := ids(titles); !slices.Equal(got, []uint32{700}) {
		t.Errorf("trending after an hour = %v, want the new list", got)
	}

	clock.at = clock.at.Add(trendingFor)
	provider.failing = true
	titles, err = service.Trending(ctx, itemmodel.KindMovie, 0)
	if err != nil {
		t.Fatalf("a provider that stopped answering failed the row: %v", err)
	}
	if got := ids(titles); !slices.Equal(got, []uint32{700}) {
		t.Errorf("trending with the provider down = %v, want the last list", got)
	}
}

func TestService_Trending_FailsWithNothingToServe(t *testing.T) {
	service := newService(&stubProvider{failing: true}, held{}, time.Now)

	if _, err := service.Trending(context.Background(), itemmodel.KindMovie, 0); !errors.Is(err, errUnreachable) {
		t.Errorf("err = %v, want the provider's", err)
	}
}

func TestService_Title(t *testing.T) {
	ctx := context.Background()

	t.Run("a summary is answered from the trending list without a request", func(t *testing.T) {
		provider := &stubProvider{trending: []int{603}}
		service := newService(provider, held{}, time.Now)
		if _, err := service.Trending(ctx, itemmodel.KindMovie, 0); err != nil {
			t.Fatalf("failed to read what is trending: %v", err)
		}

		title, found, err := service.Summary(ctx, ID(itemmodel.KindMovie, 603))
		if err != nil || !found {
			t.Fatalf("summary = %t, %v", found, err)
		}
		if _, pictured := title.Image(imagemodel.KindPrimary); !pictured {
			t.Error("the summary carries no poster")
		}
		if got := provider.asked; !slices.Equal(got, []string{"trending:Movie"}) {
			t.Errorf("asked = %v, want only the list", got)
		}
	})

	t.Run("the whole record is fetched once and kept", func(t *testing.T) {
		provider := &stubProvider{trending: []int{603}}
		service := newService(provider, held{}, time.Now)
		if _, err := service.Trending(ctx, itemmodel.KindMovie, 0); err != nil {
			t.Fatalf("failed to read what is trending: %v", err)
		}

		for range 2 {
			title, found, err := service.Title(ctx, ID(itemmodel.KindMovie, 603))
			if err != nil || !found {
				t.Fatalf("title = %t, %v", found, err)
			}
			if title.Metadata.Overview == nil {
				t.Error("the title carries the summary rather than the record")
			}
		}
		if _, err := service.Trending(ctx, itemmodel.KindMovie, 0); err != nil {
			t.Fatalf("failed to read what is trending: %v", err)
		}
		title, _, _ := service.Summary(ctx, ID(itemmodel.KindMovie, 603))
		if title.Metadata.Overview == nil {
			t.Error("the record was replaced by a summary")
		}
		if got := provider.asked; !slices.Equal(got, []string{"trending:Movie", "movie:603"}) {
			t.Errorf("asked = %v, want the list and one record", got)
		}
	})

	t.Run("a series is asked for as a series", func(t *testing.T) {
		provider := &stubProvider{}
		service := newService(provider, held{}, time.Now)

		title, found, err := service.Title(ctx, ID(itemmodel.KindSeries, 1396))
		if err != nil || !found || title.Key != "series:tmdb:1396" {
			t.Fatalf("title = %+v, %t, %v", title, found, err)
		}
		if got := provider.asked; !slices.Equal(got, []string{"series:1396"}) {
			t.Errorf("asked = %v", got)
		}
	})

	t.Run("an id this package did not write asks nothing", func(t *testing.T) {
		provider := &stubProvider{}
		service := newService(provider, held{}, time.Now)

		if _, found, err := service.Title(ctx, prefix); found || err != nil {
			t.Errorf("title = %t, %v, want a miss", found, err)
		}
		if len(provider.asked) != 0 {
			t.Errorf("asked = %v, want nothing", provider.asked)
		}
	})

	t.Run("a title the provider does not know is a miss", func(t *testing.T) {
		service := newService(&stubProvider{}, held{}, time.Now)

		if _, found, err := service.Title(ctx, ID(itemmodel.KindMovie, 404)); found || err != nil {
			t.Errorf("title = %t, %v, want a miss", found, err)
		}
	})

	t.Run("a disabled provider is a miss", func(t *testing.T) {
		provider := &stubProvider{disabled: true}
		service := newService(provider, held{}, time.Now)

		if _, found, err := service.Title(ctx, ID(itemmodel.KindMovie, 603)); found || err != nil {
			t.Errorf("title = %t, %v, want a miss", found, err)
		}
		if len(provider.asked) != 0 {
			t.Errorf("asked = %v, want nothing", provider.asked)
		}
	})

	t.Run("a provider that fails is an error", func(t *testing.T) {
		service := newService(&stubProvider{failing: true}, held{}, time.Now)

		if _, _, err := service.Title(ctx, ID(itemmodel.KindMovie, 603)); !errors.Is(err, errUnreachable) {
			t.Errorf("err = %v, want the provider's", err)
		}
	})
}

func TestTitled_SkipsAnIDItCannotCarry(t *testing.T) {
	for name, id := range map[string]string{
		"an id past thirty-two bits": "4294967296",
		"zero":                       "0",
		"a negative id":              "-603",
		"not a number":               "tt0133093",
	} {
		ids := map[string]string{providerKey: id}
		if title, ok := titled(itemmodel.KindMovie, items.Metadata{ProviderIds: &ids}); ok {
			t.Errorf("%s became %s", name, title.ID)
		}
	}
}

func TestService_Search(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name     string
		provider *stubProvider
		term     string
		want     []uint32
		asked    []string
		failing  bool
	}{
		{
			name:     "what matches and the library lacks",
			provider: &stubProvider{trending: []int{603, 604}},
			term:     "matrix",
			want:     []uint32{603},
			asked:    []string{"search:Movie:matrix"},
		},
		{name: "a blank term asks nothing", provider: &stubProvider{trending: []int{603}}, term: "  ", want: []uint32{}},
		{name: "a disabled provider asks nothing", provider: &stubProvider{disabled: true, trending: []int{603}}, term: "matrix", want: []uint32{}},
		{
			name:     "a provider that does not answer is an error",
			provider: &stubProvider{failing: true},
			term:     "matrix",
			asked:    []string{"search:Movie:matrix"},
			failing:  true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := newService(test.provider, held{"movie:tmdb:604"}, time.Now)

			titles, err := service.Search(ctx, itemmodel.KindMovie, test.term, 0)
			if (err != nil) != test.failing {
				t.Fatalf("error = %v, want one: %v", err, test.failing)
			}
			if got := ids(titles); !test.failing && !slices.Equal(got, test.want) {
				t.Errorf("found = %v, want %v", got, test.want)
			}
			if !slices.Equal(test.provider.asked, test.asked) {
				t.Errorf("asked = %v, want %v", test.provider.asked, test.asked)
			}
		})
	}

	t.Run("a found title's poster needs no further request", func(t *testing.T) {
		provider := &stubProvider{trending: []int{603}}
		service := newService(provider, held{}, time.Now)

		titles, err := service.Search(ctx, itemmodel.KindMovie, "matrix", 0)
		if err != nil || len(titles) != 1 {
			t.Fatalf("found = %v, %v", titles, err)
		}
		if _, found, err := service.Summary(ctx, titles[0].ID); err != nil || !found {
			t.Fatalf("summary = %v, %v", found, err)
		}
		if !slices.Equal(provider.asked, []string{"search:Movie:matrix"}) {
			t.Errorf("asked = %v", provider.asked)
		}
	})
}
