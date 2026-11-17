package libraries

import (
	"context"
	"slices"
	"testing"

	"github.com/FreekingDean/gojellyfin/internal/storetest"

	"github.com/google/uuid"

	librarymodel "github.com/FreekingDean/gojellyfin/internal/store/library"
	sourcemodel "github.com/FreekingDean/gojellyfin/internal/store/source"
)

func TestService_EnsureFixed(t *testing.T) {
	connection := storetest.Open(t)
	client := connection.Client()
	ctx := context.Background()
	t.Cleanup(func() {
		if _, err := client.Library.Delete().Where(librarymodel.IDIn(Movies.ID, Shows.ID, Music.ID)).Exec(context.Background()); err != nil {
			t.Errorf("failed to delete the fixed libraries: %v", err)
		}
	})

	service := New(client)
	for range 2 {
		if err := service.EnsureFixed(ctx); err != nil {
			t.Fatalf("EnsureFixed = %v", err)
		}
	}

	for _, fixed := range []Fixed{Movies, Shows, Music} {
		found, err := client.Library.Query().
			Where(librarymodel.CollectionTypeEQ(fixed.CollectionType)).
			WithOptions().
			All(ctx)
		if err != nil {
			t.Fatalf("failed to read the libraries: %v", err)
		}
		if len(found) == 0 {
			t.Fatalf("no %s library after EnsureFixed", fixed.CollectionType)
		}
		for _, library := range found {
			if library.ID == fixed.ID && library.Edges.Options == nil {
				t.Errorf("%s has no options", fixed.Name)
			}
		}
	}
	shows := serviceEnsureFixedShows(t, service, ctx)
	if shows() {
		t.Error("Music is shown with no Lidarr feeding it")
	}

	lidarr, err := client.Source.Create().
		SetName("lidarr-" + uuid.NewString()).
		SetURL("http://" + uuid.NewString() + ".invalid").
		SetAPIKeyVariable("SOURCE_API_KEY_TEST").
		SetKind(sourcemodel.KindLidarr).
		Save(ctx)
	if err != nil {
		t.Fatalf("failed to create the Lidarr: %v", err)
	}
	t.Cleanup(func() { _ = client.Source.DeleteOne(lidarr).Exec(context.Background()) })
	if err := client.LibrarySource.Create().SetLibraryID(Music.ID).SetSourceID(lidarr.ID).Exec(ctx); err != nil {
		t.Fatalf("failed to bind the Lidarr: %v", err)
	}
	if !shows() {
		t.Error("Music is hidden while a Lidarr feeds it")
	}
}

func serviceEnsureFixedShows(t *testing.T, service *Service, ctx context.Context) func() bool {
	t.Helper()

	return func() bool {
		t.Helper()

		shown, err := service.ShownLibraries(ctx)
		if err != nil {
			t.Fatalf("ShownLibraries = %v", err)
		}

		return slices.ContainsFunc(shown, func(library *Library) bool { return library.ID == Music.ID })
	}
}
