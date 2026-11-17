package storetest

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

func TestTestsOpenTheStoreThroughStoretest(t *testing.T) {
	fset := token.NewFileSet()

	walk := func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() || !strings.HasSuffix(path, "_test.go") {
			return err
		}
		if filepath.Base(filepath.Dir(path)) == "storetest" {
			return nil
		}

		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return err
		}

		ast.Inspect(file, func(node ast.Node) bool {
			selector, ok := node.(*ast.SelectorExpr)
			if !ok || selector.Sel.Name != "NewStore" {
				return true
			}
			if pkg, ok := selector.X.(*ast.Ident); ok && pkg.Name == "store" {
				t.Errorf("%s: a test opens the store through storetest.Open or storetest.Scratch", fset.Position(selector.Pos()))
			}

			return true
		})

		return nil
	}

	for _, root := range []string{"cmd", "internal", "scripts"} {
		if err := filepath.WalkDir(filepath.Join("..", "..", root), walk); err != nil {
			t.Fatalf("failed to walk %s: %v", root, err)
		}
	}
}
