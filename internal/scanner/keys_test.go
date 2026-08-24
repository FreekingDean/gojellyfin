package scanner

import "testing"

func TestKeysSurviveAMove(t *testing.T) {
	tests := []struct {
		name string
		got  string
		want string
	}{
		{"movie with a year", movieKey("The Matrix", ptr(int32(1999))), "movie:the-matrix:1999"},
		{"movie without a year", movieKey("The Matrix", nil), "movie:the-matrix"},
		{"series", seriesKey(titleSlug("The Wire", nil)), "series:the-wire"},
		{"series with a year", seriesKey(titleSlug("The Wire", ptr(int32(2002)))), "series:the-wire:2002"},
		{"season", seasonKey("the-wire", ptr(int32(1))), "season:the-wire:1"},
		{"specials", seasonKey("the-wire", ptr(int32(0))), "season:the-wire:0"},
		{"episode", episodeKey("the-wire", ptr(int32(1)), ptr(int32(3)), "The Buys"), "episode:the-wire:1:3"},
		{"unnumbered episode", episodeKey("the-wire", nil, nil, "Behind the Scenes"), "episode:the-wire:behind-the-scenes"},
		{"punctuation", movieKey("Amélie: A Film!", ptr(int32(2001))), "movie:amélie-a-film:2001"},
		{"separators collapse", movieKey("W.A.L.L - E", nil), "movie:w-a-l-l-e"},
	}

	for _, test := range tests {
		if test.got != test.want {
			t.Errorf("%s = %q, want %q", test.name, test.got, test.want)
		}
	}
}

func TestKeysIgnoreTheLocation(t *testing.T) {
	first, year := parseTitle("The Matrix (1999)")
	second, otherYear := parseTitle("The.Matrix.1999")

	if movieKey(first, year) != movieKey(second, otherYear) {
		t.Errorf("%q and %q derive different keys", movieKey(first, year), movieKey(second, otherYear))
	}
}
