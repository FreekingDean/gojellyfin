package migrations

import (
	"io/fs"
	"strings"
	"testing"
)

const firstBoundedMigration = "20261114235500"

var bounds = map[string]string{
	"lock_timeout":                     "a lock held elsewhere stalls the deploy rather than failing it",
	"client_connection_check_interval": "a run whose Job was deleted keeps its locks until it finishes",
}

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

		for setting, reason := range bounds {
			if !strings.Contains(string(body), "SET LOCAL "+setting) {
				t.Errorf("%s does not SET LOCAL %s, so %s", name, setting, reason)
			}
		}
	}
}
