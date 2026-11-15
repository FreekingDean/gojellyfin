package probe

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/env"
	"github.com/FreekingDean/gojellyfin/internal/ffmpeg"
	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/reports"
	"github.com/FreekingDean/gojellyfin/internal/store"
	itemmodel "github.com/FreekingDean/gojellyfin/internal/store/item"
	reportmodel "github.com/FreekingDean/gojellyfin/internal/store/report"
	sourcemodel "github.com/FreekingDean/gojellyfin/internal/store/source"
)

func TestProber(t *testing.T) {
	ctx := context.Background()

	config, err := env.Load()
	if err != nil {
		t.Fatalf("failed to read the environment: %v", err)
	}
	connection, err := store.NewStore(config)
	if err != nil {
		t.Fatalf("failed to open the database: %v", err)
	}
	if err := connection.Start(); err != nil {
		t.Fatalf("failed to reach the database, set DATABASE_URL: %v", err)
	}
	t.Cleanup(func() {
		if err := connection.Stop(); err != nil {
			t.Errorf("failed to close the database: %v", err)
		}
	})
	client := connection.Client()

	name := t.Name() + "-" + uuid.NewString()
	downloader, err := client.Source.Create().
		SetName(name).
		SetURL("http://" + uuid.NewString() + ".invalid").
		SetAPIKeyVariable("SOURCE_API_KEY_TEST").
		SetKind(sourcemodel.KindRadarr).
		Save(ctx)
	if err != nil {
		t.Fatalf("failed to create the source: %v", err)
	}
	t.Cleanup(func() {
		if err := client.Source.DeleteOne(downloader).Exec(context.Background()); err != nil {
			t.Errorf("failed to delete the source: %v", err)
		}
	})

	held := func(t *testing.T, path string) (uuid.UUID, uuid.UUID) {
		t.Helper()

		title := name + "-" + uuid.NewString()
		movie, err := client.Item.Create().SetKind(itemmodel.KindMovie).SetKey(title).SetName(title).SetSortName(title).Save(ctx)
		if err != nil {
			t.Fatalf("failed to create the movie: %v", err)
		}
		t.Cleanup(func() {
			if err := client.Item.DeleteOne(movie).Exec(context.Background()); err != nil {
				t.Errorf("failed to delete the movie: %v", err)
			}
		})

		file, err := client.ItemSource.Create().SetItemID(movie.ID).SetSourceID(downloader.ID).SetName(title).SetPath(path).Save(ctx)
		if err != nil {
			t.Fatalf("failed to create the file: %v", err)
		}

		return movie.ID, file.ID
	}

	prober := New(items.New(client), ffmpeg.New(), reports.New(client, nil))
	directory := t.TempDir()

	t.Run("a file ffprobe refuses is reported once", func(t *testing.T) {
		path := filepath.Join(directory, "broken.mkv")
		if err := os.WriteFile(path, []byte("not a film"), 0o600); err != nil {
			t.Fatalf("failed to write the file: %v", err)
		}
		movie, file := held(t, path)

		for range 2 {
			if err := prober.ProbeSource(ctx, file); err == nil {
				t.Fatalf("probing a broken file answered success")
			}
		}

		filed, err := client.Report.Query().Where(reportmodel.ItemID(movie)).AllModels(ctx)
		if err != nil {
			t.Fatalf("failed to query the reports: %v", err)
		}
		if len(filed) != 1 {
			t.Fatalf("got %d reports, want one", len(filed))
		}
		if filed[0].Origin != reports.OriginProbe || filed[0].ItemSourceID == nil || *filed[0].ItemSourceID != file || filed[0].Note == "" {
			t.Errorf("report = %+v", filed[0])
		}
	})

	t.Run("a file that is not there is the sweep's to find", func(t *testing.T) {
		movie, file := held(t, filepath.Join(directory, "gone.mkv"))

		if err := prober.ProbeSource(ctx, file); err == nil {
			t.Fatalf("probing a missing file answered success")
		}

		reported, err := client.Report.Query().Where(reportmodel.ItemID(movie)).Exist(ctx)
		if err != nil {
			t.Fatalf("failed to query the reports: %v", err)
		}
		if reported {
			t.Errorf("a missing file was reported as broken")
		}
	})

	t.Run("a film is stored with its keyframes and a song with none", func(t *testing.T) {
		film := filepath.Join(directory, "film.mkv")
		song := filepath.Join(directory, "song.mka")
		for path, args := range map[string][]string{
			film: {"-f", "lavfi", "-i", "testsrc=size=160x120:rate=25:duration=4", "-c:v", "libx264", "-g", "50", "-sc_threshold", "0"},
			song: {"-f", "lavfi", "-i", "sine=frequency=440:duration=1", "-c:a", "aac"},
		} {
			build := exec.Command("ffmpeg", append(append([]string{"-nostdin", "-loglevel", "error"}, args...), path)...)
			if output, err := build.CombinedOutput(); err != nil {
				t.Skipf("this ffmpeg cannot build the fixture: %v: %s", err, output)
			}
		}

		_, filmFile := held(t, film)
		_, songFile := held(t, song)
		for _, file := range []uuid.UUID{filmFile, songFile} {
			if err := prober.ProbeSource(ctx, file); err != nil {
				t.Fatalf("failed to probe: %v", err)
			}
		}

		library := items.New(client)
		stored, err := library.Keyframes(ctx, filmFile)
		if err != nil {
			t.Fatal(err)
		}
		if want := []int64{0, 20_000_000}; !slices.Equal(stored, want) {
			t.Errorf("film keyframes = %v, want %v", stored, want)
		}

		if err := prober.ProbeSource(ctx, filmFile); err != nil {
			t.Fatalf("failed to probe the film again: %v", err)
		}

		songIndex, err := library.Keyframes(ctx, songFile)
		if err != nil {
			t.Fatal(err)
		}
		if songIndex != nil {
			t.Errorf("a song was stored with a keyframe index %v", songIndex)
		}
	})
}
