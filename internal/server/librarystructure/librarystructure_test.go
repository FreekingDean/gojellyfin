package librarystructure

import (
	"context"
	"slices"
	"testing"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/env"
	"github.com/FreekingDean/gojellyfin/internal/libraries"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
	"github.com/FreekingDean/gojellyfin/internal/store"
	librarymodel "github.com/FreekingDean/gojellyfin/internal/store/library"
)

func newServer(t *testing.T) (*Server, string) {
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
	name := t.Name() + "-" + uuid.NewString()

	t.Cleanup(func() {
		ctx := context.Background()
		if _, err := client.Library.Delete().Where(librarymodel.Name(name)).Exec(ctx); err != nil {
			t.Errorf("failed to delete the library: %v", err)
		}
		if err := connection.Stop(); err != nil {
			t.Errorf("failed to close the database: %v", err)
		}
	})

	return New(libraries.New(client)), name
}

func locations(t *testing.T, server *Server, name string) []string {
	t.Helper()

	response, err := server.GetVirtualFolders(context.Background(), api.GetVirtualFoldersRequestObject{})
	if err != nil {
		t.Fatalf("failed to list the virtual folders: %v", err)
	}

	folders, ok := response.(api.GetVirtualFolders200JSONResponse)
	if !ok {
		t.Fatalf("response = %T, want 200", response)
	}

	for _, folder := range folders {
		if apiutil.Deref(folder.Name) != name {
			continue
		}

		found := apiutil.Deref(folder.Locations)
		infos := apiutil.Deref(folder.LibraryOptions.PathInfos)

		reported := make([]string, 0, len(infos))
		for _, info := range infos {
			reported = append(reported, apiutil.Deref(info.Path))
		}
		if !slices.Equal(found, reported) {
			t.Errorf("PathInfos = %v, want the same as Locations %v", reported, found)
		}

		return found
	}

	t.Fatalf("library %q was not returned", name)

	return nil
}

func TestServer_ListsLocations(t *testing.T) {
	server, name := newServer(t)
	want := []string{"/media/movies", "/media/movies-4k"}

	if _, err := server.libraries.CreateLibrary(context.Background(), name, libraries.CollectionTypeMovies, want); err != nil {
		t.Fatalf("failed to create the library: %v", err)
	}

	if found := locations(t, server, name); !slices.Equal(found, want) {
		t.Errorf("Locations = %v, want %v", found, want)
	}
}

func TestServer_RefusesReshapingTheLibraries(t *testing.T) {
	server, name := newServer(t)
	ctx := context.Background()
	if _, err := server.libraries.CreateLibrary(ctx, name, libraries.CollectionTypeMovies, nil); err != nil {
		t.Fatalf("failed to create the library: %v", err)
	}

	added, err := server.AddVirtualFolder(ctx, api.AddVirtualFolderRequestObject{
		Params: api.AddVirtualFolderParams{Name: apiutil.Ptr(name + "-added")},
	})
	if _, ok := added.(api.AddVirtualFolder403Response); !ok || err != nil {
		t.Errorf("AddVirtualFolder = %T, %v, want 403", added, err)
	}

	removed, err := server.RemoveVirtualFolder(ctx, api.RemoveVirtualFolderRequestObject{
		Params: api.RemoveVirtualFolderParams{Name: apiutil.Ptr(name)},
	})
	if _, ok := removed.(api.RemoveVirtualFolder403Response); !ok || err != nil {
		t.Errorf("RemoveVirtualFolder = %T, %v, want 403", removed, err)
	}

	pathAdded, err := server.AddMediaPath(ctx, api.AddMediaPathRequestObject{
		JSONBody: &api.AddMediaPathJSONRequestBody{Name: name, Path: apiutil.Ptr("/elsewhere")},
	})
	if _, ok := pathAdded.(api.AddMediaPath403Response); !ok || err != nil {
		t.Errorf("AddMediaPath = %T, %v, want 403", pathAdded, err)
	}

	if found := locations(t, server, name); len(found) != 0 {
		t.Errorf("Locations = %v, want none", found)
	}
}
