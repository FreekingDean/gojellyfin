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
	librarymodal "github.com/FreekingDean/gojellyfin/internal/store/library"
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
		if _, err := client.Library.Delete().Where(librarymodal.Name(name)).Exec(ctx); err != nil {
			t.Errorf("failed to delete the library: %v", err)
		}
		if err := connection.Stop(); err != nil {
			t.Errorf("failed to close the database: %v", err)
		}
	})

	return New(libraries.New(client)), name
}

func pathInfos(paths ...string) *[]api.MediaPathInfo {
	infos := make([]api.MediaPathInfo, 0, len(paths))
	for _, path := range paths {
		infos = append(infos, api.MediaPathInfo{Path: apiutil.Ptr(path)})
	}

	return &infos
}

func addVirtualFolder(t *testing.T, server *Server, name string, params *[]string, infos *[]api.MediaPathInfo) {
	t.Helper()

	collectionType := api.CollectionTypeOptions(librarymodal.CollectionTypeMovies)
	body := api.AddVirtualFolderJSONRequestBody{
		LibraryOptions: &api.LibraryOptions{PathInfos: infos},
	}

	response, err := server.AddVirtualFolder(context.Background(), api.AddVirtualFolderRequestObject{
		Params: api.AddVirtualFolderParams{
			Name:           apiutil.Ptr(name),
			CollectionType: &collectionType,
			Paths:          params,
		},
		JSONBody: &body,
	})
	if err != nil {
		t.Fatalf("failed to add the virtual folder: %v", err)
	}
	if _, ok := response.(api.AddVirtualFolder204Response); !ok {
		t.Fatalf("response = %T, want 204", response)
	}
}

func virtualFolder(t *testing.T, server *Server, name string) api.VirtualFolderInfo {
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
		if apiutil.Deref(folder.Name) == name {
			return folder
		}
	}

	t.Fatalf("library %q was not returned", name)

	return api.VirtualFolderInfo{}
}

func locations(t *testing.T, server *Server, name string) []string {
	t.Helper()

	folder := virtualFolder(t, server, name)
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

func TestServer_AddVirtualFolder(t *testing.T) {
	tests := []struct {
		name  string
		paths *[]string
		infos *[]api.MediaPathInfo
		want  []string
	}{
		{
			name:  "stores the path infos from the body",
			infos: pathInfos("/media/movies"),
			want:  []string{"/media/movies"},
		},
		{
			name:  "stores the paths from the query",
			paths: apiutil.Ptr([]string{"/media/movies"}),
			want:  []string{"/media/movies"},
		},
		{
			name:  "unions both spellings",
			paths: apiutil.Ptr([]string{"/media/movies", "/media/shared"}),
			infos: pathInfos("/media/shared", "/media/extra", ""),
			want:  []string{"/media/movies", "/media/shared", "/media/extra"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server, name := newServer(t)

			addVirtualFolder(t, server, name, test.paths, test.infos)

			if found := locations(t, server, name); !slices.Equal(found, test.want) {
				t.Errorf("Locations = %v, want %v", found, test.want)
			}
		})
	}
}

func updateLibraryOptions(t *testing.T, server *Server, name string, options *api.LibraryOptions) {
	t.Helper()

	id, err := uuid.Parse(apiutil.Deref(virtualFolder(t, server, name).ItemId))
	if err != nil {
		t.Fatalf("failed to read the library id: %v", err)
	}

	response, err := server.UpdateLibraryOptions(context.Background(), api.UpdateLibraryOptionsRequestObject{
		JSONBody: &api.UpdateLibraryOptionsJSONRequestBody{
			Id:             &id,
			LibraryOptions: options,
		},
	})
	if err != nil {
		t.Fatalf("failed to update the library options: %v", err)
	}
	if _, ok := response.(api.UpdateLibraryOptions204Response); !ok {
		t.Fatalf("response = %T, want 204", response)
	}
}

func TestServer_UpdateLibraryOptions(t *testing.T) {
	tests := []struct {
		name    string
		options *api.LibraryOptions
		want    []string
	}{
		{
			name:    "gains a folder the dashboard added",
			options: &api.LibraryOptions{PathInfos: pathInfos("/media/movies", "/media/second")},
			want:    []string{"/media/movies", "/media/second"},
		},
		{
			name:    "leaves the folders alone when the body carries none",
			options: &api.LibraryOptions{},
			want:    []string{"/media/movies"},
		},
		{
			name:    "leaves the folders alone when the body carries an empty list",
			options: &api.LibraryOptions{PathInfos: pathInfos()},
			want:    []string{"/media/movies"},
		},
		{
			name:    "leaves the folders alone when the body names nothing but blanks",
			options: &api.LibraryOptions{PathInfos: pathInfos("")},
			want:    []string{"/media/movies"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server, name := newServer(t)
			addVirtualFolder(t, server, name, nil, pathInfos("/media/movies"))

			updateLibraryOptions(t, server, name, test.options)

			if found := locations(t, server, name); !slices.Equal(found, test.want) {
				t.Errorf("Locations = %v, want %v", found, test.want)
			}
		})
	}
}

func TestServer_RemoveMediaPath(t *testing.T) {
	server, name := newServer(t)
	addVirtualFolder(t, server, name, nil, pathInfos("/media/movies", "/media/second"))

	response, err := server.RemoveMediaPath(context.Background(), api.RemoveMediaPathRequestObject{
		Params: api.RemoveMediaPathParams{
			Name: apiutil.Ptr(name),
			Path: apiutil.Ptr("/media/second"),
		},
	})
	if err != nil {
		t.Fatalf("failed to remove the media path: %v", err)
	}
	if _, ok := response.(api.RemoveMediaPath204Response); !ok {
		t.Fatalf("response = %T, want 204", response)
	}

	want := []string{"/media/movies"}
	if found := locations(t, server, name); !slices.Equal(found, want) {
		t.Errorf("Locations = %v, want %v", found, want)
	}
}
