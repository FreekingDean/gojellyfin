package scanner

import (
	"os"
	"testing"
)

func TestWalk_complete(t *testing.T) {
	found := &seen{}
	if !found.complete() {
		t.Error("a walk that skipped nothing reported incomplete")
	}

	found.file("/library/readable/movie.mkv")
	if !found.complete() {
		t.Error("finding a file made the walk incomplete")
	}

	found.skip("Radarr 4K", os.ErrPermission)
	if found.complete() {
		t.Error("a skipped source left the walk reporting complete")
	}
}
