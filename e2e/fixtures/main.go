package main

import (
	"cmp"
	"context"
	"crypto/sha1"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/FreekingDean/gojellyfin/internal/config"
	"github.com/FreekingDean/gojellyfin/internal/env"
	"github.com/FreekingDean/gojellyfin/internal/ffmpeg"
	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/libraries"
	"github.com/FreekingDean/gojellyfin/internal/probe"
	"github.com/FreekingDean/gojellyfin/internal/reports"
	"github.com/FreekingDean/gojellyfin/internal/shelves"
	"github.com/FreekingDean/gojellyfin/internal/store"
	imagemodel "github.com/FreekingDean/gojellyfin/internal/store/image"
	itemmodel "github.com/FreekingDean/gojellyfin/internal/store/item"
	sourcemodel "github.com/FreekingDean/gojellyfin/internal/store/source"
	"github.com/FreekingDean/gojellyfin/scripts/jellyfinmigrate/migration/fixture"
)

const (
	library    = "Smoke Movies"
	playable   = "Fixture Playable"
	shows      = "Smoke Shows"
	series     = "Fixture Show"
	season     = "Season 1"
	drama      = "Fixture Drama"
	finale     = "Fixture Finale"
	transcoded = "Fixture Transcoded"

	requestProfile = 1

	downloaderTitle = 71
	downloaderFile  = 72
)

var (
	shelved  = []string{"recently-aired", "new-releases", "trending-movies", "trending-shows"}
	movies   = []string{"Fixture Alpha", "Fixture Beta"}
	episodes = []string{"Fixture Pilot", "Fixture Second"}

	released = map[string]int{
		"Fixture Alpha":  2021,
		"Fixture Beta":   2019,
		"Fixture Pilot":  2020,
		"Fixture Second": 2024,
		"Fixture Finale": 2022,
	}

	posterShades = map[string]color.RGBA{
		"Fixture Alpha": {R: 220, G: 40, B: 40, A: 255},
		"Fixture Beta":  {R: 40, G: 60, B: 220, A: 255},
		"Fixture Show":  {R: 40, G: 200, B: 70, A: 255},
		"Fixture Drama": {R: 40, G: 200, B: 70, A: 255},
	}

	unownedShades = map[string]color.RGBA{
		"Wanted Film":   {R: 230, G: 150, B: 30, A: 255},
		"Wanted Show":   {R: 150, G: 60, B: 200, A: 255},
		"Asked Picture": {R: 40, G: 170, B: 120, A: 255},
	}

	backdropShades = map[string]color.RGBA{
		"Fixture Beta": {R: 230, G: 160, B: 30, A: 255},
		"Fixture Show": {R: 150, G: 40, B: 200, A: 255},
	}

	resumed = []string{"Fixture Alpha", "Fixture Beta", "Fixture Pilot"}
)

func main() {
	if len(os.Args) < 2 {
		fail(fmt.Errorf("usage: fixtures create|drop|seed|oidc|jellyfin"))
	}

	var err error
	switch os.Args[1] {
	case "create":
		err = create()
	case "drop":
		err = drop()
	case "seed":
		err = seed()
	case "oidc":
		err = oidc()
	case "jellyfin":
		err = jellyfin()
	default:
		err = fmt.Errorf("unknown action %q", os.Args[1])
	}
	if err != nil {
		fail(err)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

func jellyfin() error {
	if len(os.Args) < 3 {
		return fmt.Errorf("usage: fixtures jellyfin <directory>")
	}
	if err := fixture.Write(os.Args[2], fixture.Legacy); err != nil {
		return err
	}

	return json.NewEncoder(os.Stdout).Encode(map[string]string{
		"serverId": fixture.ServerID,
		"userId":   fixture.AliceID,
		"username": fixture.AliceUsername,
		"token":    fixture.AliceWebToken,
		"deviceId": fixture.AliceWebDevice,
	})
}

func create() error {
	name, err := scratchName()
	if err != nil {
		return err
	}

	if err := admin("CREATE DATABASE " + name); err != nil {
		return err
	}

	fmt.Println(name)

	return nil
}

func drop() error {
	name, err := scratchName()
	if err != nil {
		return err
	}

	return admin("DROP DATABASE IF EXISTS " + name + " WITH (FORCE)")
}

func scratchName() (string, error) {
	name := os.Getenv("SCRATCH_DATABASE")
	if name == "" {
		return "", fmt.Errorf("SCRATCH_DATABASE is not set")
	}
	if strings.Trim(name, "abcdefghijklmnopqrstuvwxyz0123456789_") != "" {
		return "", fmt.Errorf("SCRATCH_DATABASE %q is not a bare identifier", name)
	}

	return name, nil
}

func admin(statement string) error {
	dsn := os.Getenv("ADMIN_DATABASE_URL")
	if dsn == "" {
		return fmt.Errorf("ADMIN_DATABASE_URL is not set")
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	if _, err := db.ExecContext(context.Background(), statement); err != nil {
		return fmt.Errorf("%q failed: %w", statement, err)
	}

	return nil
}

func seed() error {
	environment, err := env.Load()
	if err != nil {
		return err
	}

	connection, err := store.NewStore(environment)
	if err != nil {
		return err
	}
	if err := connection.Start(); err != nil {
		return err
	}
	defer func() { _ = connection.Stop() }()

	if err := paintUnowned(); err != nil {
		return err
	}

	ctx := context.Background()
	client := connection.Client()
	modified := time.Now()

	record, err := libraries.New(client).CreateLibrary(ctx, library, libraries.CollectionTypeMovies, []string{"/fixtures"})
	if err != nil {
		return err
	}

	downloader, err := client.Source.Create().
		SetName("Fixtures").
		SetURL(cmp.Or(os.Getenv("FIXTURE_DOWNLOADER"), "http://fixtures.invalid")).
		SetAPIKeyVariable(env.SourceAPIKeyPrefix + "FIXTURES").
		SetKind(sourcemodel.KindRadarr).
		SetRequestProfileID(requestProfile).
		SetRequestRootFolder("/fixtures").
		Save(ctx)
	if err != nil {
		return err
	}

	if err := client.SourcePath.Create().
		SetSourceID(downloader.ID).
		SetRemotePath("/fixtures").
		SetLocalPath("/fixtures").
		Exec(ctx); err != nil {
		return err
	}

	if err := shelves.New(nil, nil, config.New(client)).Enable(ctx, shelved); err != nil {
		return err
	}

	catalogue := items.New(client)
	watching := map[string]uuid.UUID{}
	for _, name := range movies {
		item, err := catalogue.SaveScanned(ctx, items.Item{
			Kind:         itemmodel.KindMovie,
			Key:          "movie:" + slugify(name),
			Name:         name,
			SortName:     strings.ToLower(name),
			DateModified: &modified,
		})
		if err != nil {
			return err
		}

		if err := file(ctx, catalogue, downloader.ID, item.ID, name); err != nil {
			return err
		}
		if err := member(ctx, client, record.ID, downloader.ID, item.ID); err != nil {
			return err
		}
		if err := poster(ctx, catalogue, item.ID, name); err != nil {
			return err
		}
		if err := aired(ctx, catalogue, item.ID, released[name]); err != nil {
			return err
		}
		if err := backdrop(ctx, catalogue, item.ID, name); err != nil {
			return err
		}
		watching[name] = item.ID
	}

	trending, err := strconv.Atoi(os.Getenv("FIXTURE_TRENDING_TMDB"))
	if err != nil {
		return fmt.Errorf("FIXTURE_TRENDING_TMDB names no TMDB id: %w", err)
	}
	if err := playableMovie(ctx, client, catalogue, record.ID, downloader.ID, playable, items.MovieKey(trending), os.Getenv("FIXTURE_MEDIA")); err != nil {
		return err
	}
	if err := playableMovie(ctx, client, catalogue, record.ID, downloader.ID, transcoded, "movie:"+slugify(transcoded), os.Getenv("FIXTURE_TRANSCODE")); err != nil {
		return err
	}

	shown, err := libraries.New(client).CreateLibrary(ctx, shows, libraries.CollectionTypeTvshows, []string{"/fixtures/shows"})
	if err != nil {
		return err
	}

	shelf := shelf{client: client, catalogue: catalogue, libraryID: shown.ID, sourceID: downloader.ID, watching: watching, media: os.Getenv("FIXTURE_EPISODES")}
	if err := shelf.series(ctx, series, episodes, modified); err != nil {
		return err
	}
	if err := shelf.series(ctx, drama, []string{finale}, modified.Add(time.Minute)); err != nil {
		return err
	}

	if err := resume(ctx, client, watching); err != nil {
		return err
	}

	fmt.Println(record.ID)

	return nil
}

func resume(ctx context.Context, client *store.Client, watching map[string]uuid.UUID) error {
	viewer, err := client.User.Query().Only(ctx)
	if err != nil {
		return err
	}

	started := time.Now().Add(-time.Hour)
	for order, name := range resumed {
		played := started.Add(-time.Duration(order) * time.Minute)
		if err := client.UserItemData.Create().
			SetUserID(viewer.ID).
			SetItemID(watching[name]).
			SetPlaybackPositionTicks(int64(5 * time.Second / 100)).
			SetLastPlayedAt(played).
			Exec(ctx); err != nil {
			return err
		}
	}

	return nil
}

type shelf struct {
	client    *store.Client
	catalogue *items.Service
	libraryID uuid.UUID
	sourceID  uuid.UUID
	watching  map[string]uuid.UUID
	media     string
}

func (s shelf) series(ctx context.Context, name string, titles []string, modified time.Time) error {
	number := int32(1)

	show, err := s.catalogue.SaveScanned(ctx, items.Item{
		Kind:         itemmodel.KindSeries,
		Key:          "series:" + slugify(name),
		Name:         name,
		SortName:     strings.ToLower(name),
		DateModified: &modified,
	})
	if err != nil {
		return err
	}
	if err := poster(ctx, s.catalogue, show.ID, name); err != nil {
		return err
	}
	if err := backdrop(ctx, s.catalogue, show.ID, name); err != nil {
		return err
	}

	first, err := s.catalogue.SaveScanned(ctx, items.Item{
		ParentID:     &show.ID,
		Kind:         itemmodel.KindSeason,
		Key:          "season:" + slugify(name) + ":1",
		Name:         season,
		SortName:     strings.ToLower(season),
		IndexNumber:  &number,
		DateModified: &modified,
	})
	if err != nil {
		return err
	}

	for index, title := range titles {
		position := int32(index + 1)
		item, err := s.catalogue.SaveScanned(ctx, items.Item{
			ParentID:          &first.ID,
			Kind:              itemmodel.KindEpisode,
			Key:               fmt.Sprintf("episode:%s:1:%d", slugify(name), position),
			Name:              title,
			SortName:          strings.ToLower(title),
			IndexNumber:       &position,
			ParentIndexNumber: &number,
			DateModified:      &modified,
		})
		if err != nil {
			return err
		}

		if err := s.episode(ctx, item.ID, title); err != nil {
			return err
		}
		if err := member(ctx, s.client, s.libraryID, s.sourceID, item.ID); err != nil {
			return err
		}
		if err := aired(ctx, s.catalogue, item.ID, released[title]); err != nil {
			return err
		}
		s.watching[title] = item.ID
	}

	for _, id := range []uuid.UUID{show.ID, first.ID} {
		if err := member(ctx, s.client, s.libraryID, s.sourceID, id); err != nil {
			return err
		}
	}

	return nil
}

func (s shelf) episode(ctx context.Context, itemID uuid.UUID, title string) error {
	if s.media == "" {
		return file(ctx, s.catalogue, s.sourceID, itemID, title)
	}

	modified := time.Now()
	source, err := s.catalogue.SaveSource(ctx, items.MediaSource{
		SourceID:     s.sourceID,
		ItemID:       itemID,
		Path:         filepath.Join(s.media, slugify(title)+".mkv"),
		Name:         title,
		DateModified: &modified,
	})
	if err != nil {
		return err
	}

	return probe.New(s.catalogue, ffmpeg.New(), reports.New(s.client, nil)).ProbeSource(ctx, source.ID)
}

func playableMovie(ctx context.Context, client *store.Client, catalogue *items.Service, libraryID, sourceID uuid.UUID, name, key, path string) error {
	if path == "" {
		return fmt.Errorf("no media file was handed over for %q", name)
	}

	modified := time.Now()
	title, copied := int64(downloaderTitle), int64(downloaderFile)
	item, err := catalogue.SaveScanned(ctx, items.Item{
		Kind:         itemmodel.KindMovie,
		Key:          key,
		Name:         name,
		SortName:     strings.ToLower(name),
		DateModified: &modified,
	})
	if err != nil {
		return err
	}

	source, err := catalogue.SaveSource(ctx, items.MediaSource{
		SourceID:       sourceID,
		ItemID:         item.ID,
		Path:           path,
		Name:           name,
		DateModified:   &modified,
		ExternalID:     &title,
		ExternalFileID: &copied,
	})
	if err != nil {
		return err
	}

	if err := probe.New(catalogue, ffmpeg.New(), reports.New(client, nil)).ProbeSource(ctx, source.ID); err != nil {
		return err
	}

	return member(ctx, client, libraryID, sourceID, item.ID)
}

func aired(ctx context.Context, catalogue *items.Service, itemID uuid.UUID, year int) error {
	premiere := time.Date(year, time.March, 1, 0, 0, 0, 0, time.UTC)
	_, err := catalogue.UpdateMetadata(ctx, itemID, items.Metadata{PremiereDate: &premiere})

	return err
}

func member(ctx context.Context, client *store.Client, libraryID, sourceID, itemID uuid.UUID) error {
	return client.LibraryItem.Create().
		SetLibraryID(libraryID).
		SetSourceID(sourceID).
		SetItemID(itemID).
		Exec(ctx)
}

func file(ctx context.Context, catalogue *items.Service, sourceID, itemID uuid.UUID, name string) error {
	modified := time.Now()
	_, err := catalogue.SaveSource(ctx, items.MediaSource{
		SourceID:     sourceID,
		ItemID:       itemID,
		Path:         "/fixtures/" + name + ".mkv",
		Name:         name,
		DateModified: &modified,
	})

	return err
}

func poster(ctx context.Context, catalogue *items.Service, itemID uuid.UUID, name string) error {
	return artwork(ctx, catalogue, itemID, imagemodel.KindPrimary, slugify(name)+".png", image.Rect(0, 0, 200, 300), posterShades[name])
}

func backdrop(ctx context.Context, catalogue *items.Service, itemID uuid.UUID, name string) error {
	shade, ok := backdropShades[name]
	if !ok {
		return nil
	}

	return artwork(ctx, catalogue, itemID, imagemodel.KindBackdrop, slugify(name)+"-backdrop.png", image.Rect(0, 0, 640, 360), shade)
}

func artwork(ctx context.Context, catalogue *items.Service, itemID uuid.UUID, kind imagemodel.Kind, file string, bounds image.Rectangle, shade color.RGBA) error {
	directory, base := os.Getenv("POSTER_DIR"), os.Getenv("POSTER_URL")
	if directory == "" || base == "" {
		return nil
	}

	if err := paint(filepath.Join(directory, file), bounds, shade); err != nil {
		return err
	}

	url := base + "/" + file
	digest := sha1.Sum([]byte(url))

	return catalogue.SaveImage(ctx, itemID, items.Image{
		Kind: kind,
		URL:  url,
		Tag:  hex.EncodeToString(digest[:]),
	})
}

func paintUnowned() error {
	directory := os.Getenv("UNOWNED_POSTER_DIR")
	if directory == "" {
		return nil
	}

	for name, shade := range unownedShades {
		if err := paint(filepath.Join(directory, slugify(name)+".png"), image.Rect(0, 0, 200, 300), shade); err != nil {
			return err
		}
	}

	return nil
}

func paint(path string, bounds image.Rectangle, shade color.RGBA) error {
	picture := image.NewRGBA(bounds)
	for x := range bounds.Dx() {
		for y := range bounds.Dy() {
			picture.SetRGBA(x, y, shade)
		}
	}

	written, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := png.Encode(written, picture); err != nil {
		_ = written.Close()

		return err
	}

	return written.Close()
}

func slugify(name string) string {
	var slug strings.Builder
	separated := false
	for _, letter := range strings.ToLower(name) {
		if !unicode.IsLetter(letter) && !unicode.IsDigit(letter) {
			separated = slug.Len() > 0
			continue
		}
		if separated {
			slug.WriteByte('-')
			separated = false
		}
		slug.WriteRune(letter)
	}

	return slug.String()
}
