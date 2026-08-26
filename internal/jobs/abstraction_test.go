package jobs_test

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

func Test(t *testing.T) {
	root := filepath.Join("..", "..")

	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() || !strings.HasSuffix(path, ".go") {
			return err
		}
		if strings.Contains(path, string(filepath.Separator)+"jobs"+string(filepath.Separator)) {
			return nil
		}

		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, imported := range file.Imports {
			if strings.Contains(imported.Path.Value, "go.temporal.io") {
				t.Errorf("%s imports %s: the engine belongs behind internal/jobs", path, imported.Path.Value)
			}
		}

		return nil
	})
	if err != nil {
		t.Fatalf("failed to walk the tree: %v", err)
	}
}
