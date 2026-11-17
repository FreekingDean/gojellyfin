package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

func TestTheDatabaseStampsWhatItWrites(t *testing.T) {
	fset := token.NewFileSet()

	walk := func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return err
		}

		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return err
		}
		if ast.IsGenerated(file) {
			return nil
		}

		ast.Inspect(file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok || len(call.Args) != 1 {
				return true
			}
			setter, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}

			name := setter.Sel.Name
			switch {
			case strings.HasPrefix(name, "Set") && strings.HasSuffix(name, "At") && calls(call.Args[0], "time", "Now"):
				t.Errorf("%s: %s(time.Now()): the database owns the time a row is written; use Modify(store.StampNow(...))", fset.Position(call.Pos()), name)
			case name == "SetID" && calls(call.Args[0], "uuid", "New"):
				t.Errorf("%s: SetID(uuid.New()): the database owns a row's id; leave it to gen_random_uuid()", fset.Position(call.Pos()))
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

func calls(expr ast.Expr, pkg, function string) bool {
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return false
	}
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	ident, ok := selector.X.(*ast.Ident)

	return ok && ident.Name == pkg && selector.Sel.Name == function
}
