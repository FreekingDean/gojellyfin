package server

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var exportedTranslation = map[string]string{
	"configuration.ServerConfiguration":   "read helper for system and branding, not translation",
	"configuration.BrandingConfiguration": "read helper for branding, not translation",
}

func TestTagPackagesKeepTranslationUnexported(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("failed to read the tag packages: %v", err)
	}

	seen := map[string]bool{}
	for _, entry := range entries {
		if !entry.IsDir() || entry.Name() == "api" || entry.Name() == "dto" {
			continue
		}

		for _, name := range translators(t, entry.Name()) {
			qualified := entry.Name() + "." + name
			seen[qualified] = true

			if reason, ok := exportedTranslation[qualified]; !ok {
				t.Errorf("%s translates api types and is exported; move it to internal/server/dto or unexport it", qualified)
			} else if reason == "" {
				t.Errorf("%s is listed as an exception without a reason", qualified)
			}
		}
	}

	for qualified := range exportedTranslation {
		if !seen[qualified] {
			t.Errorf("%s is listed as an exception but no longer exists", qualified)
		}
	}
}

func translators(t *testing.T, dir string) []string {
	t.Helper()

	paths, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil {
		t.Fatalf("failed to list %s: %v", dir, err)
	}

	names := []string{}
	for _, path := range paths {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}

		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			t.Fatalf("failed to parse %s: %v", path, err)
		}

		api := apiImport(file)
		if api == "" {
			continue
		}

		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv != nil || !fn.Name.IsExported() || !mentions(fn.Type, api) {
				continue
			}

			names = append(names, fn.Name.Name)
		}
	}

	return names
}

func apiImport(file *ast.File) string {
	for _, spec := range file.Imports {
		if spec.Path.Value != `"github.com/FreekingDean/gojellyfin/internal/server/api"` {
			continue
		}
		if spec.Name != nil {
			return spec.Name.Name
		}

		return "api"
	}

	return ""
}

func mentions(signature *ast.FuncType, api string) bool {
	found := false
	ast.Inspect(signature, func(node ast.Node) bool {
		selector, ok := node.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if ident, ok := selector.X.(*ast.Ident); ok && ident.Name == api {
			found = true
		}

		return !found
	})

	return found
}
