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

var directives = []string{"//go:", "//nolint", "//lint:ignore", "// +build"}

func TestNoComments(t *testing.T) {
	fset := token.NewFileSet()

	walk := func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() || !strings.HasSuffix(path, ".go") {
			return err
		}

		file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			return err
		}
		if ast.IsGenerated(file) {
			return nil
		}

		for _, group := range file.Comments {
			for _, comment := range group.List {
				if directive(comment.Text) {
					continue
				}

				t.Errorf("%s: %s\ncode carries no comments; simplify it, or write the constraint down in CLAUDE.md", fset.Position(comment.Slash), comment.Text)
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

func directive(text string) bool {
	for _, prefix := range directives {
		if strings.HasPrefix(text, prefix) {
			return true
		}
	}

	return false
}
