package environment

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/FreekingDean/gojellyfin/internal/filesystem"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
)

func TestGetDirectoryContents(t *testing.T) {
	media := t.TempDir()
	movies := filepath.Join(media, "movies")
	for _, name := range []string{"movies", "music", "shows"} {
		if err := os.Mkdir(filepath.Join(media, name), 0o700); err != nil {
			t.Fatalf("failed to create %q: %v", name, err)
		}
	}
	if err := os.WriteFile(filepath.Join(movies, "Sintel (2010).mkv"), []byte("1"), 0o600); err != nil {
		t.Fatalf("failed to write the media file: %v", err)
	}

	for _, tc := range []struct {
		name      string
		path      string
		files     bool
		dirs      bool
		wantNames []string
		wantPaths []string
	}{
		{
			name: "directories only", path: media, dirs: true,
			wantNames: []string{"movies", "music", "shows"},
			wantPaths: []string{filepath.Join(media, "movies"), filepath.Join(media, "music"), filepath.Join(media, "shows")},
		},
		{name: "files excluded", path: movies, dirs: true},
		{
			name: "files only", path: movies, files: true,
			wantNames: []string{"Sintel (2010).mkv"},
			wantPaths: []string{filepath.Join(movies, "Sintel (2010).mkv")},
		},
		{
			name: "both", path: movies, files: true, dirs: true,
			wantNames: []string{"Sintel (2010).mkv"},
			wantPaths: []string{filepath.Join(movies, "Sintel (2010).mkv")},
		},
		{name: "neither", path: media},
	} {
		t.Run(tc.name, func(t *testing.T) {
			response, err := testServer().GetDirectoryContents(context.Background(), api.GetDirectoryContentsRequestObject{
				Params: api.GetDirectoryContentsParams{
					Path:               tc.path,
					IncludeFiles:       apiutil.Ptr(tc.files),
					IncludeDirectories: apiutil.Ptr(tc.dirs),
				},
			})
			if err != nil {
				t.Fatal(err)
			}

			contents := response.(api.GetDirectoryContents200JSONResponse)
			if len(contents) != len(tc.wantNames) {
				t.Fatalf("got %d entries, want %d", len(contents), len(tc.wantNames))
			}
			for i, want := range tc.wantNames {
				if got := apiutil.Deref(contents[i].Name); got != want {
					t.Errorf("entry %d: got name %q, want %q", i, got, want)
				}
				if got := apiutil.Deref(contents[i].Path); got != tc.wantPaths[i] {
					t.Errorf("entry %d: got path %q, want %q", i, got, tc.wantPaths[i])
				}
			}
		})
	}
}

func TestGetDirectoryContentsMissingPath(t *testing.T) {
	_, err := testServer().GetDirectoryContents(context.Background(), api.GetDirectoryContentsRequestObject{
		Params: api.GetDirectoryContentsParams{Path: filepath.Join(t.TempDir(), "nope")},
	})
	if !errors.Is(err, filesystem.ErrNotFound) {
		t.Fatalf("got %v, want ErrNotFound", err)
	}
}

func TestGetDrives(t *testing.T) {
	response, err := testServer().GetDrives(context.Background(), api.GetDrivesRequestObject{})
	if err != nil {
		t.Fatal(err)
	}

	drives := response.(api.GetDrives200JSONResponse)
	if len(drives) != 1 {
		t.Fatalf("got %d drives, want 1", len(drives))
	}
	if got := apiutil.Deref(drives[0].Path); got != filesystem.Root {
		t.Errorf("got path %q, want %q", got, filesystem.Root)
	}
	if got := apiutil.Deref(drives[0].Type); got != api.FileSystemEntryTypeDirectory {
		t.Errorf("got type %q, want %q", got, api.FileSystemEntryTypeDirectory)
	}
}

func TestGetParentPath(t *testing.T) {
	for path, want := range map[string]string{
		"/media/movies": "/media",
		"/media/":       "/",
		"/media":        "/",
		"/":             "/",
		"":              "/",
	} {
		response, err := testServer().GetParentPath(context.Background(), api.GetParentPathRequestObject{
			Params: api.GetParentPathParams{Path: path},
		})
		if err != nil {
			t.Fatal(err)
		}
		if got := string(response.(api.GetParentPath200JSONResponse)); got != want {
			t.Errorf("%q: got %q, want %q", path, got, want)
		}
	}
}

func TestGetDefaultDirectoryBrowser(t *testing.T) {
	response, err := testServer().GetDefaultDirectoryBrowser(context.Background(), api.GetDefaultDirectoryBrowserRequestObject{})
	if err != nil {
		t.Fatal(err)
	}
	if got := apiutil.Deref(api.DefaultDirectoryBrowserInfoDto(response.(api.GetDefaultDirectoryBrowser200JSONResponse)).Path); got != filesystem.Root {
		t.Errorf("got %q, want %q", got, filesystem.Root)
	}
}

func testServer() *Server {
	return New(filesystem.New())
}
