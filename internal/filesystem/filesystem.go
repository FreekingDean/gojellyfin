package filesystem

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strings"
)

const Root = "/"

var (
	ErrNotFound     = errors.New("filesystem: not found")
	ErrNotDirectory = errors.New("filesystem: not a directory")
)

type File struct {
	Name  string
	Dir   bool
	Files []File
}

type Service struct {
	root File
}

func New() *Service {
	return &Service{root: defaultRoot()}
}

func defaultRoot() File {
	return File{
		Name: Root,
		Dir:  true,
		Files: []File{
			{
				Name: "media",
				Dir:  true,
				Files: []File{
					{
						Name:  "movies",
						Dir:   true,
						Files: []File{{Name: "Sintel (2010).mkv"}},
					},
					{Name: "music", Dir: true},
					{Name: "shows", Dir: true},
				},
			},
		},
	}
}

func (s *Service) Drives(ctx context.Context) ([]File, error) {
	return []File{{Name: s.root.Name, Dir: true}}, nil
}

func (s *Service) Contents(ctx context.Context, path string) ([]File, error) {
	file, err := s.Stat(ctx, path)
	if err != nil {
		return nil, err
	}
	if !file.Dir {
		return nil, ErrNotDirectory
	}

	return file.Files, nil
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

func (s *Service) List(ctx context.Context, path string) ([]File, error) {
	entries, err := os.ReadDir(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to list %q: %w", path, err)
	}

	files := make([]File, 0, len(entries))
	for _, entry := range entries {
		files = append(files, File{Name: entry.Name(), Dir: entry.IsDir()})
	}

	return files, nil
}

func (s *Service) Stat(ctx context.Context, path string) (File, error) {
	file := s.root
	for _, name := range strings.Split(strings.Trim(path, Root), Root) {
		if name == "" {
			continue
		}

		child, found := childNamed(file, name)
		if !found {
			return File{}, ErrNotFound
		}
		file = child
	}

	return file, nil
}

func childNamed(file File, name string) (File, bool) {
	for _, child := range file.Files {
		if strings.EqualFold(child.Name, name) {
			return child, true
		}
	}

	return File{}, false
}
