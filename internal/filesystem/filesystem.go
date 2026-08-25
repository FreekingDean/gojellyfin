package filesystem

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

const Root = "/"

var (
	ErrNotFound     = errors.New("filesystem: not found")
	ErrNotDirectory = errors.New("filesystem: not a directory")
	ErrNotSupported = errors.New("filesystem: not supported")
)

type Service struct{}

type File struct {
	Name string
	Dir  bool
}

func New() *Service {
	s := &Service{}
	return s
}

func (s *Service) Drives(ctx context.Context) ([]File, error) {
	return []File{{Name: "/", Dir: true}}, nil
}

func (s *Service) List(ctx context.Context, name string) ([]File, error) {
	path, err := resolve(name)
	if err != nil {
		return nil, err
	}

	osFiles, err := os.ReadDir(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, ErrNotFound
	}
	if errors.Is(err, syscall.ENOTDIR) {
		return nil, ErrNotDirectory
	}
	if err != nil {
		return nil, fmt.Errorf("failed to list %q: %w", path, err)
	}

	files := make([]File, 0, len(osFiles))
	for _, file := range osFiles {
		if strings.HasPrefix(file.Name(), ".") {
			continue
		}
		files = append(files, File{Name: file.Name(), Dir: file.IsDir()})
	}

	return files, nil
}

func (s *Service) Open(ctx context.Context, name string) (io.ReadCloser, int64, error) {
	path, err := resolve(name)
	if err != nil {
		return nil, 0, err
	}

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
	return fmt.Errorf("failed to remove %q: %w", path, ErrNotSupported)
}

func (s *Service) Stat(ctx context.Context, name string) (File, error) {
	path, err := resolve(name)
	if err != nil {
		return File{}, err
	}

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

func resolve(name string) (string, error) {
	if name == "" {
		return "", ErrNotFound
	}

	path := filepath.Clean(name)
	if !filepath.IsAbs(path) || strings.Contains(path, "..") {
		return "", ErrNotFound
	}

	return path, nil
}
