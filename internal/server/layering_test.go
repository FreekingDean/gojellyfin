package server

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

func TestDomainsImportNoAPIOrMiddleware(t *testing.T) {
	transport := []string{"internal/server/api", "internal/http/middleware"}
	notDomains := map[string]bool{"server": true, "http": true}

	walk := func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if notDomains[entry.Name()] {
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
			for _, layer := range transport {
				if strings.Contains(imported.Path.Value, layer) {
					t.Errorf("%s imports %s: a domain may not see the transport it is served through", path, imported.Path.Value)
				}
			}
		}

		return nil
	}

	for _, root := range []string{"cmd", "internal"} {
		if err := filepath.WalkDir(filepath.Join("..", "..", root), walk); err != nil {
			t.Fatalf("failed to walk %s: %v", root, err)
		}
	}
}
