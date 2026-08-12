package filesystem

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"strings"
)

const Root = "/"

var (
	ErrNotFound     = errors.New("filesystem: not found")
	ErrNotDirectory = errors.New("filesystem: not a directory")
)

type Service struct{}

type File struct {
	Name string
	Dir  bool
	file *os.File
}

func New() *Service {
	s := &Service{}
	return s
}

func (s *Service) Drives(ctx context.Context) ([]File, error) {
	return []File{{Name: "/", Dir: true}}, nil
}

func (s *Service) List(ctx context.Context, path string) ([]File, error) {
	osFiles, err := os.ReadDir(path)
	if err != nil {
		log.Fatal(err)
	}
	files := make([]File, len(osFiles))
	for i, file := range osFiles {
		if strings.HasPrefix(file.Name(), ".") {
			continue
		}
		files[i] = File{Name: file.Name(), Dir: file.IsDir()}
	}

	return files, nil
}

// The browse tree above describes what an administrator may pick from; the
// bytes still live on the host until there is somewhere else to keep them.
func (s *Service) Open(ctx context.Context, path string) (io.ReadCloser, int64, error) {
	file, err := os.Open(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, 0, ErrNotFound
	}
	if err != nil {
		return nil, 0, fmt.Errorf("failed to open %q: %w", path, err)
	}

	info, err := file.Stat()
	if err != nil {
		_ = file.Close()

		return nil, 0, fmt.Errorf("failed to stat %q: %w", path, err)
	}
	if info.IsDir() {
		_ = file.Close()

		return nil, 0, ErrNotFound
	}

	return file, info.Size(), nil
}

func (s *Service) RemoveAll(ctx context.Context, path string) error {
	return fmt.Errorf("filesystem: remove all %q: not implemented", path)
}

func (s *Service) Stat(ctx context.Context, path string) (File, error) {
	osFile, err := os.Stat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return File{}, ErrNotFound
	}
	if err != nil {
		return File{}, fmt.Errorf("failed to stat %q: %w", path, err)
	}

	return File{
		Name: osFile.Name(),
		Dir:  osFile.IsDir(),
	}, nil
}
