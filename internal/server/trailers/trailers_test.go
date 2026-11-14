package trailers

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/env"
	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/libraries"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
	"github.com/FreekingDean/gojellyfin/internal/store"
	itemmodel "github.com/FreekingDean/gojellyfin/internal/store/item"
	librarymembership "github.com/FreekingDean/gojellyfin/internal/store/libraryitem"
	downloadermodel "github.com/FreekingDean/gojellyfin/internal/store/source"
	"github.com/FreekingDean/gojellyfin/internal/users"
)

type fixture struct {
	server     *Server
	client     *store.Client
	library    uuid.UUID
	downloader uuid.UUID
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
	library, err := client.Library.Create().SetName(t.Name() + "-" + uuid.NewString()).Save(context.Background())
	if err != nil {
		t.Fatalf("failed to create the library: %v", err)
	}

	t.Cleanup(func() {
		ctx := context.Background()
		if _, err := client.Item.Delete().Where(itemmodel.HasLibrariesWith(librarymembership.LibraryID(library.ID))).Exec(ctx); err != nil {
			t.Errorf("failed to delete the items: %v", err)
		}
		if err := client.Library.DeleteOne(library).Exec(ctx); err != nil {
			t.Errorf("failed to delete the library: %v", err)
		}
		if err := connection.Stop(); err != nil {
			t.Errorf("failed to close the database: %v", err)
		}
	})

	server := New(items.New(client), libraries.New(client), allLibraries{})

	downloader, err := client.Source.Create().
		SetName(t.Name() + "-" + uuid.NewString()).
		SetURL("http://" + uuid.NewString() + ".invalid").
		SetAPIKeyVariable("SOURCE_API_KEY_TEST").
		SetKind(downloadermodel.KindRadarr).
		SetRootPath("/media").
		SetLocalPath("/media").
		Save(context.Background())
	if err != nil {
		t.Fatalf("failed to create the source: %v", err)
	}
	t.Cleanup(func() {
		if err := client.Source.DeleteOne(downloader).Exec(context.Background()); err != nil {
			t.Errorf("failed to delete the source: %v", err)
		}
	})

	return &fixture{
		downloader: downloader.ID, server: server, client: client, library: library.ID}
}

func (f *fixture) add(t *testing.T, kind itemmodel.Kind, name string) {
	t.Helper()

	record, err := f.client.Item.Create().
		SetKind(kind).
		SetName(name).
		SetSortName(name).
		SetKey("test:" + f.library.String() + ":" + name).
		Save(context.Background())
	if err != nil {
		t.Fatalf("failed to create %q: %v", name, err)
	}

	if err := f.client.LibraryItem.Create().
		SetLibraryID(f.library).
		SetSourceID(f.downloader).
		SetItemID(record.ID).
		Exec(context.Background()); err != nil {
		t.Fatalf("failed to place %q in the library: %v", name, err)
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

func TestServer_GetTrailers(t *testing.T) {
	fixture := newFixture(t)

	fixture.add(t, itemmodel.KindMovie, "A Movie")
	fixture.add(t, itemmodel.KindEpisode, "An Episode")

	result := fixture.get(t, api.GetTrailersParams{})

	if got := names(*result.Items); len(got) != 0 {
		t.Errorf("trailers = %v, want none: nothing stores a trailer", got)
	}
	if *result.TotalRecordCount != 0 {
		t.Errorf("TotalRecordCount = %d, want 0", *result.TotalRecordCount)
	}
}

type allLibraries struct{}

func (allLibraries) Access(context.Context, uuid.UUID) (users.Access, error) {
	return users.Access{All: true}, nil
}
