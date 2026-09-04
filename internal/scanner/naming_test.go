package scanner

import "testing"

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
