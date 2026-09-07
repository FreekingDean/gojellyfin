package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"testing"

	"go.uber.org/fx"
)

func TestServerModules(t *testing.T) {
	if err := fx.ValidateApp(serverModules); err != nil {
		t.Fatalf("server modules do not compose: %v", err)
	}
}

func TestWorkerModules(t *testing.T) {
	if err := fx.ValidateApp(workerModules); err != nil {
		t.Fatalf("worker modules do not compose: %v", err)
	}
}

func TestCommandsComposeModulesRatherThanConstructors(t *testing.T) {
	modular := packagesDeclaringAModule(t)

	for command, list := range map[string]string{
		"server.go": "serverModules",
		"worker.go": "workerModules",
	} {
		for _, provided := range constructorsListedIn(t, command, list) {
			if modular[provided] {
				t.Errorf(
					"%s lists %s.New, so %s.Module's fx.Invoke never runs and anything it registers is missing",
					command, provided, provided,
				)
			}
		}
	}
}

func packagesDeclaringAModule(t *testing.T) map[string]bool {
	t.Helper()

	found := map[string]bool{}
	err := filepath.WalkDir("../../internal", func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() || filepath.Base(path) != "fx.go" {
			return err
		}

		parsed, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			return err
		}
		for _, declared := range parsed.Decls {
			values, ok := declared.(*ast.GenDecl)
			if !ok || values.Tok != token.VAR {
				continue
			}
			for _, spec := range values.Specs {
				for _, name := range spec.(*ast.ValueSpec).Names {
					if name.Name == "Module" {
						found[parsed.Name.Name] = true
					}
				}
			}
		}

		return nil
	})
	if err != nil {
		t.Fatalf("failed to walk internal: %v", err)
	}

	return found
}

func constructorsListedIn(t *testing.T, file, list string) []string {
	t.Helper()

	parsed, err := parser.ParseFile(token.NewFileSet(), file, nil, 0)
	if err != nil {
		t.Fatalf("failed to parse %s: %v", file, err)
	}

	composed := modules(t, parsed, list)

	packages := make([]string, 0)
	ast.Inspect(composed, func(node ast.Node) bool {
		selector, ok := node.(*ast.SelectorExpr)
		if !ok || selector.Sel.Name != "New" {
			return true
		}
		if named, ok := selector.X.(*ast.Ident); ok {
			packages = append(packages, named.Name)
		}

		return true
	})

	return packages
}

func modules(t *testing.T, parsed *ast.File, name string) ast.Node {
	t.Helper()

	for _, declared := range parsed.Decls {
		values, ok := declared.(*ast.GenDecl)
		if !ok || values.Tok != token.VAR {
			continue
		}
		for _, spec := range values.Specs {
			value := spec.(*ast.ValueSpec)
			for i, named := range value.Names {
				if named.Name == name {
					return value.Values[i]
				}
			}
		}
	}

	t.Fatalf("%s declares no %s", parsed.Name.Name, name)

	return nil
}
