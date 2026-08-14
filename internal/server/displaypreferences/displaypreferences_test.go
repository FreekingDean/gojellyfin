package displaypreferences

import (
	"context"
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/auth"
	"github.com/FreekingDean/gojellyfin/internal/displaypreferences"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
	"github.com/FreekingDean/gojellyfin/internal/sessions"
	"github.com/FreekingDean/gojellyfin/internal/store"
	devicemodal "github.com/FreekingDean/gojellyfin/internal/store/device"
	displaypreferencesmodal "github.com/FreekingDean/gojellyfin/internal/store/displaypreferences"
	itemmodal "github.com/FreekingDean/gojellyfin/internal/store/item"
	usermodal "github.com/FreekingDean/gojellyfin/internal/store/user"
	"github.com/FreekingDean/gojellyfin/internal/users"
)

type fixture struct {
	server *Server
	client *store.Client
	ctx    context.Context
	userID uuid.UUID
	itemID uuid.UUID
}

func connect(t *testing.T) *store.Client {
	t.Helper()

	connection, err := store.NewStore()
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

	return connection.Client()
}

func newFixture(t *testing.T) *fixture {
	t.Helper()

	client := connect(t)
	ctx := context.Background()
	name := t.Name() + "-" + uuid.NewString()

	user, err := users.New(client).CreateUser(ctx, name, "hash", false)
	if err != nil {
		t.Fatalf("failed to create the user: %v", err)
	}

	sessionService := sessions.New(client)
	token := uuid.NewString()
	if _, err := sessionService.Create(ctx, user.ID, token, sessions.DeviceInfo{ID: name, Name: name}); err != nil {
		t.Fatalf("failed to create the session: %v", err)
	}

	authenticated, err := auth.New(sessionService).Authenticate(ctx, token)
	if err != nil {
		t.Fatalf("failed to authenticate: %v", err)
	}

	library, err := client.Library.Create().SetName(name).Save(ctx)
	if err != nil {
		t.Fatalf("failed to create the library: %v", err)
	}

	item, err := client.Item.Create().
		SetLibraryID(library.ID).
		SetKind(itemmodal.KindMovie).
		SetName(name).
		SetSortName(name).
		SetKey("test:" + name).
		Save(ctx)
	if err != nil {
		t.Fatalf("failed to create the item: %v", err)
	}

	t.Cleanup(func() {
		ctx := context.Background()
		if _, err := client.DisplayPreferences.Delete().
			Where(displaypreferencesmodal.HasUserWith(usermodal.ID(user.ID))).
			Exec(ctx); err != nil {
			t.Errorf("failed to delete the display preferences: %v", err)
		}
		if err := sessionService.DeleteByToken(ctx, token); err != nil {
			t.Errorf("failed to delete the session: %v", err)
		}
		if _, err := client.Device.Delete().Where(devicemodal.ClientID(name)).Exec(ctx); err != nil {
			t.Errorf("failed to delete the device: %v", err)
		}
		if err := users.New(client).DeleteUser(ctx, user.ID); err != nil {
			t.Errorf("failed to delete the user: %v", err)
		}
		if err := client.Item.DeleteOne(item).Exec(ctx); err != nil {
			t.Errorf("failed to delete the item: %v", err)
		}
		if err := client.Library.DeleteOne(library).Exec(ctx); err != nil {
			t.Errorf("failed to delete the library: %v", err)
		}
	})

	return &fixture{
		server: New(displaypreferences.New(client)),
		client: client,
		ctx:    authenticated,
		userID: user.ID,
		itemID: item.ID,
	}
}

func (f *fixture) save(t *testing.T, id, client string, body api.DisplayPreferencesDto) {
	t.Helper()

	response, err := f.server.UpdateDisplayPreferences(f.ctx, api.UpdateDisplayPreferencesRequestObject{
		DisplayPreferencesId: id,
		Params:               api.UpdateDisplayPreferencesParams{Client: client},
		JSONBody:             &body,
	})
	if err != nil {
		t.Fatalf("failed to update %q: %v", id, err)
	}
	if _, ok := response.(api.UpdateDisplayPreferences204Response); !ok {
		t.Fatalf("update response = %T, want 204", response)
	}
}

func (f *fixture) load(t *testing.T, id, client string) api.DisplayPreferencesDto {
	t.Helper()

	response, err := f.server.GetDisplayPreferences(f.ctx, api.GetDisplayPreferencesRequestObject{
		DisplayPreferencesId: id,
		Params:               api.GetDisplayPreferencesParams{Client: client},
	})
	if err != nil {
		t.Fatalf("failed to get %q: %v", id, err)
	}

	dto, ok := response.(api.GetDisplayPreferences200JSONResponse)
	if !ok {
		t.Fatalf("get response = %T, want 200", response)
	}

	return api.DisplayPreferencesDto(dto)
}

func TestDisplayPreferencesRoundTrip(t *testing.T) {
	fixture := newFixture(t)

	fixture.save(t, "usersettings", "emby", api.DisplayPreferencesDto{
		ViewType:          apiutil.Ptr("Poster"),
		SortBy:            apiutil.Ptr("DateCreated"),
		IndexBy:           apiutil.Ptr("Genre"),
		SortOrder:         apiutil.Ptr(api.SortOrder("Descending")),
		ScrollDirection:   apiutil.Ptr(api.ScrollDirection("Vertical")),
		RememberSorting:   apiutil.Ptr(true),
		ShowSidebar:       apiutil.Ptr(true),
		CustomPrefs:       &map[string]*string{"landing-movies": apiutil.Ptr("suggestions")},
		PrimaryImageWidth: apiutil.Ptr(int32(320)),
	})

	dto := fixture.load(t, "usersettings", "emby")

	if got := apiutil.Deref(dto.Id); got != "usersettings" {
		t.Errorf("Id = %q, want %q", got, "usersettings")
	}
	if got := apiutil.Deref(dto.Client); got != "emby" {
		t.Errorf("Client = %q, want %q", got, "emby")
	}
	if got := apiutil.Deref(dto.ViewType); got != "Poster" {
		t.Errorf("ViewType = %q, want %q", got, "Poster")
	}
	if got := apiutil.Deref(dto.SortBy); got != "DateCreated" {
		t.Errorf("SortBy = %q, want %q", got, "DateCreated")
	}
	if got := apiutil.Deref(dto.IndexBy); got != "Genre" {
		t.Errorf("IndexBy = %q, want %q", got, "Genre")
	}
	if got := apiutil.Deref(dto.SortOrder); got != api.SortOrder("Descending") {
		t.Errorf("SortOrder = %q, want %q", got, "Descending")
	}
	if got := apiutil.Deref(dto.ScrollDirection); got != api.ScrollDirection("Vertical") {
		t.Errorf("ScrollDirection = %q, want %q", got, "Vertical")
	}
	if got := apiutil.Deref(dto.RememberSorting); !got {
		t.Error("RememberSorting = false, want true")
	}
	if got := apiutil.Deref(dto.ShowSidebar); !got {
		t.Error("ShowSidebar = false, want true")
	}
	if got := apiutil.Deref(dto.PrimaryImageWidth); got != 320 {
		t.Errorf("PrimaryImageWidth = %d, want 320", got)
	}
	if got := apiutil.Deref(apiutil.Deref(dto.CustomPrefs)["landing-movies"]); got != "suggestions" {
		t.Errorf("CustomPrefs[landing-movies] = %q, want %q", got, "suggestions")
	}
}

func TestDisplayPreferencesKeying(t *testing.T) {
	fixture := newFixture(t)

	fixture.save(t, "usersettings", "emby", api.DisplayPreferencesDto{ViewType: apiutil.Ptr("Poster")})
	fixture.save(t, "usersettings", "android", api.DisplayPreferencesDto{ViewType: apiutil.Ptr("List")})
	fixture.save(t, fixture.itemID.String(), "emby", api.DisplayPreferencesDto{ViewType: apiutil.Ptr("Thumb")})

	for _, want := range []struct {
		id       string
		client   string
		viewType string
	}{
		{id: "usersettings", client: "emby", viewType: "Poster"},
		{id: "usersettings", client: "android", viewType: "List"},
		{id: fixture.itemID.String(), client: "emby", viewType: "Thumb"},
	} {
		if got := apiutil.Deref(fixture.load(t, want.id, want.client).ViewType); got != want.viewType {
			t.Errorf("ViewType for %q on %q = %q, want %q", want.id, want.client, got, want.viewType)
		}
	}
}

func TestDisplayPreferencesConcurrentGets(t *testing.T) {
	fixture := newFixture(t)

	const callers = 8
	var group sync.WaitGroup
	start := make(chan struct{})
	failures := make(chan error, callers)

	for i := 0; i < callers; i++ {
		caller := New(displaypreferences.New(connect(t)))
		group.Add(1)
		go func() {
			defer group.Done()
			<-start
			_, err := caller.GetDisplayPreferences(fixture.ctx, api.GetDisplayPreferencesRequestObject{
				DisplayPreferencesId: "usersettings",
				Params:               api.GetDisplayPreferencesParams{Client: "emby"},
			})
			failures <- err
		}()
	}

	close(start)
	group.Wait()
	close(failures)

	for err := range failures {
		if err != nil {
			t.Fatalf("concurrent get failed: %v", err)
		}
	}

	count, err := fixture.client.DisplayPreferences.Query().
		Where(displaypreferencesmodal.HasUserWith(usermodal.ID(fixture.userID))).
		Count(context.Background())
	if err != nil {
		t.Fatalf("failed to count the display preferences: %v", err)
	}
	if count != 1 {
		t.Errorf("rows = %d, want 1", count)
	}
}

func TestDisplayPreferencesDefaults(t *testing.T) {
	fixture := newFixture(t)

	dto := fixture.load(t, "usersettings", "emby")

	if got := apiutil.Deref(dto.SortBy); got != "SortName" {
		t.Errorf("SortBy = %q, want %q", got, "SortName")
	}
	if got := apiutil.Deref(dto.SortOrder); got != api.SortOrder("Ascending") {
		t.Errorf("SortOrder = %q, want %q", got, "Ascending")
	}
	if got := apiutil.Deref(dto.ScrollDirection); got != api.ScrollDirection("Horizontal") {
		t.Errorf("ScrollDirection = %q, want %q", got, "Horizontal")
	}
	if got := apiutil.Deref(dto.ShowBackdrop); !got {
		t.Error("ShowBackdrop = false, want true")
	}
}
