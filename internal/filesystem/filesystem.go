package filesystem

import (
	"context"
	"errors"
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
