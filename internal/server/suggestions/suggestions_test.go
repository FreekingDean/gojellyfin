package suggestions

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/env"
	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/libraries"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/store"
	itemmodal "github.com/FreekingDean/gojellyfin/internal/store/item"
)

type fixture struct {
	server  *Server
	client  *store.Client
	library uuid.UUID
	prefix  string
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

	client := connection.Client()
	prefix := t.Name() + "-" + uuid.NewString() + "-"
	library, err := client.Library.Create().SetName(prefix).Save(context.Background())
	if err != nil {
		t.Fatalf("failed to create the library: %v", err)
	}

	t.Cleanup(func() {
		ctx := context.Background()
		if _, err := client.Item.Delete().Where(itemmodal.LibraryID(library.ID)).Exec(ctx); err != nil {
			t.Errorf("failed to delete the items: %v", err)
		}
		if err := client.Library.DeleteOne(library).Exec(ctx); err != nil {
			t.Errorf("failed to delete the library: %v", err)
		}
		if err := connection.Stop(); err != nil {
			t.Errorf("failed to close the database: %v", err)
		}
	})

	server := New(items.New(client), libraries.New(client))

	return &fixture{server: server, client: client, library: library.ID, prefix: prefix}
}

func (f *fixture) add(t *testing.T, kind itemmodal.Kind, mediaType itemmodal.MediaType, name string) {
	t.Helper()

	_, err := f.client.Item.Create().
		SetLibraryID(f.library).
		SetKind(kind).
		SetMediaType(mediaType).
		SetName(f.prefix + name).
		SetSortName(f.prefix + name).
		SetPath("/" + f.library.String() + "/" + name).
		Save(context.Background())
	if err != nil {
		t.Fatalf("failed to create %q: %v", name, err)
	}
}

func (f *fixture) mine(t *testing.T, params api.GetSuggestionsParams) []string {
	t.Helper()

	response, err := f.server.GetSuggestions(context.Background(), api.GetSuggestionsRequestObject{Params: params})
	if err != nil {
		t.Fatalf("failed to get the suggestions: %v", err)
	}

	result, ok := response.(api.GetSuggestions200JSONResponse)
	if !ok {
		t.Fatalf("response = %T, want api.GetSuggestions200JSONResponse", response)
	}

	found := make([]string, 0, len(*result.Items))
	for _, item := range *result.Items {
		if name, ok := strings.CutPrefix(*item.Name, f.prefix); ok {
			found = append(found, name)
		}
	}
	slices.Sort(found)

	return found
}

func TestServer_GetSuggestions(t *testing.T) {
	fixture := newFixture(t)

	fixture.add(t, itemmodal.KindMovie, itemmodal.MediaTypeVideo, "Movie One")
	fixture.add(t, itemmodal.KindMovie, itemmodal.MediaTypeVideo, "Movie Two")
	fixture.add(t, itemmodal.KindSeries, itemmodal.MediaTypeUnknown, "Series")
	fixture.add(t, itemmodal.KindAudio, itemmodal.MediaTypeAudio, "Song")

	tests := []struct {
		name   string
		params api.GetSuggestionsParams
		want   []string
	}{
		{
			name:   "filters by item type",
			params: api.GetSuggestionsParams{Type: &[]api.BaseItemKind{api.BaseItemKindMovie}},
			want:   []string{"Movie One", "Movie Two"},
		},
		{
			name:   "filters by media type",
			params: api.GetSuggestionsParams{MediaType: &[]api.MediaType{api.MediaTypeAudio}},
			want:   []string{"Song"},
		},
		{
			name:   "returns every type when unfiltered",
			params: api.GetSuggestionsParams{},
			want:   []string{"Movie One", "Movie Two", "Series", "Song"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := fixture.mine(t, test.params); !slices.Equal(got, test.want) {
				t.Errorf("suggestions = %v, want %v", got, test.want)
			}
		})
	}
}
