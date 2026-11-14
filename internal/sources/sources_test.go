package sources

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	librarymembership "github.com/FreekingDean/gojellyfin/internal/store/libraryitem"
)

func (f *fixture) configured(t *testing.T) Configured {
	t.Helper()

	entry, err := f.service.Get(context.Background(), f.sourceID)
	if err != nil {
		t.Fatalf("failed to read the source: %v", err)
	}

	return entry
}

func (f *fixture) folder(t *testing.T) string {
	t.Helper()

	folder := "/srv/" + uuid.NewString()
	library, err := f.client.Library.Create().
		SetName(t.Name() + "-" + uuid.NewString()).
		SetLocations([]string{folder}).
		Save(context.Background())
	if err != nil {
		t.Fatalf("failed to create the library: %v", err)
	}
	t.Cleanup(func() {
		if err := f.client.Library.DeleteOne(library).Exec(context.Background()); err != nil {
			t.Errorf("failed to delete the library: %v", err)
		}
	})

	return folder
}

func candidate(local ...string) Configured {
	paths := make([]*Path, len(local))
	for i, path := range local {
		paths[i] = &Path{RemotePath: "/remote" + path, LocalPath: path}
	}

	return Configured{Source: Source{
		Name:           "candidate-" + uuid.NewString(),
		URL:            "http://" + uuid.NewString() + ".invalid",
		APIKeyVariable: "SOURCE_API_KEY_TEST",
		Kind:           KindSonarr,
		Paths:          paths,
	}}
}

func TestCreate_RefusesWhatCannotBeServed(t *testing.T) {
	fixture := newFixture(t, &[]movie{})
	fixture.library(t, "")
	elsewhere := fixture.folder(t)

	for name, entry := range map[string]Configured{
		"no folders":                candidate(),
		"a folder no library holds": candidate("/srv/" + uuid.NewString()),
		"a relative folder":         candidate("srv/tv"),
		"another source's folder":   candidate("/media/tv"),
		"a folder above another's":  candidate("/"),
		"one folder twice":          candidate(elsewhere, elsewhere+"/tv"),
		"a key outside the prefix": func() Configured {
			entry := candidate(elsewhere)
			entry.Source.APIKeyVariable = "DATABASE_URL"

			return entry
		}(),
		"no host": func() Configured {
			entry := candidate(elsewhere)
			entry.Source.URL = "sonarr:8989"

			return entry
		}(),
	} {
		t.Run(name, func(t *testing.T) {
			created, err := fixture.service.Create(context.Background(), entry)
			if !errors.Is(err, ErrInvalid) {
				t.Errorf("Create = %v, %v, want it refused as invalid", created.Source.ID, err)
			}
		})
	}
}

func TestCreateUpdateDelete(t *testing.T) {
	fixture := newFixture(t, &[]movie{})
	ctx := context.Background()
	elsewhere := fixture.folder(t)

	created, err := fixture.service.Create(ctx, candidate(elsewhere+"/tv", elsewhere+"/anime"))
	if err != nil {
		t.Fatalf("failed to create: %v", err)
	}
	if len(created.Source.Paths) != 2 {
		t.Fatalf("paths = %d, want both folders", len(created.Source.Paths))
	}

	changed := candidate(elsewhere + "/tv")
	updated, err := fixture.service.Update(ctx, created.Source.ID, changed)
	if err != nil {
		t.Fatalf("failed to update: %v", err)
	}
	if updated.Source.ID != created.Source.ID || updated.Source.URL != changed.Source.URL {
		t.Errorf("updated = %s at %s, want %s moved to %s",
			updated.Source.ID, updated.Source.URL, created.Source.ID, changed.Source.URL)
	}
	if len(updated.Source.Paths) != 1 {
		t.Errorf("paths = %d, want the dropped folder gone", len(updated.Source.Paths))
	}

	if err := fixture.service.Delete(ctx, created.Source.ID); err != nil {
		t.Fatalf("failed to delete: %v", err)
	}
	if _, err := fixture.service.Get(ctx, created.Source.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("Get after Delete = %v, want ErrNotFound", err)
	}
	if _, err := fixture.service.Update(ctx, created.Source.ID, changed); !errors.Is(err, ErrNotFound) {
		t.Errorf("Update after Delete = %v, want ErrNotFound", err)
	}
	if err := fixture.service.Delete(ctx, created.Source.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("Delete after Delete = %v, want ErrNotFound", err)
	}
}

func TestUpdate_KeepsTheFilesAcrossANewURL(t *testing.T) {
	reported := []movie{
		{TmdbID: 949, Title: "Heat", Year: 1995, HasFile: true,
			Path: "/media/Heat", File: at("/media/Heat/heat.mkv")},
	}

	fixture := newFixture(t, &reported)
	library := fixture.library(t, "")
	fixture.write(t, library, fixture.refresh(t, library))

	before := fixture.files(t)
	if len(before) != 1 {
		t.Fatalf("files = %v, want the one the source reported", before)
	}

	entry := fixture.configured(t)
	entry.Source.URL = "http://moved-" + uuid.NewString() + ".invalid"
	if _, err := fixture.service.Update(context.Background(), fixture.sourceID, entry); err != nil {
		t.Fatalf("failed to update: %v", err)
	}

	after := fixture.files(t)
	for path, id := range before {
		if after[path] != id {
			t.Errorf("%s went away when the source's URL changed", path)
		}
	}
}

func TestRefresh_RecordsWhetherTheInstanceAnswered(t *testing.T) {
	fixture := newFixture(t, &[]movie{})
	library := fixture.library(t, "")

	fixture.refresh(t, library)

	entry := fixture.configured(t)
	if entry.Source.Version != "5.14.0" || entry.Source.ContactedAt == nil || entry.Source.LastError != "" {
		t.Errorf("health = %q at %v (%q), want the version the instance reported and no error",
			entry.Source.Version, entry.Source.ContactedAt, entry.Source.LastError)
	}
}

func TestUpdate_KeepsTheBindingsItStillNames(t *testing.T) {
	fixture := newFixture(t, &[]movie{})
	kept := fixture.library(t, "")
	dropped := fixture.library(t, "")

	entry := fixture.configured(t)
	before := map[uuid.UUID]uuid.UUID{}
	for _, bound := range entry.Libraries {
		before[bound.LibraryID] = bound.ID
	}

	entry.Libraries = []LibrarySource{{LibraryID: kept, TagFilter: "4k"}}
	updated, err := fixture.service.Update(context.Background(), fixture.sourceID, entry)
	if err != nil {
		t.Fatalf("failed to update: %v", err)
	}

	if len(updated.Libraries) != 1 {
		t.Fatalf("libraries = %d, want %s dropped", len(updated.Libraries), dropped)
	}
	if got := updated.Libraries[0]; got.ID != before[kept] || got.TagFilter != "4k" {
		t.Errorf("binding = %s filtering %q, want %s kept in place with the new filter", got.ID, got.TagFilter, before[kept])
	}
}

func TestUpdate_UnbindingALibraryTakesItsTitlesOutOfIt(t *testing.T) {
	reported := []movie{
		{TmdbID: 949, Title: "Heat", Year: 1995, HasFile: true,
			Path: "/media/Heat", File: at("/media/Heat/heat.mkv")},
	}

	fixture := newFixture(t, &reported)
	kept := fixture.library(t, "")
	dropped := fixture.library(t, "")
	for _, library := range []uuid.UUID{kept, dropped} {
		fixture.write(t, library, fixture.refresh(t, library))
	}

	entry := fixture.configured(t)
	entry.Libraries = []LibrarySource{{LibraryID: kept}}
	if _, err := fixture.service.Update(context.Background(), fixture.sourceID, entry); err != nil {
		t.Fatalf("failed to update: %v", err)
	}

	for library, want := range map[uuid.UUID]int{kept: 1, dropped: 0} {
		held, err := fixture.client.LibraryItem.Query().
			Where(librarymembership.LibraryID(library)).
			Count(context.Background())
		if err != nil {
			t.Fatalf("failed to count the membership: %v", err)
		}
		if held != want {
			t.Errorf("library %s holds %d titles, want %d", library, held, want)
		}
	}
	if files := fixture.files(t); len(files) != 1 {
		t.Errorf("files = %v, want the source's file kept for the library still bound", files)
	}
}
