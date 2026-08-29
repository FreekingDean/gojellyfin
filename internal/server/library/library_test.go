package library

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/activity"
	"github.com/FreekingDean/gojellyfin/internal/auth"
	"github.com/FreekingDean/gojellyfin/internal/env"
	"github.com/FreekingDean/gojellyfin/internal/filesystem"
	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/jobs"
	"github.com/FreekingDean/gojellyfin/internal/libraries"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/sessions"
	"github.com/FreekingDean/gojellyfin/internal/store"
	devicemodal "github.com/FreekingDean/gojellyfin/internal/store/device"
	imagemodal "github.com/FreekingDean/gojellyfin/internal/store/image"
	itemmodal "github.com/FreekingDean/gojellyfin/internal/store/item"
	librarymodal "github.com/FreekingDean/gojellyfin/internal/store/library"
	optionsmodal "github.com/FreekingDean/gojellyfin/internal/store/libraryoptions"
	sessionmodal "github.com/FreekingDean/gojellyfin/internal/store/session"
	usermodal "github.com/FreekingDean/gojellyfin/internal/store/user"
	configurationmodal "github.com/FreekingDean/gojellyfin/internal/store/userconfiguration"
	datamodal "github.com/FreekingDean/gojellyfin/internal/store/useritemdata"
	policymodal "github.com/FreekingDean/gojellyfin/internal/store/userpolicy"
	"github.com/FreekingDean/gojellyfin/internal/users"
)

type fixture struct {
	server    *Server
	client    *store.Client
	libraryID uuid.UUID
	prefix    string
}

type seed struct {
	kind     items.Kind
	name     string
	parentID *uuid.UUID
	path     string
}

func newFixture(t *testing.T) *fixture {
	t.Helper()

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

	ctx := context.Background()
	client := connection.Client()
	prefix := t.Name() + "-" + uuid.NewString()

	library, err := libraries.New(client).CreateLibrary(ctx, prefix, librarymodal.CollectionTypeMovies, []string{"/media/" + prefix})
	if err != nil {
		t.Fatalf("failed to create the library: %v", err)
	}

	t.Cleanup(func() {
		if _, err := client.Image.Delete().
			Where(imagemodal.HasItemWith(itemmodal.LibraryID(library.ID))).
			Exec(ctx); err != nil {
			t.Errorf("failed to delete the images: %v", err)
		}
		if _, err := client.UserItemData.Delete().
			Where(datamodal.HasItemWith(itemmodal.LibraryID(library.ID))).
			Exec(ctx); err != nil {
			t.Errorf("failed to delete the user item data: %v", err)
		}
		if _, err := client.Item.Delete().Where(itemmodal.LibraryID(library.ID)).Exec(ctx); err != nil {
			t.Errorf("failed to delete the items: %v", err)
		}
		if _, err := client.Session.Delete().
			Where(sessionmodal.HasUserWith(usermodal.UsernameHasPrefix(prefix))).
			Exec(ctx); err != nil {
			t.Errorf("failed to delete the sessions: %v", err)
		}
		if _, err := client.Device.Delete().Where(devicemodal.ClientIDHasPrefix(prefix)).Exec(ctx); err != nil {
			t.Errorf("failed to delete the devices: %v", err)
		}
		if _, err := client.UserPolicy.Delete().
			Where(policymodal.HasUserWith(usermodal.UsernameHasPrefix(prefix))).
			Exec(ctx); err != nil {
			t.Errorf("failed to delete the user policies: %v", err)
		}
		if _, err := client.UserConfiguration.Delete().
			Where(configurationmodal.HasUserWith(usermodal.UsernameHasPrefix(prefix))).
			Exec(ctx); err != nil {
			t.Errorf("failed to delete the user configurations: %v", err)
		}
		if _, err := client.User.Delete().Where(usermodal.UsernameHasPrefix(prefix)).Exec(ctx); err != nil {
			t.Errorf("failed to delete the users: %v", err)
		}
		if _, err := client.LibraryOptions.Delete().
			Where(optionsmodal.HasLibraryWith(librarymodal.NameHasPrefix(prefix))).
			Exec(ctx); err != nil {
			t.Errorf("failed to delete the library options: %v", err)
		}
		if _, err := client.Library.Delete().Where(librarymodal.NameHasPrefix(prefix)).Exec(ctx); err != nil {
			t.Errorf("failed to delete the libraries: %v", err)
		}
		if err := connection.Stop(); err != nil {
			t.Errorf("failed to close the database: %v", err)
		}
	})

	server := New(items.New(client), libraries.New(client), users.New(client), filesystem.New(env.Config{MediaDirectories: []string{filesystem.Root}}), jobs.NewService(disconnected(t), jobs.NewRegistry()))

	return &fixture{server: server, client: client, libraryID: library.ID, prefix: prefix}
}

func (f *fixture) add(t *testing.T, item seed) uuid.UUID {
	t.Helper()

	record, err := f.client.Item.Create().
		SetLibraryID(f.libraryID).
		SetKind(item.kind).
		SetKey(f.prefix + ":" + item.name).
		SetName(item.name).
		SetSortName(item.name).
		SetNillableParentID(item.parentID).
		Save(context.Background())
	if err != nil {
		t.Fatalf("failed to create %q: %v", item.name, err)
	}

	if item.path != "" {
		err := f.client.MediaSource.Create().
			SetItemID(record.ID).
			SetLibraryID(f.libraryID).
			SetName(filepath.Base(item.path)).
			SetPath(item.path).
			Exec(context.Background())
		if err != nil {
			t.Fatalf("failed to create the source of %q: %v", item.name, err)
		}
	}

	return record.ID
}

func (f *fixture) signIn(t *testing.T, name string, administrator bool) context.Context {
	t.Helper()

	ctx := context.Background()
	user, err := users.New(f.client).CreateUser(ctx, f.prefix+name, "hash", administrator)
	if err != nil {
		t.Fatalf("failed to create the user: %v", err)
	}

	device, err := f.client.Device.Create().
		SetClientID(f.prefix + name).
		SetName("Test").
		SetAppName("Jellyfin Web").
		SetAppVersion("10.10.0").
		SetSupportsMediaControl(false).
		SetSupportsPersistentIdentifier(false).
		Save(ctx)
	if err != nil {
		t.Fatalf("failed to create the device: %v", err)
	}

	token := uuid.NewString()
	if _, err := f.client.Session.Create().
		SetUserID(user.ID).
		SetDeviceID(device.ID).
		SetAccessToken(token).
		Save(ctx); err != nil {
		t.Fatalf("failed to create the session: %v", err)
	}

	authenticated, err := auth.New(sessions.New(f.client, activity.New(f.client))).Authenticate(ctx, token)
	if err != nil {
		t.Fatalf("failed to authenticate: %v", err)
	}

	return authenticated
}

func names(dtos []api.BaseItemDto) []string {
	found := make([]string, 0, len(dtos))
	for _, dto := range dtos {
		found = append(found, *dto.Name)
	}

	return found
}

func TestServer_GetPhysicalPaths(t *testing.T) {
	fixture := newFixture(t)
	ctx := context.Background()

	shared := "/media/" + fixture.prefix
	_, err := libraries.New(fixture.client).
		CreateLibrary(ctx, fixture.prefix+"-second", librarymodal.CollectionTypeTvshows, []string{shared, shared + "-shows"})
	if err != nil {
		t.Fatalf("failed to create the second library: %v", err)
	}

	response, err := fixture.server.GetPhysicalPaths(ctx, api.GetPhysicalPathsRequestObject{})
	if err != nil {
		t.Fatalf("failed to get the physical paths: %v", err)
	}

	paths, ok := response.(api.GetPhysicalPaths200JSONResponse)
	if !ok {
		t.Fatalf("response = %T, want api.GetPhysicalPaths200JSONResponse", response)
	}

	mine := []string{}
	for _, path := range paths {
		if path == shared || path == shared+"-shows" {
			mine = append(mine, path)
		}
	}
	if want := []string{shared, shared + "-shows"}; !slices.Equal(mine, want) {
		t.Errorf("paths = %v, want %v once each", mine, want)
	}
}

func TestServer_GetAncestors(t *testing.T) {
	fixture := newFixture(t)
	ctx := context.Background()

	series := fixture.add(t, seed{kind: itemmodal.KindSeries, name: "Series"})
	season := fixture.add(t, seed{kind: itemmodal.KindSeason, name: "Season 1", parentID: &series})
	episode := fixture.add(t, seed{kind: itemmodal.KindEpisode, name: "S01E01", parentID: &season})

	t.Run("walks up to the library and the root folder", func(t *testing.T) {
		response, err := fixture.server.GetAncestors(ctx, api.GetAncestorsRequestObject{ItemId: episode})
		if err != nil {
			t.Fatalf("failed to get the ancestors: %v", err)
		}

		dtos, ok := response.(api.GetAncestors200JSONResponse)
		if !ok {
			t.Fatalf("response = %T, want api.GetAncestors200JSONResponse", response)
		}

		want := []string{"Season 1", "Series", fixture.prefix, "Media Folders"}
		if got := names(dtos); !slices.Equal(got, want) {
			t.Errorf("ancestors = %v, want %v", got, want)
		}
		if *dtos[0].Id != season || *dtos[1].Id != series {
			t.Errorf("ancestor ids = %v, %v, want %v, %v", *dtos[0].Id, *dtos[1].Id, season, series)
		}
		if *dtos[2].Type != api.BaseItemKindCollectionFolder {
			t.Errorf("library type = %q, want CollectionFolder", *dtos[2].Type)
		}
	})

	t.Run("reports an unknown item as missing", func(t *testing.T) {
		response, err := fixture.server.GetAncestors(ctx, api.GetAncestorsRequestObject{ItemId: uuid.New()})
		if err != nil {
			t.Fatalf("failed to get the ancestors: %v", err)
		}
		if _, ok := response.(api.GetAncestors404JSONResponse); !ok {
			t.Fatalf("response = %T, want api.GetAncestors404JSONResponse", response)
		}
	})
}

func TestServer_GetSimilarItems(t *testing.T) {
	fixture := newFixture(t)
	ctx := context.Background()

	movie := fixture.add(t, seed{kind: itemmodal.KindMovie, name: "Movie"})

	response, err := fixture.server.GetSimilarItems(ctx, api.GetSimilarItemsRequestObject{ItemId: movie})
	if err != nil {
		t.Fatalf("failed to get the similar items: %v", err)
	}

	result, ok := response.(api.GetSimilarItems200JSONResponse)
	if !ok {
		t.Fatalf("response = %T, want api.GetSimilarItems200JSONResponse", response)
	}
	if len(*result.Items) != 0 || *result.TotalRecordCount != 0 {
		t.Errorf("similar = %v, want an empty result", names(*result.Items))
	}
}

func TestServer_DeleteItem(t *testing.T) {
	fixture := newFixture(t)

	directory := t.TempDir()
	seriesPath := filepath.Join(directory, "Series")
	episodePath := filepath.Join(seriesPath, "S01E01.mkv")
	if err := os.MkdirAll(seriesPath, 0o755); err != nil {
		t.Fatalf("failed to create the series directory: %v", err)
	}
	if err := os.WriteFile(episodePath, []byte("media"), 0o600); err != nil {
		t.Fatalf("failed to create the episode: %v", err)
	}

	series := fixture.add(t, seed{kind: itemmodal.KindSeries, name: "Series"})
	episode := fixture.add(t, seed{kind: itemmodal.KindEpisode, name: "S01E01", parentID: &series, path: episodePath})

	t.Run("refuses a user without the deletion policy", func(t *testing.T) {
		ctx := fixture.signIn(t, "viewer", false)

		response, err := fixture.server.DeleteItem(ctx, api.DeleteItemRequestObject{ItemId: series})
		if err != nil {
			t.Fatalf("failed to delete the item: %v", err)
		}
		if _, ok := response.(api.DeleteItem403Response); !ok {
			t.Fatalf("response = %T, want api.DeleteItem403Response", response)
		}
		if _, err := os.Stat(episodePath); err != nil {
			t.Errorf("the episode was removed for a user that may not delete: %v", err)
		}
	})

	t.Run("refuses while the filesystem cannot remove media", func(t *testing.T) {
		ctx := fixture.signIn(t, "admin", true)

		response, err := fixture.server.DeleteItem(ctx, api.DeleteItemRequestObject{ItemId: series})
		if err != nil {
			t.Fatalf("failed to delete the item: %v", err)
		}
		if _, ok := response.(api.DeleteItem403Response); !ok {
			t.Fatalf("response = %T, want api.DeleteItem403Response", response)
		}

		if _, err := os.Stat(episodePath); err != nil {
			t.Errorf("the media was removed even though the delete was refused: %v", err)
		}

		remaining, err := fixture.client.Item.Query().
			Where(itemmodal.IDIn(series, episode)).
			Count(context.Background())
		if err != nil {
			t.Fatalf("failed to count the remaining items: %v", err)
		}
		if remaining != 2 {
			t.Errorf("remaining items = %d, want 2; a refused delete must not orphan rows", remaining)
		}
	})

	t.Run("an item that is already gone is already deleted", func(t *testing.T) {
		ctx := fixture.signIn(t, "second-admin", true)

		response, err := fixture.server.DeleteItem(ctx, api.DeleteItemRequestObject{ItemId: uuid.New()})
		if err != nil {
			t.Fatalf("failed to delete the item: %v", err)
		}
		if _, ok := response.(api.DeleteItem204Response); !ok {
			t.Fatalf("response = %T, want api.DeleteItem204Response", response)
		}
	})
}

func TestServer_GetDownload(t *testing.T) {
	fixture := newFixture(t)

	path := filepath.Join(t.TempDir(), "Movie.mkv")
	if err := os.WriteFile(path, []byte("media"), 0o600); err != nil {
		t.Fatalf("failed to create the movie: %v", err)
	}
	movie := fixture.add(t, seed{kind: itemmodal.KindMovie, name: "Movie", path: path})

	t.Run("streams the file back", func(t *testing.T) {
		ctx := fixture.signIn(t, "viewer", false)

		response, err := fixture.server.GetDownload(ctx, api.GetDownloadRequestObject{ItemId: movie})
		if err != nil {
			t.Fatalf("failed to download the item: %v", err)
		}

		download, ok := response.(api.GetDownload200VideoResponse)
		if !ok {
			t.Fatalf("response = %T, want api.GetDownload200VideoResponse", response)
		}

		body, err := io.ReadAll(download.Body)
		if err != nil {
			t.Fatalf("failed to read the body: %v", err)
		}
		if string(body) != "media" {
			t.Errorf("body = %q, want %q", body, "media")
		}
		if download.ContentLength != int64(len("media")) {
			t.Errorf("ContentLength = %d, want %d", download.ContentLength, len("media"))
		}
	})

	t.Run("refuses a user without the downloading policy", func(t *testing.T) {
		ctx := fixture.signIn(t, "restricted", false)
		err := users.New(fixture.client).UpdatePolicy(auth.UserID(ctx)).
			SetEnableContentDownloading(false).
			Exec(context.Background())
		if err != nil {
			t.Fatalf("failed to update the policy: %v", err)
		}

		response, err := fixture.server.GetDownload(ctx, api.GetDownloadRequestObject{ItemId: movie})
		if err != nil {
			t.Fatalf("failed to download the item: %v", err)
		}
		if _, ok := response.(api.GetDownload403Response); !ok {
			t.Fatalf("response = %T, want api.GetDownload403Response", response)
		}
	})

	t.Run("reports a missing file", func(t *testing.T) {
		ctx := fixture.signIn(t, "reader", false)
		gone := fixture.add(t, seed{kind: itemmodal.KindMovie, name: "Gone", path: filepath.Join(t.TempDir(), "gone.mkv")})

		response, err := fixture.server.GetFile(ctx, api.GetFileRequestObject{ItemId: gone})
		if err != nil {
			t.Fatalf("failed to get the file: %v", err)
		}
		if _, ok := response.(api.GetFile404JSONResponse); !ok {
			t.Fatalf("response = %T, want api.GetFile404JSONResponse", response)
		}
	})
}

func disconnected(t *testing.T) *jobs.Client {
	t.Helper()

	client, err := jobs.NewClient(env.Config{})
	if err != nil {
		t.Fatalf("failed to build the temporal client: %v", err)
	}

	return client
}
