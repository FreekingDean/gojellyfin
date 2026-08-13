package filesystem

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestStat(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "readme.txt")
	if err := os.WriteFile(path, []byte("notes"), 0o600); err != nil {
		t.Fatalf("failed to write the file: %v", err)
	}

	file, err := New().Stat(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if file.Name != "readme.txt" || file.Dir {
		t.Errorf("got %+v, want a file named readme.txt", file)
	}

	directoryFile, err := New().Stat(context.Background(), directory)
	if err != nil {
		t.Fatal(err)
	}
	if !directoryFile.Dir {
		t.Errorf("got %+v, want a directory", directoryFile)
	}
}

func TestStatErrors(t *testing.T) {
	if _, err := New().Stat(context.Background(), filepath.Join(t.TempDir(), "nope")); !errors.Is(err, ErrNotFound) {
		t.Errorf("got %v, want ErrNotFound", err)
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

func TestListSkipsHiddenFiles(t *testing.T) {
	directory := t.TempDir()
	for _, name := range []string{".DS_Store", "Movie.mkv", ".hidden"} {
		if err := os.WriteFile(filepath.Join(directory, name), []byte("1"), 0o600); err != nil {
			t.Fatalf("failed to write %q: %v", name, err)
		}
	}

	files, err := New().List(context.Background(), directory)
	if err != nil {
		t.Fatalf("failed to list the directory: %v", err)
	}

	if len(files) != 1 || files[0].Name != "Movie.mkv" {
		t.Errorf("files = %+v, want only Movie.mkv", files)
	}
}

func TestListErrors(t *testing.T) {
	directory := t.TempDir()

	if _, err := New().List(context.Background(), filepath.Join(directory, "nope")); !errors.Is(err, ErrNotFound) {
		t.Errorf("missing path: got %v, want ErrNotFound", err)
	}

	path := filepath.Join(directory, "poster.jpg")
	if err := os.WriteFile(path, []byte("1"), 0o600); err != nil {
		t.Fatalf("failed to write the file: %v", err)
	}
	if _, err := New().List(context.Background(), path); !errors.Is(err, ErrNotDirectory) {
		t.Errorf("file path: got %v, want ErrNotDirectory", err)
	}
}

func TestRemoveAllIsNotImplemented(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "keep.mkv")
	if err := os.WriteFile(path, []byte("1"), 0o600); err != nil {
		t.Fatalf("failed to write the file: %v", err)
	}

	if err := New().RemoveAll(context.Background(), path); err == nil {
		t.Fatal("RemoveAll returned no error; if it is implemented now, this test needs replacing with one that asserts what it deletes")
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("the file was touched: %v", err)
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

func TestRelativePathsAreRejected(t *testing.T) {
	for _, name := range []string{"", "relative/path", "../etc", "media/../../etc"} {
		if _, err := New().Stat(context.Background(), name); !errors.Is(err, ErrNotFound) {
			t.Errorf("Stat(%q) = %v, want ErrNotFound", name, err)
		}
		if _, err := New().List(context.Background(), name); !errors.Is(err, ErrNotFound) {
			t.Errorf("List(%q) = %v, want ErrNotFound", name, err)
		}
		if _, _, err := New().Open(context.Background(), name); !errors.Is(err, ErrNotFound) {
			t.Errorf("Open(%q) = %v, want ErrNotFound", name, err)
		}
	}
}
