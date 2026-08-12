package scanner

import "testing"

func TestParseTitle(t *testing.T) {
	tests := []struct {
		in   string
		name string
		year int32
	}{
		{"Blade Runner (1982)", "Blade Runner", 1982},
		{"Blade.Runner.1982", "Blade Runner", 1982},
		{"Blade Runner [1982]", "Blade Runner", 1982},
		{"Blade Runner", "Blade Runner", 0},
		{"2001 A Space Odyssey (1968)", "2001 A Space Odyssey", 1968},
		{"1917", "1917", 0},
	}

	for _, test := range tests {
		name, year := parseTitle(test.in)
		if name != test.name {
			t.Errorf("parseTitle(%q) name = %q, want %q", test.in, name, test.name)
		}
		if test.year == 0 {
			if year != nil {
				t.Errorf("parseTitle(%q) year = %d, want nil", test.in, *year)
			}
			continue
		}
		if year == nil || *year != test.year {
			t.Errorf("parseTitle(%q) year = %v, want %d", test.in, year, test.year)
		}
	}
}

func TestParseEpisode(t *testing.T) {
	tests := []struct {
		in      string
		season  int32
		episode int32
		title   string
		ok      bool
	}{
		{"The Wire S01E03 The Buys.mkv", 1, 3, "The Buys", true},
		{"The.Wire.s01e03.mkv", 1, 3, "", true},
		{"The Wire 1x03.mkv", 1, 3, "", true},
		{"The Wire S01E03.mkv", 1, 3, "", true},
		{"Blade Runner (1982).mkv", 0, 0, "", false},
	}

	for _, test := range tests {
		season, episode, title, ok := parseEpisode(test.in)
		if ok != test.ok {
			t.Errorf("parseEpisode(%q) ok = %v, want %v", test.in, ok, test.ok)
			continue
		}
		if !ok {
			continue
		}
		if *season != test.season || *episode != test.episode {
			t.Errorf("parseEpisode(%q) = S%dE%d, want S%dE%d", test.in, *season, *episode, test.season, test.episode)
		}
		if title != test.title {
			t.Errorf("parseEpisode(%q) title = %q, want %q", test.in, title, test.title)
		}
	}
}

func TestParseSeason(t *testing.T) {
	tests := []struct {
		in     string
		number int32
		ok     bool
	}{
		{"Season 1", 1, true},
		{"season 01", 1, true},
		{"S02", 2, true},
		{"Specials", 0, true},
		{"Extras", 0, false},
	}

	for _, test := range tests {
		number, ok := parseSeason(test.in)
		if ok != test.ok {
			t.Errorf("parseSeason(%q) ok = %v, want %v", test.in, ok, test.ok)
			continue
		}
		if ok && *number != test.number {
			t.Errorf("parseSeason(%q) = %d, want %d", test.in, *number, test.number)
		}
	}
}

func TestParseSubtitle(t *testing.T) {
	tests := []struct {
		base      string
		name      string
		language  string
		title     string
		codec     string
		forced    bool
		isDefault bool
		impaired  bool
		ok        bool
	}{
		{base: "Blade Runner (1982)", name: "Blade Runner (1982).srt", codec: "srt", ok: true},
		{base: "Blade Runner (1982)", name: "Blade Runner (1982).en.srt", language: "en", codec: "srt", ok: true},
		{base: "Blade Runner (1982)", name: "Blade Runner (1982).eng.forced.srt", language: "eng", codec: "srt", forced: true, ok: true},
		{base: "Blade Runner (1982)", name: "Blade Runner (1982).en.default.forced.vtt", language: "en", codec: "vtt", forced: true, isDefault: true, ok: true},
		{base: "Blade Runner (1982)", name: "Blade Runner (1982).en.sdh.ass", language: "en", codec: "ass", impaired: true, ok: true},
		{base: "Blade Runner (1982)", name: "Blade Runner (1982).Commentary.en.srt", language: "en", title: "commentary", codec: "srt", ok: true},
		{base: "Blade Runner (1982)", name: "blade runner (1982).EN.SRT", language: "en", codec: "srt", ok: true},
		{base: "Blade Runner (1982)", name: "Blade Runner (1982).mkv"},
		{base: "Blade Runner (1982)", name: "Blade Runner (1982) Extras.en.srt"},
		{base: "Blade Runner (1982)", name: "Other Movie.en.srt"},
	}

	for _, test := range tests {
		subtitle, ok := parseSubtitle(test.base, test.name)
		if ok != test.ok {
			t.Errorf("parseSubtitle(%q) ok = %v, want %v", test.name, ok, test.ok)
			continue
		}
		if !ok {
			continue
		}
		if subtitle.Language != test.language {
			t.Errorf("parseSubtitle(%q) language = %q, want %q", test.name, subtitle.Language, test.language)
		}
		if subtitle.Title != test.title {
			t.Errorf("parseSubtitle(%q) title = %q, want %q", test.name, subtitle.Title, test.title)
		}
		if subtitle.Codec != test.codec {
			t.Errorf("parseSubtitle(%q) codec = %q, want %q", test.name, subtitle.Codec, test.codec)
		}
		if subtitle.IsForced != test.forced {
			t.Errorf("parseSubtitle(%q) forced = %v, want %v", test.name, subtitle.IsForced, test.forced)
		}
		if subtitle.IsDefault != test.isDefault {
			t.Errorf("parseSubtitle(%q) default = %v, want %v", test.name, subtitle.IsDefault, test.isDefault)
		}
		if subtitle.IsHearingImpaired != test.impaired {
			t.Errorf("parseSubtitle(%q) hearing impaired = %v, want %v", test.name, subtitle.IsHearingImpaired, test.impaired)
		}
	}
}

func TestSortName(t *testing.T) {
	tests := map[string]string{
		"The Wire":     "wire",
		"A New Hope":   "new hope",
		"An Education": "education",
		"Blade Runner": "blade runner",
	}

	for in, want := range tests {
		if got := sortName(in); got != want {
			t.Errorf("sortName(%q) = %q, want %q", in, got, want)
		}
	}
}
