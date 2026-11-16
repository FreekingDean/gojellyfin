package userviews

import (
	"context"
	"github.com/FreekingDean/gojellyfin/internal/cards"
	"github.com/FreekingDean/gojellyfin/internal/channels"
	"github.com/FreekingDean/gojellyfin/internal/discover"
	"github.com/FreekingDean/gojellyfin/internal/items"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/config"
	"github.com/FreekingDean/gojellyfin/internal/env"
	"github.com/FreekingDean/gojellyfin/internal/libraries"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
	"github.com/FreekingDean/gojellyfin/internal/shelves"
	"github.com/FreekingDean/gojellyfin/internal/store"
	librarymodel "github.com/FreekingDean/gojellyfin/internal/store/library"
	"github.com/FreekingDean/gojellyfin/internal/users"
)

type fixture struct {
	server *Server
	client *store.Client
	prefix string
}

func newFixture(t *testing.T) *fixture {
	t.Helper()

	environment, err := env.Load()
	if err != nil {
		t.Fatalf("failed to read the environment: %v", err)
	}

	connection, err := store.NewStore(environment)
	if err != nil {
		t.Fatalf("failed to open the database: %v", err)
	}
	if err := connection.Start(); err != nil {
		t.Fatalf("failed to reach the database, set DATABASE_URL: %v", err)
	}

	client := connection.Client()
	prefix := t.Name() + "-" + uuid.NewString() + "-"

	t.Cleanup(func() {
		ctx := context.Background()
		if _, err := client.Library.Delete().Where(librarymodel.NameHasPrefix(prefix)).Exec(ctx); err != nil {
			t.Errorf("failed to delete the libraries: %v", err)
		}
		if err := connection.Stop(); err != nil {
			t.Errorf("failed to close the database: %v", err)
		}
	})

	catalogue := items.New(client)
	shown := shelves.New(catalogue, discover.New(noProvider{}, catalogue), config.New(client))

	return &fixture{server: New(libraries.New(client), shown, channels.New(client, catalogue), users.New(client), allLibraries{}, cards.New(catalogue, libraries.New(client), shown, users.New(client))), client: client, prefix: prefix}
}

type noProvider struct{}

func (noProvider) Enabled() bool { return false }

func (noProvider) Movie(context.Context, int) (items.Metadata, bool, error) {
	return items.Metadata{}, false, nil
}

func (noProvider) Series(context.Context, int) (items.Metadata, bool, error) {
	return items.Metadata{}, false, nil
}

func (noProvider) Trending(context.Context, items.Kind, int) ([]items.Metadata, error) {
	return nil, nil
}

func (noProvider) Search(context.Context, items.Kind, string) ([]items.Metadata, error) {
	return nil, nil
}

type allLibraries struct{}

func (allLibraries) Access(context.Context, uuid.UUID) (users.Access, error) {
	return users.Access{All: true}, nil
}

type someLibraries []uuid.UUID

func (s someLibraries) Access(context.Context, uuid.UUID) (users.Access, error) {
	return users.Access{Libraries: s}, nil
}

func (f *fixture) add(t *testing.T, collectionType librarymodel.CollectionType, name string) uuid.UUID {
	t.Helper()

	library, err := f.client.Library.Create().
		SetName(f.prefix + name).
		SetCollectionType(collectionType).
		Save(context.Background())
	if err != nil {
		t.Fatalf("failed to create the library %q: %v", name, err)
	}

	return library.ID
}

func TestServer_GetUserViews(t *testing.T) {
	fixture := newFixture(t)

	id := fixture.add(t, librarymodel.CollectionTypeMovies, "Feature Films")

	response, err := fixture.server.GetUserViews(context.Background(), api.GetUserViewsRequestObject{})
	if err != nil {
		t.Fatalf("failed to get the user views: %v", err)
	}

	result, ok := response.(api.GetUserViews200JSONResponse)
	if !ok {
		t.Fatalf("response = %T, want api.GetUserViews200JSONResponse", response)
	}

	var view api.BaseItemDto
	for _, candidate := range *result.Items {
		if *candidate.Id == id {
			view = candidate
		}
	}
	if view.Id == nil {
		t.Fatalf("views do not contain the library %v", id)
	}

	want := api.BaseItemDto{
		Id:                   &id,
		ServerId:             apiutil.Ptr(config.ServerID),
		Name:                 apiutil.Ptr(fixture.prefix + "Feature Films"),
		SortName:             apiutil.Ptr(strings.ToLower(fixture.prefix + "Feature Films")),
		Type:                 apiutil.Ptr(api.BaseItemKindCollectionFolder),
		CollectionType:       apiutil.Ptr(api.CollectionType(librarymodel.CollectionTypeMovies)),
		DisplayPreferencesId: apiutil.Ptr(strings.ReplaceAll(id.String(), "-", "")),
		IsFolder:             apiutil.Ptr(true),
		LocationType:         apiutil.Ptr(api.FileSystem),
		ImageTags:            &map[string]*string{},
		BackdropImageTags:    &[]string{},
	}
	if !reflect.DeepEqual(view, want) {
		t.Errorf("view = %+v, want %+v", view, want)
	}
}

func TestServer_GetUserViews_HidesLibrariesTheUserCannotSee(t *testing.T) {
	fixture := newFixture(t)

	allowed := fixture.add(t, librarymodel.CollectionTypeMovies, "Allowed Films")
	hidden := fixture.add(t, librarymodel.CollectionTypeTvshows, "Hidden Shows")
	fixture.server.policies = someLibraries{allowed}

	response, err := fixture.server.GetUserViews(context.Background(), api.GetUserViewsRequestObject{})
	if err != nil {
		t.Fatalf("failed to get the user views: %v", err)
	}

	var ids []uuid.UUID
	for _, view := range *response.(api.GetUserViews200JSONResponse).Items {
		ids = append(ids, *view.Id)
	}
	if !slices.Contains(ids, allowed) {
		t.Errorf("views %v do not contain the library the user may see", ids)
	}
	if slices.Contains(ids, hidden) {
		t.Errorf("views %v contain a library the user may not see", ids)
	}
}

func TestServer_GetUserViews_Shelves(t *testing.T) {
	fixture := newFixture(t)

	response, err := fixture.server.GetUserViews(context.Background(), api.GetUserViewsRequestObject{})
	if err != nil {
		t.Fatalf("failed to get the user views: %v", err)
	}

	result, ok := response.(api.GetUserViews200JSONResponse)
	if !ok {
		t.Fatalf("response = %T, want api.GetUserViews200JSONResponse", response)
	}

	found := map[uuid.UUID]api.BaseItemDto{}
	for _, view := range *result.Items {
		found[*view.Id] = view
	}

	for slug, want := range map[string]struct {
		name       string
		collection librarymodel.CollectionType
	}{
		"recently-aired": {name: "Recently Aired", collection: librarymodel.CollectionTypeTvshows},
		"new-releases":   {name: "New Releases", collection: librarymodel.CollectionTypeMovies},
	} {
		view, ok := found[shelves.ID(slug)]
		if !ok {
			t.Errorf("views do not contain the %s shelf", slug)
			continue
		}
		if *view.Name != want.name || *view.CollectionType != api.CollectionType(want.collection) || *view.LocationType != api.Virtual {
			t.Errorf("%s view = %s %s %s, want %s %s Virtual", slug, *view.Name, *view.CollectionType, *view.LocationType, want.name, want.collection)
		}
	}
}

func TestServer_GetGroupingOptions(t *testing.T) {
	fixture := newFixture(t)

	movies := fixture.add(t, librarymodel.CollectionTypeMovies, "Movies")
	shows := fixture.add(t, librarymodel.CollectionTypeTvshows, "Shows")
	mixed := fixture.add(t, librarymodel.CollectionTypeMixed, "Mixed")
	fixture.add(t, librarymodel.CollectionTypeMusic, "Music")
	fixture.add(t, librarymodel.CollectionTypeBooks, "Books")

	response, err := fixture.server.GetGroupingOptions(context.Background(), api.GetGroupingOptionsRequestObject{})
	if err != nil {
		t.Fatalf("failed to get the grouping options: %v", err)
	}

	result, ok := response.(api.GetGroupingOptions200JSONResponse)
	if !ok {
		t.Fatalf("response = %T, want api.GetGroupingOptions200JSONResponse", response)
	}

	names := make([]string, 0, len(result))
	ids := map[string]string{}
	for _, option := range result {
		name, mine := strings.CutPrefix(*option.Name, fixture.prefix)
		if !mine {
			continue
		}
		names = append(names, name)
		ids[name] = *option.Id
	}

	want := []string{"Mixed", "Movies", "Shows"}
	if !slices.Equal(names, want) {
		t.Errorf("options = %v, want %v", names, want)
	}
	if ids["Movies"] != movies.String() || ids["Shows"] != shows.String() || ids["Mixed"] != mixed.String() {
		t.Errorf("ids = %v, want the library ids", ids)
	}
}
