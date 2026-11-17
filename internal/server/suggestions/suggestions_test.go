package suggestions

import (
	"context"
	"slices"
	"strings"
	"testing"

	librarymodel "github.com/FreekingDean/gojellyfin/internal/store/library"

	"github.com/FreekingDean/gojellyfin/internal/storetest"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/libraries"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/store"
	itemmodel "github.com/FreekingDean/gojellyfin/internal/store/item"
	playlistmodel "github.com/FreekingDean/gojellyfin/internal/store/playlist"
	"github.com/FreekingDean/gojellyfin/internal/users"
)

type fixture struct {
	server  *Server
	client  *store.Client
	library uuid.UUID
	prefix  string
}

func newFixture(t *testing.T) *fixture {
	t.Helper()

	connection := storetest.Open(t)

	client := connection.Client()
	prefix := t.Name() + "-" + uuid.NewString() + "-"
	library := storetest.Library(t, client, librarymodel.CollectionTypeMixed)

	server := New(items.New(client), libraries.New(client), allLibraries{})

	return &fixture{server: server, client: client, library: library.ID, prefix: prefix}
}

func (f *fixture) add(t *testing.T, kind itemmodel.Kind, mediaType playlistmodel.MediaType, name string) {
	t.Helper()

	_, err := f.client.Item.Create().
		SetKind(kind).
		SetName(f.prefix + name).
		SetSortName(f.prefix + name).
		SetKey("test:" + f.prefix + name).
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

	fixture.add(t, itemmodel.KindMovie, playlistmodel.MediaTypeVideo, "Movie One")
	fixture.add(t, itemmodel.KindMovie, playlistmodel.MediaTypeVideo, "Movie Two")
	fixture.add(t, itemmodel.KindSeries, playlistmodel.MediaTypeUnknown, "Series")
	fixture.add(t, itemmodel.KindAudio, playlistmodel.MediaTypeAudio, "Song")

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

type allLibraries struct{}

func (allLibraries) Access(context.Context, uuid.UUID) (users.Access, error) {
	return users.Access{All: true}, nil
}
