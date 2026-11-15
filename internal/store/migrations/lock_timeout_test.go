package migrations

import (
	"io/fs"
	"strings"
	"testing"
)

const firstBoundedMigration = "20261114235500"

func TestMigrationsBoundTheirLocks(t *testing.T) {
	names, err := fs.Glob(FS, "*.sql")
	if err != nil {
		t.Fatal(err)
	}

	for _, name := range names {
		if name < firstBoundedMigration {
			continue
		}

		body, err := fs.ReadFile(FS, name)
		if err != nil {
			t.Fatal(err)
		}

		if !strings.Contains(string(body), "SET LOCAL lock_timeout") {
			t.Errorf("%s does not SET LOCAL lock_timeout, so a lock held elsewhere stalls the deploy rather than failing it", name)
		}
	}
}
