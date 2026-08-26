package env

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnvIsTheOnlyReaderOfTheEnvironment(t *testing.T) {
	elsewhere := map[string]bool{"env": true}
	reads := map[string]bool{"Getenv": true, "LookupEnv": true, "Environ": true}

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

		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			return err
		}
		for _, imported := range file.Imports {
			if strings.Contains(imported.Path.Value, "spf13/viper") {
				t.Errorf("%s imports %s: the environment is read by internal/env alone", path, imported.Path.Value)
			}
		}

		ast.Inspect(file, func(node ast.Node) bool {
			selector, ok := node.(*ast.SelectorExpr)
			if !ok || !reads[selector.Sel.Name] {
				return true
			}
			if ident, ok := selector.X.(*ast.Ident); ok && ident.Name == "os" {
				t.Errorf("%s calls os.%s: a knob is a field on env.Config, not a variable read where it is used", path, selector.Sel.Name)
			}

			return true
		})

		return nil
	}

	for _, root := range []string{"cmd", "internal"} {
		if err := filepath.WalkDir(filepath.Join("..", "..", root), walk); err != nil {
			t.Fatalf("failed to walk %s: %v", root, err)
		}
	}
}
