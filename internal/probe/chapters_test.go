package probe

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/FreekingDean/gojellyfin/internal/storetest"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/ffmpeg"
	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/reports"
	itemmodel "github.com/FreekingDean/gojellyfin/internal/store/item"
	segmentmodel "github.com/FreekingDean/gojellyfin/internal/store/mediasegment"
	sourcemodel "github.com/FreekingDean/gojellyfin/internal/store/source"
)

func TestChapterKind(t *testing.T) {
	cases := map[string]items.SegmentKind{
		"Intro":              segmentmodel.KindIntro,
		"Opening Credits":    segmentmodel.KindIntro,
		"OP":                 segmentmodel.KindIntro,
		"Previously On":      segmentmodel.KindRecap,
		"Recap":              segmentmodel.KindRecap,
		"End Credits":        segmentmodel.KindOutro,
		"Credits":            segmentmodel.KindOutro,
		"ED":                 segmentmodel.KindOutro,
		"Next Episode":       segmentmodel.KindPreview,
		"Preview":            segmentmodel.KindPreview,
		"Chapter 01":         "",
		"Operation Overlord": "",
		"Opposites Attract":  "",
		"":                   "",
	}

	for title, want := range cases {
		got, ok := chapterKind(title)
		if want == "" && ok {
			t.Errorf("%q read as %s, want no segment", title, got)
		}
		if want != "" && got != want {
			t.Errorf("%q read as %q, want %s", title, got, want)
		}
	}
}

func TestProber_ReadsChapterSegments(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg is not on PATH")
	}
	ctx := context.Background()

	connection := storetest.Open(t)
	t.Cleanup(func() {
	})
	client := connection.Client()

	dir := t.TempDir()
	chapters := filepath.Join(dir, "chapters.txt")
	metadata := ";FFMETADATA1\n" +
		"[CHAPTER]\nTIMEBASE=1/1000\nSTART=0\nEND=2000\ntitle=Chapter 1\n" +
		"[CHAPTER]\nTIMEBASE=1/1000\nSTART=2000\nEND=5000\ntitle=Opening\n" +
		"[CHAPTER]\nTIMEBASE=1/1000\nSTART=5000\nEND=8000\ntitle=Chapter 3\n" +
		"[CHAPTER]\nTIMEBASE=1/1000\nSTART=8000\nEND=10000\ntitle=End Credits\n"
	if err := os.WriteFile(chapters, []byte(metadata), 0o600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "episode.mkv")
	build := exec.Command("ffmpeg", "-nostdin", "-loglevel", "error",
		"-f", "lavfi", "-i", "sine=frequency=440:duration=10",
		"-i", chapters, "-map_metadata", "1", "-map_chapters", "1", "-c:a", "aac", path)
	if output, err := build.CombinedOutput(); err != nil {
		t.Skipf("this ffmpeg cannot build the fixture: %v: %s", err, output)
	}

	name := t.Name() + "-" + uuid.NewString()
	downloader, err := client.Source.Create().
		SetName(name).
		SetURL("http://" + uuid.NewString() + ".invalid").
		SetAPIKeyVariable("SOURCE_API_KEY_TEST").
		SetKind(sourcemodel.KindSonarr).
		Save(ctx)
	if err != nil {
		t.Fatalf("failed to create the source: %v", err)
	}
	episode, err := client.Item.Create().SetKind(itemmodel.KindEpisode).SetKey(name).SetName(name).SetSortName(name).Save(ctx)
	if err != nil {
		t.Fatalf("failed to create the episode: %v", err)
	}
	t.Cleanup(func() {
		if err := client.Item.DeleteOne(episode).Exec(context.Background()); err != nil {
			t.Errorf("failed to delete the episode: %v", err)
		}
		if err := client.Source.DeleteOne(downloader).Exec(context.Background()); err != nil {
			t.Errorf("failed to delete the source: %v", err)
		}
	})
	file, err := client.ItemSource.Create().SetItemID(episode.ID).SetSourceID(downloader.ID).SetName(name).SetPath(path).Save(ctx)
	if err != nil {
		t.Fatalf("failed to create the file: %v", err)
	}
	t.Cleanup(func() {
		if err := client.ItemSource.DeleteOne(file).Exec(context.Background()); err != nil {
			t.Errorf("failed to delete the file: %v", err)
		}
	})

	prober := New(items.New(client), ffmpeg.New(), reports.New(client, nil))
	for range 2 {
		if err := prober.ProbeSource(ctx, file.ID); err != nil {
			t.Fatalf("failed to probe: %v", err)
		}
		if err := client.ItemSource.UpdateOne(file).ClearProbedAt().Exec(ctx); err != nil {
			t.Fatal(err)
		}
	}

	found, err := client.MediaSegment.Query().
		Where(segmentmodel.ItemSourceID(file.ID)).
		Order(segmentmodel.ByStartTicks()).
		All(ctx)
	if err != nil {
		t.Fatal(err)
	}

	want := proberReadsChapterSegmentsCases
	if len(found) != len(want) {
		t.Fatalf("segments = %d, want %d after probing twice", len(found), len(want))
	}
	for i, segment := range found {
		if segment.Kind != want[i].kind || segment.Origin != segmentmodel.OriginChapter ||
			segment.StartTicks != want[i].start || segment.EndTicks != want[i].end {
			t.Errorf("segment %d = %s %s %d-%d, want %s from a chapter %d-%d", i,
				segment.Kind, segment.Origin, segment.StartTicks, segment.EndTicks,
				want[i].kind, want[i].start, want[i].end)
		}
	}
}

type proberReadsChapterSegmentsCase struct {
	kind       segmentmodel.Kind
	start, end int64
}

var proberReadsChapterSegmentsCases = []proberReadsChapterSegmentsCase{
	{segmentmodel.KindIntro, 20_000_000, 50_000_000},
	{segmentmodel.KindOutro, 80_000_000, 100_000_000},
}
