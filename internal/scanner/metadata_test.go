package scanner

import (
	"slices"
	"testing"

	"github.com/FreekingDean/gojellyfin/internal/ffmpeg"
	"github.com/FreekingDean/gojellyfin/internal/items"
	creditmodal "github.com/FreekingDean/gojellyfin/internal/store/credit"
)

func TestMetadata(t *testing.T) {
	t.Run("genres", func(t *testing.T) {
		cases := []struct {
			name  string
			value string
			want  []string
		}{
			{name: "single", value: "Comedy", want: []string{"Comedy"}},
			{name: "slash separated", value: "Comedy/Drama", want: []string{"Comedy", "Drama"}},
			{name: "semicolon separated", value: "Comedy; Drama", want: []string{"Comedy", "Drama"}},
			{name: "pipe separated", value: "Comedy|Drama", want: []string{"Comedy", "Drama"}},
			{name: "comma separated", value: "Comedy, Drama", want: []string{"Comedy", "Drama"}},
			{name: "mixed delimiters", value: "Comedy, Drama; Action", want: []string{"Comedy", "Drama", "Action"}},
			{name: "mixed slash and comma", value: "Comedy/Drama, Action", want: []string{"Comedy", "Drama", "Action"}},
			{name: "repeated", value: "Comedy/Comedy", want: []string{"Comedy"}},
			{name: "padded", value: "  Comedy ,  Drama  ", want: []string{"Comedy", "Drama"}},
			{name: "empty segments", value: "Comedy;;Drama", want: []string{"Comedy", "Drama"}},
			{name: "unicode", value: "アニメ, ドラマ", want: []string{"アニメ", "ドラマ"}},
			{name: "multi word", value: "Science Fiction|Film Noir", want: []string{"Science Fiction", "Film Noir"}},
			{name: "empty", value: "", want: []string{}},
			{name: "blanks only", value: " / ", want: []string{}},
			{name: "delimiters only", value: ",;|/", want: []string{}},
		}

		for _, test := range cases {
			t.Run(test.name, func(t *testing.T) {
				got := metadata(&ffmpeg.Probe{Format: ffmpeg.Format{Tags: map[string]string{"genre": test.value}}})
				if !slices.Equal(got.Genres, test.want) {
					t.Errorf("genres = %v, want %v", got.Genres, test.want)
				}
			})
		}
	})

	t.Run("tag keys", func(t *testing.T) {
		probe := &ffmpeg.Probe{Format: ffmpeg.Format{Tags: map[string]string{
			"GENRE":        "Rock",
			"ALBUM_ARTIST": "The Band",
			"WRITTEN-BY":   "A Writer",
			"keywords":     "live;remaster",
			"publisher":    "A Label",
		}}}

		got := metadata(probe)

		if want := []string{"Rock"}; !slices.Equal(got.Genres, want) {
			t.Errorf("genres = %v, want %v", got.Genres, want)
		}
		if want := []string{"live", "remaster"}; !slices.Equal(got.Tags, want) {
			t.Errorf("tags = %v, want %v", got.Tags, want)
		}
		if want := []string{"A Label"}; !slices.Equal(got.Studios, want) {
			t.Errorf("studios = %v, want %v", got.Studios, want)
		}
		want := []items.Person{
			{Name: "The Band", Kind: creditmodal.KindAlbumArtist},
			{Name: "A Writer", Kind: creditmodal.KindWriter},
		}
		if !slices.Equal(got.People, want) {
			t.Errorf("people = %v, want %v", got.People, want)
		}
	})

	t.Run("people", func(t *testing.T) {
		probe := &ffmpeg.Probe{Format: ffmpeg.Format{Tags: map[string]string{
			"artist":    "First/Second",
			"composer":  "First",
			"writer":    "A Writer",
			"WrittenBy": "A Writer",
			"director":  "",
		}}}

		want := []items.Person{
			{Name: "First", Kind: creditmodal.KindArtist},
			{Name: "Second", Kind: creditmodal.KindArtist},
			{Name: "First", Kind: creditmodal.KindComposer},
			{Name: "A Writer", Kind: creditmodal.KindWriter},
		}
		if got := metadata(probe).People; !slices.Equal(got, want) {
			t.Errorf("people = %v, want %v", got, want)
		}
	})

	t.Run("people keep commas", func(t *testing.T) {
		probe := &ffmpeg.Probe{Format: ffmpeg.Format{Tags: map[string]string{
			"artist":    "Doe, John; Roe, Jane",
			"composer":  "Bach, Johann Sebastian",
			"publisher": "Warner Bros., Inc.",
		}}}

		got := metadata(probe)

		want := []items.Person{
			{Name: "Doe, John", Kind: creditmodal.KindArtist},
			{Name: "Roe, Jane", Kind: creditmodal.KindArtist},
			{Name: "Bach, Johann Sebastian", Kind: creditmodal.KindComposer},
		}
		if !slices.Equal(got.People, want) {
			t.Errorf("people = %v, want %v", got.People, want)
		}
		if want := []string{"Warner Bros., Inc."}; !slices.Equal(got.Studios, want) {
			t.Errorf("studios = %v, want %v", got.Studios, want)
		}
	})

	t.Run("stream tags", func(t *testing.T) {
		probe := &ffmpeg.Probe{
			Format: ffmpeg.Format{Tags: map[string]string{"studio": "A Studio"}},
			Streams: []ffmpeg.Stream{
				{CodecType: "audio", Tags: map[string]string{"genre": "Jazz", "studio": "Ignored"}},
				{CodecType: "subtitle", Tags: map[string]string{"genre": "Nonsense"}},
			},
		}

		got := metadata(probe)

		if want := []string{"Jazz"}; !slices.Equal(got.Genres, want) {
			t.Errorf("genres = %v, want %v", got.Genres, want)
		}
		if want := []string{"A Studio"}; !slices.Equal(got.Studios, want) {
			t.Errorf("studios = %v, want %v", got.Studios, want)
		}
	})

	t.Run("studios deduplicate", func(t *testing.T) {
		probe := &ffmpeg.Probe{Format: ffmpeg.Format{Tags: map[string]string{
			"studio":    "A Studio",
			"publisher": "A Studio",
		}}}

		if want := []string{"A Studio"}; !slices.Equal(metadata(probe).Studios, want) {
			t.Errorf("studios = %v, want %v", metadata(probe).Studios, want)
		}
	})

	t.Run("without tags", func(t *testing.T) {
		got := metadata(&ffmpeg.Probe{})

		if len(got.Genres) != 0 || len(got.Studios) != 0 || len(got.Tags) != 0 || len(got.People) != 0 {
			t.Errorf("metadata = %+v, want empty", got)
		}
	})
}
