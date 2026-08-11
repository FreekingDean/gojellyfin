package trailers

import (
	"context"
	"slices"
	"testing"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/libraries"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
	serveritems "github.com/FreekingDean/gojellyfin/internal/server/items"
	"github.com/FreekingDean/gojellyfin/internal/store"
	itemmodal "github.com/FreekingDean/gojellyfin/internal/store/item"
)

type fixture struct {
	server  *Server
	client  *store.Client
	library uuid.UUID
}

func newFixture(t *testing.T) *fixture {
	t.Helper()

	connection, err := store.NewStore()
	if err != nil {
		t.Fatalf("failed to open the database: %v", err)
	}
	if err := connection.Start(); err != nil {
		t.Fatalf("failed to reach the database, set DATABASE_URL: %v", err)
	}

	client := connection.Client()
	library, err := client.Library.Create().SetName(t.Name() + "-" + uuid.NewString()).Save(context.Background())
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

	server := New(serveritems.New(items.New(client), libraries.New(client)))

	return &fixture{server: server, client: client, library: library.ID}
}

func (f *fixture) add(t *testing.T, kind itemmodal.Kind, name string) {
	t.Helper()

	_, err := f.client.Item.Create().
		SetLibraryID(f.library).
		SetKind(kind).
		SetName(name).
		SetSortName(name).
		SetPath("/" + f.library.String() + "/" + name).
		Save(context.Background())
	if err != nil {
		t.Fatalf("failed to create %q: %v", name, err)
	}
}

func (f *fixture) get(t *testing.T, params api.GetTrailersParams) api.BaseItemDtoQueryResult {
	t.Helper()

	params.ParentId = &f.library
	params.Recursive = apiutil.Ptr(true)

	response, err := f.server.GetTrailers(context.Background(), api.GetTrailersRequestObject{Params: params})
	if err != nil {
		t.Fatalf("failed to get the trailers: %v", err)
	}

	result, ok := response.(api.GetTrailers200JSONResponse)
	if !ok {
		t.Fatalf("response = %T, want api.GetTrailers200JSONResponse", response)
	}

	return api.BaseItemDtoQueryResult(result)
}

func names(items []api.BaseItemDto) []string {
	found := make([]string, 0, len(items))
	for _, item := range items {
		found = append(found, *item.Name)
	}

	return found
}

func TestGetTrailers(t *testing.T) {
	fixture := newFixture(t)

	fixture.add(t, itemmodal.KindTrailer, "Alpha Trailer")
	fixture.add(t, itemmodal.KindTrailer, "Beta Trailer")
	fixture.add(t, itemmodal.KindTrailer, "Gamma Trailer")
	fixture.add(t, itemmodal.KindMovie, "A Movie")
	fixture.add(t, itemmodal.KindEpisode, "An Episode")

	t.Run("returns only trailers", func(t *testing.T) {
		result := fixture.get(t, api.GetTrailersParams{})

		want := []string{"Alpha Trailer", "Beta Trailer", "Gamma Trailer"}
		if got := names(*result.Items); !slices.Equal(got, want) {
			t.Errorf("trailers = %v, want %v", got, want)
		}
		if *result.TotalRecordCount != 3 {
			t.Errorf("TotalRecordCount = %d, want 3", *result.TotalRecordCount)
		}
	})

	t.Run("filters by search term", func(t *testing.T) {
		result := fixture.get(t, api.GetTrailersParams{SearchTerm: apiutil.Ptr("beta")})

		want := []string{"Beta Trailer"}
		if got := names(*result.Items); !slices.Equal(got, want) {
			t.Errorf("trailers = %v, want %v", got, want)
		}
	})

	t.Run("pages without changing the total", func(t *testing.T) {
		result := fixture.get(t, api.GetTrailersParams{
			StartIndex: apiutil.Ptr(int32(1)),
			Limit:      apiutil.Ptr(int32(1)),
		})

		want := []string{"Beta Trailer"}
		if got := names(*result.Items); !slices.Equal(got, want) {
			t.Errorf("trailers = %v, want %v", got, want)
		}
		if *result.TotalRecordCount != 3 {
			t.Errorf("TotalRecordCount = %d, want 3", *result.TotalRecordCount)
		}
		if *result.StartIndex != 1 {
			t.Errorf("StartIndex = %d, want 1", *result.StartIndex)
		}
	})
}
