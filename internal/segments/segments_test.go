package segments

import (
	"context"
	"fmt"
	"os/exec"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/env"
	"github.com/FreekingDean/gojellyfin/internal/ffmpeg"
	"github.com/FreekingDean/gojellyfin/internal/store"
	itemmodel "github.com/FreekingDean/gojellyfin/internal/store/item"
	segmentmodel "github.com/FreekingDean/gojellyfin/internal/store/mediasegment"
	sourcemodel "github.com/FreekingDean/gojellyfin/internal/store/source"
)

type fixture struct {
	client     *store.Client
	service    *Service
	downloader uuid.UUID
	season     uuid.UUID
	name       string
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
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
	client := connection.Client()

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
	season, err := client.Item.Create().SetKind(itemmodel.KindSeason).SetKey(name).SetName(name).SetSortName(name).Save(ctx)
	if err != nil {
		t.Fatalf("failed to create the season: %v", err)
	}

	t.Cleanup(func() {
		ctx := context.Background()
		if _, err := client.Item.Delete().Where(itemmodel.ParentID(season.ID)).Exec(ctx); err != nil {
			t.Errorf("failed to delete the episodes: %v", err)
		}
		if err := client.Item.DeleteOne(season).Exec(ctx); err != nil {
			t.Errorf("failed to delete the season: %v", err)
		}
		if err := client.Source.DeleteOne(downloader).Exec(ctx); err != nil {
			t.Errorf("failed to delete the source: %v", err)
		}
		if err := connection.Stop(); err != nil {
			t.Errorf("failed to close the database: %v", err)
		}
	})

	return &fixture{client: client, service: New(client, ffmpeg.New()), downloader: downloader.ID, season: season.ID, name: name}
}

func (f *fixture) episode(t *testing.T, number int32, path string) (uuid.UUID, uuid.UUID) {
	t.Helper()
	ctx := context.Background()

	key := fmt.Sprintf("%s:%d", f.name, number)
	item, err := f.client.Item.Create().
		SetKind(itemmodel.KindEpisode).
		SetParentID(f.season).
		SetIndexNumber(number).
		SetKey(key).SetName(key).SetSortName(key).
		Save(ctx)
	if err != nil {
		t.Fatalf("failed to create the episode: %v", err)
	}

	file, err := f.client.ItemSource.Create().
		SetItemID(item.ID).
		SetSourceID(f.downloader).
		SetName(key).
		SetPath(path).
		SetRunTimeTicks(int64(30 * time.Minute / 100)).
		Save(ctx)
	if err != nil {
		t.Fatalf("failed to create the file: %v", err)
	}
	t.Cleanup(func() {
		if err := f.client.ItemSource.DeleteOne(file).Exec(context.Background()); err != nil {
			t.Errorf("failed to delete the file: %v", err)
		}
	})

	return item.ID, file.ID
}

func TestService_DetectSeason(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg is not on PATH")
	}
	ctx := context.Background()
	f := newFixture(t)
	dir := t.TempDir()

	starts := []float64{12.37, 5.5, 3.21}
	var episodes, files []uuid.UUID
	for i, start := range starts {
		episode, file := f.episode(t, int32(i+1), episode(t, dir, i+1, start, "128k"))
		episodes = append(episodes, episode)
		files = append(files, file)
	}

	err := f.client.MediaSegment.Create().
		SetItemSourceID(files[2]).
		SetKind(segmentmodel.KindIntro).
		SetOrigin(segmentmodel.OriginChapter).
		SetStartTicks(30_000_000).
		SetEndTicks(330_000_000).
		Exec(ctx)
	if err != nil {
		t.Fatal(err)
	}

	needing, err := f.service.SeasonsNeedingDetection(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !contains(needing, f.season) {
		t.Fatal("the season is not waiting for intro detection")
	}

	if err := f.service.DetectSeason(ctx, f.season); err != nil {
		t.Fatalf("failed to detect: %v", err)
	}

	for i, episode := range episodes[:2] {
		_, found, err := f.service.ForItem(ctx, episode, nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(found) != 1 || found[0].Kind != segmentmodel.KindIntro || found[0].Origin != segmentmodel.OriginFingerprint {
			t.Fatalf("episode %d segments = %v, want one intro it was heard to share", i+1, found)
		}
		start, end := time.Duration(found[0].StartTicks*100), time.Duration(found[0].EndTicks*100)
		if !near(start, secs(starts[i])) || !near(end, secs(starts[i]+30)) {
			t.Errorf("episode %d intro = %v to %v, want %.2fs to %.2fs", i+1, start, end, starts[i], starts[i]+30)
		}
	}

	owner, chaptered, err := f.service.ForItem(ctx, files[2], []Kind{segmentmodel.KindIntro})
	if err != nil {
		t.Fatal(err)
	}
	if owner != episodes[2] {
		t.Errorf("file's item = %v, want %v", owner, episodes[2])
	}
	if len(chaptered) != 1 || chaptered[0].Origin != segmentmodel.OriginChapter {
		t.Errorf("chaptered file = %v, want the chapter's intro", chaptered)
	}

	_, outros, err := f.service.ForItem(ctx, episodes[0], []Kind{segmentmodel.KindOutro})
	if err != nil {
		t.Fatal(err)
	}
	if len(outros) != 0 {
		t.Errorf("outros = %v, want none", outros)
	}

	needing, err = f.service.SeasonsNeedingDetection(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if contains(needing, f.season) {
		t.Error("the season still waits for detection after it ran")
	}
}

func TestService_DetectSeason_OneEpisode(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg is not on PATH")
	}
	ctx := context.Background()
	f := newFixture(t)

	only, _ := f.episode(t, 1, episode(t, t.TempDir(), 1, 10, "128k"))
	if err := f.service.DetectSeason(ctx, f.season); err != nil {
		t.Fatalf("failed to detect: %v", err)
	}

	_, found, err := f.service.ForItem(ctx, only, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 0 {
		t.Errorf("segments = %v, want none with nothing to compare against", found)
	}
}

func contains(ids []uuid.UUID, want uuid.UUID) bool {
	for _, id := range ids {
		if id == want {
			return true
		}
	}

	return false
}
