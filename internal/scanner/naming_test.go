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
