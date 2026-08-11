package scanner

import (
	"slices"
	"testing"

	"github.com/FreekingDean/gojellyfin/internal/ffmpeg"
	"github.com/FreekingDean/gojellyfin/internal/items"
	creditmodal "github.com/FreekingDean/gojellyfin/internal/store/credit"
)

func TestMetadataGenres(t *testing.T) {
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
		{name: "repeated", value: "Comedy/Comedy", want: []string{"Comedy"}},
		{name: "empty", value: "", want: []string{}},
		{name: "blanks only", value: " / ", want: []string{}},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			got := metadata(&ffmpeg.Probe{Format: ffmpeg.Format{Tags: map[string]string{"genre": test.value}}})
			if !slices.Equal(got.Genres, test.want) {
				t.Errorf("genres = %v, want %v", got.Genres, test.want)
			}
		})
	}
}

func TestMetadataTagKeys(t *testing.T) {
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
}

func TestMetadataPeople(t *testing.T) {
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
}

func TestMetadataStreamTags(t *testing.T) {
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
}

func TestMetadataStudiosDeduplicate(t *testing.T) {
	probe := &ffmpeg.Probe{Format: ffmpeg.Format{Tags: map[string]string{
		"studio":    "A Studio",
		"publisher": "A Studio",
	}}}

	if want := []string{"A Studio"}; !slices.Equal(metadata(probe).Studios, want) {
		t.Errorf("studios = %v, want %v", metadata(probe).Studios, want)
	}
}

func TestMetadataWithoutTags(t *testing.T) {
	got := metadata(&ffmpeg.Probe{})

	if len(got.Genres) != 0 || len(got.Studios) != 0 || len(got.Tags) != 0 || len(got.People) != 0 {
		t.Errorf("metadata = %+v, want empty", got)
	}
}
