package userviews

import (
	"context"
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
	"github.com/FreekingDean/gojellyfin/internal/store"
	librarymodal "github.com/FreekingDean/gojellyfin/internal/store/library"
)

type fixture struct {
	server *Server
	client *store.Client
	prefix string
}

func newFixture(t *testing.T) *fixture {
	t.Helper()

	connection, err := store.NewStore(env.MustLoad())
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
		if _, err := client.Library.Delete().Where(librarymodal.NameHasPrefix(prefix)).Exec(ctx); err != nil {
			t.Errorf("failed to delete the libraries: %v", err)
		}
		if err := connection.Stop(); err != nil {
			t.Errorf("failed to close the database: %v", err)
		}
	})

	return &fixture{server: New(libraries.New(client)), client: client, prefix: prefix}
}

func (f *fixture) add(t *testing.T, collectionType librarymodal.CollectionType, name string) uuid.UUID {
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

func TestGetUserViews(t *testing.T) {
	fixture := newFixture(t)

	id := fixture.add(t, librarymodal.CollectionTypeMovies, "Feature Films")

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
		Id:                &id,
		ServerId:          apiutil.Ptr(config.ServerID),
		Name:              apiutil.Ptr(fixture.prefix + "Feature Films"),
		SortName:          apiutil.Ptr(strings.ToLower(fixture.prefix + "Feature Films")),
		Type:              apiutil.Ptr(api.BaseItemKindCollectionFolder),
		CollectionType:    apiutil.Ptr(api.CollectionType(librarymodal.CollectionTypeMovies)),
		IsFolder:          apiutil.Ptr(true),
		LocationType:      apiutil.Ptr(api.FileSystem),
		ImageTags:         &map[string]*string{},
		BackdropImageTags: &[]string{},
	}
	if !reflect.DeepEqual(view, want) {
		t.Errorf("view = %+v, want %+v", view, want)
	}
}

func TestGetGroupingOptions(t *testing.T) {
	fixture := newFixture(t)

	movies := fixture.add(t, librarymodal.CollectionTypeMovies, "Movies")
	shows := fixture.add(t, librarymodal.CollectionTypeTvshows, "Shows")
	mixed := fixture.add(t, librarymodal.CollectionTypeMixed, "Mixed")
	fixture.add(t, librarymodal.CollectionTypeMusic, "Music")
	fixture.add(t, librarymodal.CollectionTypeBooks, "Books")

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
