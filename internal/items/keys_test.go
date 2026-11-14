package items

import "testing"

func TestTmdbID(t *testing.T) {
	cases := []struct {
		key     string
		want    int
		matched bool
	}{
		{MovieKey(603), 603, true},
		{SeriesKey(1396), 1396, true},
		{SeasonKey(1396, 0), 1396, true},
		{EpisodeKey(1396, 1, 3), 1396, true},
		{"movie:tmdb:0", 0, false},
		{"movie:tmdb:-603", 0, false},
		{"movie:tmdb:abc", 0, false},
		{"movie:tvdb:603", 0, false},
		{"movie:tmdb", 0, false},
		{"movie:the-matrix-1999", 0, false},
		{"", 0, false},
	}

	for _, tc := range cases {
		t.Run(tc.key, func(t *testing.T) {
			got, matched := TmdbID(tc.key)
			if got != tc.want || matched != tc.matched {
				t.Errorf("TmdbID(%q) = %d, %v, want %d, %v", tc.key, got, matched, tc.want, tc.matched)
			}
		})
	}
}
