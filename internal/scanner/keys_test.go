package scanner

import "testing"

type keyCase struct {
	name string
	got  string
	want string
}

func checkKeys(t *testing.T, cases []keyCase) {
	t.Helper()

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if test.got != test.want {
				t.Errorf("= %q, want %q", test.got, test.want)
			}
		})
	}
}

func TestMovieKey(t *testing.T) {
	checkKeys(t, []keyCase{
		{"with a year", movieKey("The Matrix", ptr(int32(1999))), "movie:the-matrix:1999"},
		{"without a year", movieKey("The Matrix", nil), "movie:the-matrix"},
		{"punctuation", movieKey("Amélie: A Film!", ptr(int32(2001))), "movie:amélie-a-film:2001"},
		{"separators collapse", movieKey("W.A.L.L - E", nil), "movie:w-a-l-l-e"},
	})

	t.Run("ignores the location the file was found at", func(t *testing.T) {
		first, year := parseTitle("The Matrix (1999)")
		second, otherYear := parseTitle("The.Matrix.1999")

		if movieKey(first, year) != movieKey(second, otherYear) {
			t.Errorf("%q and %q derive different keys", movieKey(first, year), movieKey(second, otherYear))
		}
	})
}

func TestSeriesKey(t *testing.T) {
	checkKeys(t, []keyCase{
		{"plain", seriesKey(titleSlug("The Wire", nil)), "series:the-wire"},
		{"with a year", seriesKey(titleSlug("The Wire", ptr(int32(2002)))), "series:the-wire:2002"},
	})
}

func TestSeasonKey(t *testing.T) {
	checkKeys(t, []keyCase{
		{"numbered", seasonKey("the-wire", ptr(int32(1))), "season:the-wire:1"},
		{"specials", seasonKey("the-wire", ptr(int32(0))), "season:the-wire:0"},
	})
}

func TestEpisodeKey(t *testing.T) {
	checkKeys(t, []keyCase{
		{"numbered", episodeKey("the-wire", ptr(int32(1)), ptr(int32(3)), "The Buys"), "episode:the-wire:1:3"},
		{"unnumbered", episodeKey("the-wire", nil, nil, "Behind the Scenes"), "episode:the-wire:behind-the-scenes"},
	})
}
