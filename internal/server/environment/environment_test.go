package environment

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/auth"
	"github.com/FreekingDean/gojellyfin/internal/env"
	"github.com/FreekingDean/gojellyfin/internal/filesystem"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
	"github.com/FreekingDean/gojellyfin/internal/sessions"
	"github.com/FreekingDean/gojellyfin/internal/store"
	devicemodal "github.com/FreekingDean/gojellyfin/internal/store/device"
	sessionmodal "github.com/FreekingDean/gojellyfin/internal/store/session"
	usermodal "github.com/FreekingDean/gojellyfin/internal/store/user"
	configurationmodal "github.com/FreekingDean/gojellyfin/internal/store/userconfiguration"
	policymodal "github.com/FreekingDean/gojellyfin/internal/store/userpolicy"
	"github.com/FreekingDean/gojellyfin/internal/users"
)

type fixture struct {
	server *Server
	client *store.Client
	prefix string
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
		t.Fatalf("failed to reach the database: %v", err)
	}

	client := connection.Client()
	prefix := t.Name() + "-" + uuid.NewString() + "-"

	t.Cleanup(func() {
		ctx := context.Background()
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
			t.Errorf("failed to delete the policies: %v", err)
		}
		if _, err := client.UserConfiguration.Delete().
			Where(configurationmodal.HasUserWith(usermodal.UsernameHasPrefix(prefix))).
			Exec(ctx); err != nil {
			t.Errorf("failed to delete the configurations: %v", err)
		}
		if _, err := client.User.Delete().Where(usermodal.UsernameHasPrefix(prefix)).Exec(ctx); err != nil {
			t.Errorf("failed to delete the users: %v", err)
		}
		if err := connection.Stop(); err != nil {
			t.Errorf("failed to close the database: %v", err)
		}
	})

	return &fixture{server: New(filesystem.New(), users.New(client)), client: client, prefix: prefix}
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

	authenticated, err := auth.New(sessions.New(f.client)).Authenticate(ctx, token)
	if err != nil {
		t.Fatalf("failed to authenticate: %v", err)
	}

	return authenticated
}

func TestServer_GetDirectoryContents(t *testing.T) {
	fixture := newFixture(t)
	ctx := fixture.signIn(t, "admin", true)

	media := t.TempDir()
	movies := filepath.Join(media, "movies")
	for _, name := range []string{"movies", "music", "shows"} {
		if err := os.Mkdir(filepath.Join(media, name), 0o700); err != nil {
			t.Fatalf("failed to create %q: %v", name, err)
		}
	}
	if err := os.WriteFile(filepath.Join(movies, "Sintel (2010).mkv"), []byte("1"), 0o600); err != nil {
		t.Fatalf("failed to write the media file: %v", err)
	}

	for _, tc := range []struct {
		name      string
		path      string
		files     bool
		dirs      bool
		wantNames []string
		wantPaths []string
	}{
		{
			name: "directories only", path: media, dirs: true,
			wantNames: []string{"movies", "music", "shows"},
			wantPaths: []string{filepath.Join(media, "movies"), filepath.Join(media, "music"), filepath.Join(media, "shows")},
		},
		{name: "files excluded", path: movies, dirs: true},
		{
			name: "files only", path: movies, files: true,
			wantNames: []string{"Sintel (2010).mkv"},
			wantPaths: []string{filepath.Join(movies, "Sintel (2010).mkv")},
		},
		{
			name: "both", path: movies, files: true, dirs: true,
			wantNames: []string{"Sintel (2010).mkv"},
			wantPaths: []string{filepath.Join(movies, "Sintel (2010).mkv")},
		},
		{name: "neither", path: media},
	} {
		t.Run(tc.name, func(t *testing.T) {
			response, err := fixture.server.GetDirectoryContents(ctx, api.GetDirectoryContentsRequestObject{
				Params: api.GetDirectoryContentsParams{
					Path:               tc.path,
					IncludeFiles:       apiutil.Ptr(tc.files),
					IncludeDirectories: apiutil.Ptr(tc.dirs),
				},
			})
			if err != nil {
				t.Fatal(err)
			}

			contents := response.(api.GetDirectoryContents200JSONResponse)
			if len(contents) != len(tc.wantNames) {
				t.Fatalf("got %d entries, want %d", len(contents), len(tc.wantNames))
			}
			for i, want := range tc.wantNames {
				if got := apiutil.Deref(contents[i].Name); got != want {
					t.Errorf("entry %d: got name %q, want %q", i, got, want)
				}
				if got := apiutil.Deref(contents[i].Path); got != tc.wantPaths[i] {
					t.Errorf("entry %d: got path %q, want %q", i, got, tc.wantPaths[i])
				}
			}
		})
	}
	t.Run("a missing path is not found", func(t *testing.T) {
		fixture := newFixture(t)
		ctx := fixture.signIn(t, "admin", true)

		_, err := fixture.server.GetDirectoryContents(ctx, api.GetDirectoryContentsRequestObject{
			Params: api.GetDirectoryContentsParams{Path: filepath.Join(t.TempDir(), "nope")},
		})
		if !errors.Is(err, filesystem.ErrNotFound) {
			t.Fatalf("got %v, want ErrNotFound", err)
		}
	})

}

func TestServer_GetDrives(t *testing.T) {
	fixture := newFixture(t)
	ctx := fixture.signIn(t, "admin", true)

	response, err := fixture.server.GetDrives(ctx, api.GetDrivesRequestObject{})
	if err != nil {
		t.Fatal(err)
	}

	drives := response.(api.GetDrives200JSONResponse)
	if len(drives) != 1 {
		t.Fatalf("got %d drives, want 1", len(drives))
	}
	if got := apiutil.Deref(drives[0].Path); got != filesystem.Root {
		t.Errorf("got path %q, want %q", got, filesystem.Root)
	}
	if got := apiutil.Deref(drives[0].Type); got != api.FileSystemEntryTypeDirectory {
		t.Errorf("got type %q, want %q", got, api.FileSystemEntryTypeDirectory)
	}
}

func TestServer_GetParentPath(t *testing.T) {
	fixture := newFixture(t)
	ctx := fixture.signIn(t, "admin", true)

	for path, want := range map[string]string{
		"/media/movies": "/media",
		"/media/":       "/",
		"/media":        "/",
		"/":             "/",
		"":              "/",
	} {
		response, err := fixture.server.GetParentPath(ctx, api.GetParentPathRequestObject{
			Params: api.GetParentPathParams{Path: path},
		})
		if err != nil {
			t.Fatal(err)
		}
		if got := string(response.(api.GetParentPath200JSONResponse)); got != want {
			t.Errorf("%q: got %q, want %q", path, got, want)
		}
	}
}

func TestServer_GetDefaultDirectoryBrowser(t *testing.T) {
	fixture := newFixture(t)
	ctx := fixture.signIn(t, "admin", true)

	response, err := fixture.server.GetDefaultDirectoryBrowser(ctx, api.GetDefaultDirectoryBrowserRequestObject{})
	if err != nil {
		t.Fatal(err)
	}
	if got := apiutil.Deref(api.DefaultDirectoryBrowserInfoDto(response.(api.GetDefaultDirectoryBrowser200JSONResponse)).Path); got != filesystem.Root {
		t.Errorf("got %q, want %q", got, filesystem.Root)
	}
}

func TestServer(t *testing.T) {
	fixture := newFixture(t)
	ctx := fixture.signIn(t, "viewer", false)
	directory := t.TempDir()

	contents, err := fixture.server.GetDirectoryContents(ctx, api.GetDirectoryContentsRequestObject{
		Params: api.GetDirectoryContentsParams{Path: directory, IncludeDirectories: apiutil.Ptr(true)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := contents.(api.GetDirectoryContents403Response); !ok {
		t.Errorf("GetDirectoryContents = %T, want a 403", contents)
	}

	drives, err := fixture.server.GetDrives(ctx, api.GetDrivesRequestObject{})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := drives.(api.GetDrives403Response); !ok {
		t.Errorf("GetDrives = %T, want a 403", drives)
	}

	parent, err := fixture.server.GetParentPath(ctx, api.GetParentPathRequestObject{
		Params: api.GetParentPathParams{Path: directory},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := parent.(api.GetParentPath403Response); !ok {
		t.Errorf("GetParentPath = %T, want a 403", parent)
	}

	browser, err := fixture.server.GetDefaultDirectoryBrowser(ctx, api.GetDefaultDirectoryBrowserRequestObject{})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := browser.(api.GetDefaultDirectoryBrowser403Response); !ok {
		t.Errorf("GetDefaultDirectoryBrowser = %T, want a 403", browser)
	}

	validated, err := fixture.server.ValidatePath(ctx, api.ValidatePathRequestObject{
		JSONBody: &api.ValidatePathDto{Path: apiutil.Ptr(directory)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := validated.(api.ValidatePath403Response); !ok {
		t.Errorf("ValidatePath = %T, want a 403", validated)
	}
}
