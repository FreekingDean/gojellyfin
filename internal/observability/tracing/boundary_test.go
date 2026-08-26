package tracing

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

func TestOtelStaysBehindTracing(t *testing.T) {
	elsewhere := map[string]bool{"tracing": true}

	walk := func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if elsewhere[entry.Name()] {
				return fs.SkipDir
			}

			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}

		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, imported := range file.Imports {
			if strings.Contains(imported.Path.Value, "go.opentelemetry.io") {
				t.Errorf("%s imports %s: otel belongs behind internal/observability/tracing", path, imported.Path.Value)
			}
		}

		return nil
	}

	for _, root := range []string{"cmd", "internal"} {
		if err := filepath.WalkDir(filepath.Join("..", "..", "..", root), walk); err != nil {
			t.Fatalf("failed to walk %s: %v", root, err)
		}
	}
}
