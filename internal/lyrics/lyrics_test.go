package lyrics

import "testing"

func TestParse(t *testing.T) {
	t.Run("a timed file is synced and ordered by when each line is sung", func(t *testing.T) {
		parsed := Parse("[ar:The Band]\n[ti:Opener]\n[offset:250]\n[00:12.50]second\n[00:01.2][00:30.00]first and again\n\n")

		if !parsed.Synced {
			t.Fatal("a file with timestamps was read as unsynced")
		}
		if parsed.Metadata.Artist != "The Band" || parsed.Metadata.Title != "Opener" {
			t.Errorf("metadata = %+v, want the artist and title tags", parsed.Metadata)
		}
		if parsed.Metadata.Offset == nil || *parsed.Metadata.Offset != 2_500_000 {
			t.Errorf("offset = %v, want 250ms in ticks", parsed.Metadata.Offset)
		}

		want := []struct {
			start int64
			text  string
		}{{12_000_000, "first and again"}, {125_000_000, "second"}, {300_000_000, "first and again"}}
		if len(parsed.Lines) != len(want) {
			t.Fatalf("lines = %+v, want %d", parsed.Lines, len(want))
		}
		for i, line := range parsed.Lines {
			if *line.Start != want[i].start || line.Text != want[i].text {
				t.Errorf("line %d = %d %q, want %d %q", i, *line.Start, line.Text, want[i].start, want[i].text)
			}
		}
	})

	t.Run("plain text is unsynced and kept whole", func(t *testing.T) {
		parsed := Parse("verse one\r\nverse two\n")

		if parsed.Synced {
			t.Error("plain text was read as synced")
		}
		if len(parsed.Lines) != 2 || parsed.Lines[0].Start != nil || parsed.Lines[1].Text != "verse two" {
			t.Errorf("lines = %+v, want both lines with no start", parsed.Lines)
		}
	})
}
