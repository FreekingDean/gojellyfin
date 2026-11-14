package migration

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/FreekingDean/gojellyfin/internal/activity"
	"github.com/FreekingDean/gojellyfin/internal/auth"
	"github.com/FreekingDean/gojellyfin/internal/env"
	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/sessions"
	"github.com/FreekingDean/gojellyfin/internal/store"
	apikeymodel "github.com/FreekingDean/gojellyfin/internal/store/apikey"
	devicemodel "github.com/FreekingDean/gojellyfin/internal/store/device"
	itemmodel "github.com/FreekingDean/gojellyfin/internal/store/item"
	sourcemodel "github.com/FreekingDean/gojellyfin/internal/store/source"
	usermodel "github.com/FreekingDean/gojellyfin/internal/store/user"
	datamodel "github.com/FreekingDean/gojellyfin/internal/store/useritemdata"
	"github.com/FreekingDean/gojellyfin/scripts/jellyfinmigrate/migration/fixture"
)

const takenUsername = "carol"

var (
	aliceID = uuid.MustParse(fixture.AliceID)
	bobID   = uuid.MustParse(fixture.BobID)
	carolID = uuid.MustParse(fixture.CarolID)
)

type harness struct {
	client *store.Client
	titles map[string]uuid.UUID
}

func newHarness(t *testing.T) *harness {
	t.Helper()

	config, err := env.Load()
	require.NoError(t, err)
	db, err := store.NewStore(config)
	require.NoError(t, err)
	require.NoError(t, db.Start())
	t.Cleanup(func() { _ = db.Stop() })

	h := &harness{client: db.Client(), titles: map[string]uuid.UUID{}}
	h.clear(t)
	t.Cleanup(func() { h.clear(t) })

	ctx := context.Background()
	downloader, err := h.client.Source.Create().
		SetName(sourceName).
		SetURL("http://jellyfinmigrate.invalid").
		SetAPIKeyVariable("SOURCE_API_KEY_TEST").
		SetKind(sourcemodel.KindRadarr).
		Save(ctx)
	require.NoError(t, err)

	h.title(t, itemmodel.KindMovie, items.MovieKey(fixture.MovieTmdb), nil)
	h.title(t, itemmodel.KindMovie, items.MovieKey(fixture.ImdbOnlyTmdb), map[string]string{"Imdb": fixture.ImdbOnlyImdb})
	pathOnly := h.title(t, itemmodel.KindMovie, items.MovieKey(pathOnlyTmdb), nil)
	h.title(t, itemmodel.KindSeries, items.SeriesKey(fixture.SeriesTmdb), nil)
	h.title(t, itemmodel.KindSeason, items.SeasonKey(fixture.SeriesTmdb, 1), nil)
	h.title(t, itemmodel.KindEpisode, items.EpisodeKey(fixture.SeriesTmdb, 1, 2), nil)
	h.title(t, itemmodel.KindSeries, items.SeriesKey(fixture.ImdbSeriesTmdb), map[string]string{"Imdb": fixture.ImdbSeriesImdb})
	h.title(t, itemmodel.KindEpisode, items.EpisodeKey(fixture.ImdbSeriesTmdb, 2, 5), nil)

	err = h.client.ItemSource.Create().
		SetItemID(pathOnly).
		SetSourceID(downloader.ID).
		SetName("Path Only").
		SetPath(fixture.PathOnlyPath).
		Exec(ctx)
	require.NoError(t, err)

	_, err = h.client.User.Create().SetName(takenUsername).SetUsername(takenUsername).SetPasswordHash("").Save(ctx)
	require.NoError(t, err)

	return h
}

const (
	sourceName   = "jellyfinmigrate-test"
	pathOnlyTmdb = 990003
)

func (h *harness) title(t *testing.T, kind itemmodel.Kind, key string, providers map[string]string) uuid.UUID {
	t.Helper()

	create := h.client.Item.Create().SetKind(kind).SetKey(key).SetName(key)
	if providers != nil {
		create.SetProviderIds(providers)
	}
	item, err := create.Save(context.Background())
	require.NoError(t, err)
	h.titles[key] = item.ID

	return item.ID
}

func (h *harness) clear(t *testing.T) {
	t.Helper()
	ctx := context.Background()

	_, err := h.client.User.Delete().Where(usermodel.Or(
		usermodel.IDIn(aliceID, bobID, carolID),
		usermodel.UsernameEqualFold(takenUsername),
	)).Exec(ctx)
	require.NoError(t, err)
	_, err = h.client.Device.Delete().Where(devicemodel.ClientIDHasPrefix("device-")).Exec(ctx)
	require.NoError(t, err)
	_, err = h.client.ApiKey.Delete().Where(apikeymodel.AccessToken(fixture.APIKey)).Exec(ctx)
	require.NoError(t, err)
	_, err = h.client.Source.Delete().Where(sourcemodel.Name(sourceName)).Exec(ctx)
	require.NoError(t, err)
	_, err = h.client.Item.Delete().Where(itemmodel.Or(
		itemmodel.KeyHasSuffix(":tmdb:99000"),
		itemmodel.KeyContains(":tmdb:9900"),
	)).Exec(ctx)
	require.NoError(t, err)
}

func (h *harness) state(t *testing.T, user uuid.UUID, key string) *store.UserItemData {
	t.Helper()

	row, err := h.client.UserItemData.Query().
		Where(datamodel.UserID(user), datamodel.ItemID(h.titles[key])).
		Only(context.Background())
	require.NoError(t, err, key)

	return row
}

func read(t *testing.T, layout fixture.Layout) *Source {
	t.Helper()

	directory := t.TempDir()
	require.NoError(t, fixture.Write(directory, layout))
	source, err := Read(context.Background(), directory)
	require.NoError(t, err)

	return source
}

func TestRun(t *testing.T) {
	for _, layout := range []fixture.Layout{fixture.Legacy, fixture.Consolidated} {
		t.Run(string(layout), func(t *testing.T) {
			ctx := context.Background()
			h := newHarness(t)
			source := read(t, layout)
			signedIn := sessions.New(h.client, activity.New(h.client))

			t.Run("a dry run reports and writes nothing", func(t *testing.T) {
				report, err := Run(ctx, h.client, source, true)
				require.NoError(t, err)
				require.Equal(t, 3, report.SessionsCarried)
				require.Equal(t, 8, report.WatchedCarried)

				exists, err := h.client.User.Query().Where(usermodel.ID(aliceID)).Exist(ctx)
				require.NoError(t, err)
				require.False(t, exists)
			})

			report, err := Run(ctx, h.client, source, false)
			require.NoError(t, err)

			t.Run("reports what it carried and what it could not", func(t *testing.T) {
				require.Equal(t, fixture.ServerID, report.ServerID)
				require.Equal(t, []UserOutcome{
					{Username: fixture.CarolUsername, State: UserNameTaken},
					{Username: fixture.AliceUsername, State: UserCreated},
					{Username: "bob", State: UserCreated, SignInUnset: true, Restricted: true},
				}, report.Users)
				require.Equal(t, 3, report.SessionsCarried)
				require.Equal(t, 1, report.SessionsSkipped)
				require.Equal(t, 1, report.IntegrationsCarried)
				require.Equal(t, 8, report.WatchedCarried)
				require.Equal(t, 1, report.WatchedSkipped)
				require.Equal(t, 2, report.Orphaned)

				missing := report.Unmatched()
				require.Len(t, missing, 1)
				require.Equal(t, fixture.UnmatchedTitle, missing[0].Name)
				require.Equal(t, 1, missing[0].Users)
				require.Contains(t, report.String(), "SERVER_ID="+fixture.ServerID)
			})

			t.Run("a user keeps their id, their password and their policy", func(t *testing.T) {
				alice, err := h.client.User.Query().Where(usermodel.ID(aliceID)).WithPolicy().WithConfiguration().Only(ctx)
				require.NoError(t, err)
				require.Equal(t, fixture.AliceUsername, alice.Username)
				require.True(t, alice.Edges.Policy.IsAdministrator)
				require.True(t, alice.Edges.Policy.EnableContentDeletion)
				require.Equal(t, "Smart", string(alice.Edges.Configuration.SubtitleMode))
				require.Equal(t, "eng", alice.Edges.Configuration.AudioLanguagePreference)
				require.NotNil(t, alice.LastLoginAt)

				matches, err := auth.Verify(fixture.AlicePassword, alice.PasswordHash)
				require.NoError(t, err)
				require.True(t, matches)

				bob, err := h.client.User.Query().Where(usermodel.ID(bobID)).WithPolicy().Only(ctx)
				require.NoError(t, err)
				require.Empty(t, bob.PasswordHash)
				require.False(t, bob.Edges.Policy.IsAdministrator)
				require.False(t, bob.Edges.Policy.EnableAllFolders)
				require.False(t, bob.Edges.Policy.EnableContentDownloading)
				require.Equal(t, int32(8000000), bob.Edges.Policy.RemoteClientBitrateLimit)
				require.Equal(t, "JoinGroups", string(bob.Edges.Policy.SyncPlayAccess))
			})

			t.Run("a token Jellyfin issued signs in as the same user", func(t *testing.T) {
				for token, user := range map[string]uuid.UUID{
					fixture.AliceWebToken: aliceID,
					fixture.AliceTVToken:  aliceID,
					fixture.BobToken:      bobID,
				} {
					session, err := signedIn.ByToken(ctx, token)
					require.NoError(t, err)
					require.Equal(t, user, session.User.ID)
				}

				web, err := signedIn.ByToken(ctx, fixture.AliceWebToken)
				require.NoError(t, err)
				require.Equal(t, fixture.AliceWebDevice, web.Device.ClientID)
				require.Equal(t, "Jellyfin Web", web.Device.AppName)
			})

			t.Run("a signed out device and a skipped user's device stay behind", func(t *testing.T) {
				for _, token := range []string{fixture.AliceOldToken, fixture.CarolToken} {
					_, err := signedIn.ByToken(ctx, token)
					require.Error(t, err)
				}
			})

			t.Run("an api key is carried", func(t *testing.T) {
				key, err := h.client.ApiKey.Query().Where(apikeymodel.AccessToken(fixture.APIKey)).Only(ctx)
				require.NoError(t, err)
				require.Equal(t, "Homepage", key.AppName)
			})

			t.Run("watch state lands on the title its provider id names", func(t *testing.T) {
				movie := h.state(t, aliceID, items.MovieKey(fixture.MovieTmdb))
				require.True(t, movie.Played)
				require.Equal(t, int32(2), movie.PlayCount)
				require.Equal(t, time.Date(2024, 5, 1, 20, 15, 0, 0, time.UTC), movie.LastPlayedAt.UTC())

				series := h.state(t, aliceID, items.SeriesKey(fixture.SeriesTmdb))
				require.True(t, series.IsFavorite)
				require.InDelta(t, 8.5, *series.Rating, 0.001)

				require.True(t, h.state(t, aliceID, items.SeasonKey(fixture.SeriesTmdb, 1)).Played)
				require.Equal(t, int32(3), h.state(t, aliceID, items.EpisodeKey(fixture.SeriesTmdb, 1, 2)).PlayCount)
				require.Equal(t, int64(500000000), h.state(t, bobID, items.MovieKey(fixture.MovieTmdb)).PlaybackPositionTicks)
			})

			t.Run("a title with no tmdb id is found by another provider's", func(t *testing.T) {
				require.True(t, h.state(t, aliceID, items.MovieKey(fixture.ImdbOnlyTmdb)).IsFavorite)
				episode := h.state(t, aliceID, items.EpisodeKey(fixture.ImdbSeriesTmdb, 2, 5))
				require.Equal(t, int64(fixture.EpisodePosition), episode.PlaybackPositionTicks)
			})

			t.Run("a title with no provider id is found by its path", func(t *testing.T) {
				require.Equal(t, int64(fixture.ResumePosition), h.state(t, aliceID, items.MovieKey(pathOnlyTmdb)).PlaybackPositionTicks)
			})

			t.Run("a second run carries nothing twice", func(t *testing.T) {
				again, err := Run(ctx, h.client, source, false)
				require.NoError(t, err)
				require.Equal(t, UserAlreadyHere, again.Users[1].State)
				require.Equal(t, UserAlreadyHere, again.Users[2].State)
				require.Zero(t, again.SessionsCarried)
				require.Equal(t, 3, again.SessionsAlreadyHere)
				require.Zero(t, again.IntegrationsCarried)
				require.Zero(t, again.WatchedCarried)
				require.Zero(t, again.WatchedMerged)
				require.Equal(t, 8, again.WatchedAlreadyHere)
			})

			t.Run("a later run keeps whichever answer is further along", func(t *testing.T) {
				key := items.MovieKey(fixture.MovieTmdb)
				err := h.client.UserItemData.UpdateOne(h.state(t, bobID, key)).
					SetPlayed(true).
					SetPlayCount(4).
					SetPlaybackPositionTicks(0).
					Exec(ctx)
				require.NoError(t, err)

				again, err := Run(ctx, h.client, source, false)
				require.NoError(t, err)
				require.Equal(t, 1, again.WatchedMerged)

				merged := h.state(t, bobID, key)
				require.True(t, merged.Played)
				require.Equal(t, int32(4), merged.PlayCount)
				require.Equal(t, int64(500000000), merged.PlaybackPositionTicks)
			})
		})
	}
}

func TestRead_RefusesADirectoryWithNoDatabase(t *testing.T) {
	_, err := Read(context.Background(), t.TempDir())
	require.ErrorContains(t, err, "jellyfin.db")
}
