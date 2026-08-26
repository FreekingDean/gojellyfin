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

func TestMusicKeysSeparateTracksThatShareATitle(t *testing.T) {
	album := albumSlug("queen", "A Night at the Opera", ptr(int32(1975)))

	tests := []struct {
		name string
		got  string
		want string
	}{
		{"artist", musicArtistKey(titleSlug("Queen", nil)), "musicartist:queen"},
		{"album", musicAlbumKey(album), "musicalbum:queen:a-night-at-the-opera:1975"},
		{"track", audioKey(album, nil, ptr(int32(3)), "Love of My Life"), "audio:queen:a-night-at-the-opera:1975:1:3"},
		{"track on a second disc", audioKey(album, ptr(int32(2)), ptr(int32(3)), "Love of My Life"), "audio:queen:a-night-at-the-opera:1975:2:3"},
		{"unnumbered track", audioKey(album, nil, nil, "Love of My Life"), "audio:queen:a-night-at-the-opera:1975:love-of-my-life"},
		{"track with no album", audioKey("queen", nil, nil, "Loose Track"), "audio:queen:loose-track"},
		{"track with nothing above it", audioKey("", nil, nil, "Root Level"), "audio:root-level"},
	}

	for _, test := range tests {
		if test.got != test.want {
			t.Errorf("%s = %q, want %q", test.name, test.got, test.want)
		}
	}
}

// Two recordings of one song are told apart by their position, and the same
// song on a compilation is told apart by its album, so neither folds into the
// other the way two encodes of one track are meant to.
func TestMusicKeysDistinguishVersionsAndCompilations(t *testing.T) {
	album := albumSlug("queen", "A Night at the Opera", nil)
	compilation := albumSlug("various-artists", "Greatest Hits", nil)

	original := audioKey(album, nil, ptr(int32(5)), "Love of My Life")
	reprise := audioKey(album, nil, ptr(int32(12)), "Love of My Life")
	if original == reprise {
		t.Errorf("two versions on one album share the key %q", original)
	}

	if elsewhere := audioKey(compilation, nil, ptr(int32(5)), "Love of My Life"); elsewhere == original {
		t.Errorf("the same song on a compilation shares the key %q", original)
	}

	flac, mp3 := audioKey(album, nil, ptr(int32(5)), "Love of My Life"), audioKey(album, nil, ptr(int32(5)), "Love of My Life")
	if flac != mp3 {
		t.Errorf("two encodes of one track derive different keys, %q and %q", flac, mp3)
	}
}
