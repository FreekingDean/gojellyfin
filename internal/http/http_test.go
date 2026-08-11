package http

import (
	"slices"
	"strings"
	"testing"
)

// The mux matches in registration order, so a parameter registered before a
// literal it can match swallows it: /Users/{userId}/Items/{itemId} would take
// /Users/{userId}/Items/Resume and hand "Resume" over as an item id.
func TestLegacyPatternsPutLiteralsBeforeParameters(t *testing.T) {
	patterns := legacyPatterns()

	for _, pattern := range patterns {
		for _, other := range patterns {
			if pattern == other || !shadows(other, pattern) {
				continue
			}

			if slices.Index(patterns, other) < slices.Index(patterns, pattern) {
				t.Errorf("%q is registered before %q and swallows it", other, pattern)
			}
		}
	}
}

func TestLegacyPatternsAreStable(t *testing.T) {
	first := legacyPatterns()
	for range 20 {
		if !slices.Equal(first, legacyPatterns()) {
			t.Fatal("want a stable order, got one that varies per call")
		}
	}
}

// Whether a request matching pattern would also match candidate, which is only
// true when candidate is the same shape with a parameter where pattern has a
// literal.
func shadows(candidate, pattern string) bool {
	a := strings.Split(candidate, "/")
	b := strings.Split(pattern, "/")
	if len(a) != len(b) {
		return false
	}

	loose := false
	for i := range a {
		switch {
		case a[i] == b[i]:
		case strings.HasPrefix(a[i], "{"):
			loose = true
		default:
			return false
		}
	}

	return loose
}
