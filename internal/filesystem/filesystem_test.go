package filesystem

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func testService() *Service {
	return &Service{root: File{
		Name: Root,
		Dir:  true,
		Files: []File{
			{Name: "media", Dir: true, Files: []File{
				{Name: "movies", Dir: true},
				{Name: "readme.txt"},
			}},
		},
	}}
}

func TestContents(t *testing.T) {
	for _, tc := range []struct {
		path      string
		wantNames []string
	}{
		{path: "/", wantNames: []string{"media"}},
		{path: "", wantNames: []string{"media"}},
		{path: "/media", wantNames: []string{"movies", "readme.txt"}},
		{path: "/media/", wantNames: []string{"movies", "readme.txt"}},
		{path: "/MEDIA", wantNames: []string{"movies", "readme.txt"}},
		{path: "/media/movies", wantNames: nil},
	} {
		files, err := testService().Contents(context.Background(), tc.path)
		if err != nil {
			t.Fatalf("%q: %v", tc.path, err)
		}
		if len(files) != len(tc.wantNames) {
			t.Fatalf("%q: got %d files, want %d", tc.path, len(files), len(tc.wantNames))
		}
		for i, want := range tc.wantNames {
			if files[i].Name != want {
				t.Errorf("%q: file %d is %q, want %q", tc.path, i, files[i].Name, want)
			}
		}
	}
}

func TestContentsErrors(t *testing.T) {
	if _, err := testService().Contents(context.Background(), "/nope"); !errors.Is(err, ErrNotFound) {
		t.Errorf("got %v, want ErrNotFound", err)
	}
	if _, err := testService().Contents(context.Background(), "/media/readme.txt"); !errors.Is(err, ErrNotDirectory) {
		t.Errorf("got %v, want ErrNotDirectory", err)
	}
}

func TestStat(t *testing.T) {
	file, err := testService().Stat(context.Background(), "/media/readme.txt")
	if err != nil {
		t.Fatal(err)
	}
	if file.Name != "readme.txt" || file.Dir {
		t.Errorf("got %+v, want a file named readme.txt", file)
	}
}

func TestOpen(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "poster.jpg")
	if err := os.WriteFile(path, []byte("poster-bytes"), 0o600); err != nil {
		t.Fatalf("failed to write the file: %v", err)
	}

	body, size, err := New().Open(context.Background(), path)
	if err != nil {
		t.Fatalf("failed to open the file: %v", err)
	}
	defer func() { _ = body.Close() }()

	content, err := io.ReadAll(body)
	if err != nil {
		t.Fatalf("failed to read the file: %v", err)
	}
	if string(content) != "poster-bytes" {
		t.Errorf("content = %q, want poster-bytes", content)
	}
	if size != int64(len(content)) {
		t.Errorf("size = %d, want %d", size, len(content))
	}
}

func TestOpenErrors(t *testing.T) {
	directory := t.TempDir()

	if _, _, err := New().Open(context.Background(), filepath.Join(directory, "nope.jpg")); !errors.Is(err, ErrNotFound) {
		t.Errorf("got %v, want ErrNotFound", err)
	}
	if _, _, err := New().Open(context.Background(), directory); !errors.Is(err, ErrNotFound) {
		t.Errorf("got %v, want ErrNotFound", err)
	}
}

func TestList(t *testing.T) {
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "Movie.en.srt"), []byte("1"), 0o600); err != nil {
		t.Fatalf("failed to write the file: %v", err)
	}
	if err := os.Mkdir(filepath.Join(directory, "Extras"), 0o700); err != nil {
		t.Fatalf("failed to create the directory: %v", err)
	}

	files, err := New().List(context.Background(), directory)
	if err != nil {
		t.Fatalf("failed to list the directory: %v", err)
	}

	if len(files) != 2 {
		t.Fatalf("files = %+v, want two entries", files)
	}
	if files[0].Name != "Extras" || !files[0].Dir {
		t.Errorf("first = %+v, want the Extras directory", files[0])
	}
	if files[1].Name != "Movie.en.srt" || files[1].Dir {
		t.Errorf("second = %+v, want the subtitle file", files[1])
	}
}

func TestListErrors(t *testing.T) {
	if _, err := New().List(context.Background(), filepath.Join(t.TempDir(), "nope")); !errors.Is(err, ErrNotFound) {
		t.Errorf("got %v, want ErrNotFound", err)
	}
}

func TestDrives(t *testing.T) {
	drives, err := New().Drives(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(drives) != 1 || drives[0].Name != Root || !drives[0].Dir {
		t.Errorf("got %+v, want a single directory named %q", drives, Root)
	}
}
