package environment

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
)

func TestGetDirectoryContents(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "Movies"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, ".hidden"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "note.txt"), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name      string
		files     bool
		dirs      bool
		wantNames []string
	}{
		{name: "directories only", dirs: true, wantNames: []string{"Movies"}},
		{name: "files only", files: true, wantNames: []string{"note.txt"}},
		{name: "both", files: true, dirs: true, wantNames: []string{"Movies", "note.txt"}},
		{name: "neither", wantNames: nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			response, err := New().GetDirectoryContents(context.Background(), api.GetDirectoryContentsRequestObject{
				Params: api.GetDirectoryContentsParams{
					Path:               dir,
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
					t.Errorf("entry %d: got %q, want %q", i, got, want)
				}
			}
		})
	}
}

func TestGetDirectoryContentsMissingPath(t *testing.T) {
	response, err := New().GetDirectoryContents(context.Background(), api.GetDirectoryContentsRequestObject{
		Params: api.GetDirectoryContentsParams{Path: filepath.Join(t.TempDir(), "nope")},
	})
	if err != nil {
		t.Fatal(err)
	}
	if contents := response.(api.GetDirectoryContents200JSONResponse); len(contents) != 0 {
		t.Fatalf("got %d entries, want 0", len(contents))
	}
}

func TestGetParentPath(t *testing.T) {
	for path, want := range map[string]string{
		"/media/movies": "/media",
		"/media/":       "/",
		"/media":        "/",
		"/":             "",
	} {
		response, err := New().GetParentPath(context.Background(), api.GetParentPathRequestObject{
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
