package filesystem

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/FreekingDean/gojellyfin/internal/env"
)

func service(roots ...string) *Service {
	if len(roots) == 0 {
		roots = []string{Root}
	}

	return New(env.Config{MediaDirectories: roots})
}

func TestService_Stat(t *testing.T) {
	t.Run("describes a file and a directory", func(t *testing.T) {
		directory := t.TempDir()
		path := filepath.Join(directory, "readme.txt")
		if err := os.WriteFile(path, []byte("notes"), 0o600); err != nil {
			t.Fatalf("failed to write the file: %v", err)
		}

		file, err := service().Stat(context.Background(), path)
		if err != nil {
			t.Fatal(err)
		}
		if file.Name != "readme.txt" || file.Dir {
			t.Errorf("got %+v, want a file named readme.txt", file)
		}

		directoryFile, err := service().Stat(context.Background(), directory)
		if err != nil {
			t.Fatal(err)
		}
		if !directoryFile.Dir {
			t.Errorf("got %+v, want a directory", directoryFile)
		}
	})

	t.Run("misses a path that is not there", func(t *testing.T) {
		if _, err := service().Stat(context.Background(), filepath.Join(t.TempDir(), "nope")); !errors.Is(err, ErrNotFound) {
			t.Errorf("got %v, want ErrNotFound", err)
		}
	})
}

func TestService_Open(t *testing.T) {
	t.Run("reads the file back with its size", func(t *testing.T) {
		directory := t.TempDir()
		path := filepath.Join(directory, "poster.jpg")
		if err := os.WriteFile(path, []byte("poster-bytes"), 0o600); err != nil {
			t.Fatalf("failed to write the file: %v", err)
		}

		body, size, err := service().Open(context.Background(), path)
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
	})

	t.Run("misses a missing file and a directory", func(t *testing.T) {
		directory := t.TempDir()

		if _, _, err := service().Open(context.Background(), filepath.Join(directory, "nope.jpg")); !errors.Is(err, ErrNotFound) {
			t.Errorf("got %v, want ErrNotFound", err)
		}
		if _, _, err := service().Open(context.Background(), directory); !errors.Is(err, ErrNotFound) {
			t.Errorf("got %v, want ErrNotFound", err)
		}
	})
}

func TestService_List(t *testing.T) {
	t.Run("returns directories before files", func(t *testing.T) {
		directory := t.TempDir()
		if err := os.WriteFile(filepath.Join(directory, "Movie.en.srt"), []byte("1"), 0o600); err != nil {
			t.Fatalf("failed to write the file: %v", err)
		}
		if err := os.Mkdir(filepath.Join(directory, "Extras"), 0o700); err != nil {
			t.Fatalf("failed to create the directory: %v", err)
		}

		files, err := service().List(context.Background(), directory)
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
	})

	t.Run("skips hidden files", func(t *testing.T) {
		directory := t.TempDir()
		for _, name := range []string{".DS_Store", "Movie.mkv", ".hidden"} {
			if err := os.WriteFile(filepath.Join(directory, name), []byte("1"), 0o600); err != nil {
				t.Fatalf("failed to write %q: %v", name, err)
			}
		}

		files, err := service().List(context.Background(), directory)
		if err != nil {
			t.Fatalf("failed to list the directory: %v", err)
		}

		if len(files) != 1 || files[0].Name != "Movie.mkv" {
			t.Errorf("files = %+v, want only Movie.mkv", files)
		}
	})

	t.Run("misses a missing path and refuses a file", func(t *testing.T) {
		directory := t.TempDir()

		if _, err := service().List(context.Background(), filepath.Join(directory, "nope")); !errors.Is(err, ErrNotFound) {
			t.Errorf("missing path: got %v, want ErrNotFound", err)
		}

		path := filepath.Join(directory, "poster.jpg")
		if err := os.WriteFile(path, []byte("1"), 0o600); err != nil {
			t.Fatalf("failed to write the file: %v", err)
		}
		if _, err := service().List(context.Background(), path); !errors.Is(err, ErrNotDirectory) {
			t.Errorf("file path: got %v, want ErrNotDirectory", err)
		}
	})
}

func TestService_RemoveAll(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "keep.mkv")
	if err := os.WriteFile(path, []byte("1"), 0o600); err != nil {
		t.Fatalf("failed to write the file: %v", err)
	}

	if err := service().RemoveAll(context.Background(), path); err == nil {
		t.Fatal("RemoveAll returned no error; if it is implemented now, this test needs replacing with one that asserts what it deletes")
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("the file was touched: %v", err)
	}
}

func TestService_Drives(t *testing.T) {
	drives, err := service().Drives(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(drives) != 1 || drives[0].Name != Root || !drives[0].Dir {
		t.Errorf("got %+v, want a single directory named %q", drives, Root)
	}
}

func TestResolve(t *testing.T) {
	outside := t.TempDir()
	if _, err := service(t.TempDir()).Stat(context.Background(), outside); !errors.Is(err, ErrNotFound) {
		t.Errorf("Stat(%q) = %v, want a path outside the media directories refused", outside, err)
	}

	for _, name := range []string{"", "relative/path", "../etc", "media/../../etc"} {
		if _, err := service().Stat(context.Background(), name); !errors.Is(err, ErrNotFound) {
			t.Errorf("Stat(%q) = %v, want ErrNotFound", name, err)
		}
		if _, err := service().List(context.Background(), name); !errors.Is(err, ErrNotFound) {
			t.Errorf("List(%q) = %v, want ErrNotFound", name, err)
		}
		if _, _, err := service().Open(context.Background(), name); !errors.Is(err, ErrNotFound) {
			t.Errorf("Open(%q) = %v, want ErrNotFound", name, err)
		}
	}
}
