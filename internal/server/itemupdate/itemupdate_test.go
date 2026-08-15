package itemupdate

import (
	"context"
	"maps"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/env"
	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/libraries"
	"github.com/FreekingDean/gojellyfin/internal/localization"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
	"github.com/FreekingDean/gojellyfin/internal/store"
	itemmodal "github.com/FreekingDean/gojellyfin/internal/store/item"
)

var (
	probedAt     = time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)
	dateModified = time.Date(2021, 2, 3, 4, 5, 6, 0, time.UTC)
)

type fixture struct {
	server    *Server
	client    *store.Client
	libraryID uuid.UUID
	itemID    uuid.UUID
	folderID  uuid.UUID
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
	library, err := client.Library.Create().
		SetName(t.Name() + "-" + uuid.NewString()).
		SetCollectionType(libraries.CollectionTypeMovies).
		Save(ctx)
	if err != nil {
		t.Fatalf("failed to create the library: %v", err)
	}

	t.Cleanup(func() {
		if _, err := client.Item.Delete().Where(itemmodal.LibraryID(library.ID)).Exec(ctx); err != nil {
			t.Errorf("failed to delete the items: %v", err)
		}
		if err := client.Library.DeleteOne(library).Exec(ctx); err != nil {
			t.Errorf("failed to delete the library: %v", err)
		}
		if err := connection.Stop(); err != nil {
			t.Errorf("failed to close the database: %v", err)
		}
	})

	record, err := client.Item.Create().
		SetLibraryID(library.ID).
		SetKind(itemmodal.KindMovie).
		SetName("Original Name").
		SetSortName("original name").
		SetKey("movie:original-name").
		SetContainer("mkv").
		SetRunTimeTicks(72_000_000_000).
		SetProbedAt(probedAt).
		SetDateModified(dateModified).
		Save(ctx)
	if err != nil {
		t.Fatalf("failed to create the item: %v", err)
	}

	folder, err := client.Item.Create().
		SetLibraryID(library.ID).
		SetKind(itemmodal.KindFolder).
		SetIsFolder(true).
		SetName("Folder").
		SetSortName("folder").
		SetKey("folder:folder").
		Save(ctx)
	if err != nil {
		t.Fatalf("failed to create the folder: %v", err)
	}

	service, err := localization.New()
	if err != nil {
		t.Fatalf("failed to load the localization data: %v", err)
	}

	return &fixture{
		server:    New(items.New(client), libraries.New(client), service),
		client:    client,
		libraryID: library.ID,
		itemID:    record.ID,
		folderID:  folder.ID,
	}
}

func (f *fixture) item(t *testing.T) *store.Item {
	t.Helper()

	record, err := f.client.Item.Get(context.Background(), f.itemID)
	if err != nil {
		t.Fatalf("failed to read the item: %v", err)
	}

	return record
}

func (f *fixture) update(t *testing.T, id uuid.UUID, body api.BaseItemDto) api.UpdateItemResponseObject {
	t.Helper()

	response, err := f.server.UpdateItem(context.Background(), api.UpdateItemRequestObject{
		ItemId:   id,
		JSONBody: &body,
	})
	if err != nil {
		t.Fatalf("failed to update the item: %v", err)
	}

	return response
}

func (f *fixture) editorInfo(t *testing.T, id uuid.UUID) api.GetMetadataEditorInfo200JSONResponse {
	t.Helper()

	response, err := f.server.GetMetadataEditorInfo(context.Background(), api.GetMetadataEditorInfoRequestObject{ItemId: id})
	if err != nil {
		t.Fatalf("failed to get the metadata editor info: %v", err)
	}

	info, ok := response.(api.GetMetadataEditorInfo200JSONResponse)
	if !ok {
		t.Fatalf("response = %T, want GetMetadataEditorInfo200JSONResponse", response)
	}

	return info
}

func TestUpdateItem(t *testing.T) {
	fixture := newFixture(t)
	premiere := time.Date(1999, 3, 31, 0, 0, 0, 0, time.UTC)

	response := fixture.update(t, fixture.itemID, api.BaseItemDto{
		Name:           apiutil.Ptr("The Matrix"),
		SortName:       apiutil.Ptr("matrix, the"),
		OriginalTitle:  apiutil.Ptr("The Matrix"),
		Overview:       apiutil.Ptr("A hacker learns the truth."),
		OfficialRating: apiutil.Ptr("R"),
		ProductionYear: apiutil.Ptr(int32(1999)),
		PremiereDate:   &premiere,
		IndexNumber:    apiutil.Ptr(int32(3)),
		LockData:       apiutil.Ptr(true),
		Tags:           &[]string{"cyberpunk", "classic"},
		ProviderIds:    &map[string]*string{"Imdb": apiutil.Ptr("tt0133093")},
		LockedFields:   &[]api.MetadataField{api.MetadataFieldName, api.MetadataFieldOverview},
	})
	if _, ok := response.(api.UpdateItem204Response); !ok {
		t.Fatalf("response = %T, want UpdateItem204Response", response)
	}

	record := fixture.item(t)
	if record.Name != "The Matrix" {
		t.Errorf("name = %q, want %q", record.Name, "The Matrix")
	}
	if record.SortName != "matrix, the" {
		t.Errorf("sort name = %q, want %q", record.SortName, "matrix, the")
	}
	if record.Overview != "A hacker learns the truth." {
		t.Errorf("overview = %q, want the updated overview", record.Overview)
	}
	if record.OfficialRating != "R" {
		t.Errorf("official rating = %q, want %q", record.OfficialRating, "R")
	}
	if apiutil.Deref(record.ProductionYear) != 1999 {
		t.Errorf("production year = %v, want 1999", record.ProductionYear)
	}
	if record.PremiereDate == nil || !record.PremiereDate.Equal(premiere) {
		t.Errorf("premiere date = %v, want %v", record.PremiereDate, premiere)
	}
	if apiutil.Deref(record.IndexNumber) != 3 {
		t.Errorf("index number = %v, want 3", record.IndexNumber)
	}
	if !record.LockData {
		t.Error("lock data = false, want true")
	}
	if want := []string{"cyberpunk", "classic"}; !slices.Equal(record.Tags, want) {
		t.Errorf("tags = %v, want %v", record.Tags, want)
	}
	if want := []string{"Name", "Overview"}; !slices.Equal(record.LockedFields, want) {
		t.Errorf("locked fields = %v, want %v", record.LockedFields, want)
	}
	if want := map[string]string{"Imdb": "tt0133093"}; !maps.Equal(record.ProviderIds, want) {
		t.Errorf("provider ids = %v, want %v", record.ProviderIds, want)
	}
}

func TestUpdateItemKeepsForeignRating(t *testing.T) {
	fixture := newFixture(t)

	fixture.update(t, fixture.itemID, api.BaseItemDto{OfficialRating: apiutil.Ptr("FSK 16")})

	if record := fixture.item(t); record.OfficialRating != "FSK 16" {
		t.Errorf("official rating = %q, want %q", record.OfficialRating, "FSK 16")
	}
}

func TestUpdateItemKeepsProbedColumns(t *testing.T) {
	fixture := newFixture(t)

	fixture.update(t, fixture.itemID, api.BaseItemDto{
		Name:         apiutil.Ptr("Renamed"),
		Container:    apiutil.Ptr("mp4"),
		RunTimeTicks: apiutil.Ptr(int64(1)),
		Path:         apiutil.Ptr("/somewhere/else.mp4"),
		Type:         apiutil.Ptr(api.BaseItemKindEpisode),
		ParentId:     apiutil.Ptr(fixture.folderID),
		MediaSources: &[]api.MediaSourceInfo{{Container: apiutil.Ptr("mp4")}},
	})

	record := fixture.item(t)
	if record.Name != "Renamed" {
		t.Errorf("name = %q, want %q", record.Name, "Renamed")
	}
	if record.Container != "mkv" {
		t.Errorf("container = %q, want %q", record.Container, "mkv")
	}
	if apiutil.Deref(record.RunTimeTicks) != 72_000_000_000 {
		t.Errorf("run time ticks = %v, want 72000000000", record.RunTimeTicks)
	}
	if !record.ProbedAt.Equal(probedAt) {
		t.Errorf("probed at = %v, want %v", record.ProbedAt, probedAt)
	}
	if !record.DateModified.Equal(dateModified) {
		t.Errorf("date modified = %v, want %v", record.DateModified, dateModified)
	}
	if record.Key != "movie:original-name" {
		t.Errorf("key = %q, want the scan's key: the metadata editor must not move an item's identity", record.Key)
	}
	if record.Kind != itemmodal.KindMovie {
		t.Errorf("kind = %q, want %q", record.Kind, itemmodal.KindMovie)
	}
	if record.ParentID != nil {
		t.Errorf("parent id = %v, want none", record.ParentID)
	}
	if record.LibraryID != fixture.libraryID {
		t.Errorf("library id = %v, want %v", record.LibraryID, fixture.libraryID)
	}
	if sources, err := record.QueryMediaSources().Count(context.Background()); err != nil {
		t.Fatalf("failed to count the media sources: %v", err)
	} else if sources != 0 {
		t.Errorf("media sources = %d, want none", sources)
	}
}

func TestUpdateItemUnknownItem(t *testing.T) {
	fixture := newFixture(t)

	response := fixture.update(t, uuid.New(), api.BaseItemDto{Name: apiutil.Ptr("Nowhere")})
	if _, ok := response.(api.UpdateItem404JSONResponse); !ok {
		t.Fatalf("response = %T, want UpdateItem404JSONResponse", response)
	}
}

func TestGetMetadataEditorInfo(t *testing.T) {
	fixture := newFixture(t)

	t.Run("describes the item", func(t *testing.T) {
		info := fixture.editorInfo(t, fixture.itemID)

		if info.ContentType == nil || *info.ContentType != api.CollectionTypeMovies {
			t.Errorf("content type = %v, want movies", info.ContentType)
		}
		if len(apiutil.Deref(info.ContentTypeOptions)) != 0 {
			t.Errorf("content type options = %v, want none for a file", *info.ContentTypeOptions)
		}
		if len(apiutil.Deref(info.Countries)) == 0 {
			t.Error("countries are empty")
		}
		if len(apiutil.Deref(info.Cultures)) == 0 {
			t.Error("cultures are empty")
		}
		if len(apiutil.Deref(info.ParentalRatingOptions)) == 0 {
			t.Error("parental rating options are empty")
		}
	})

	t.Run("offers the supported content types for a folder", func(t *testing.T) {
		info := fixture.editorInfo(t, fixture.folderID)

		values := make([]string, 0, len(apiutil.Deref(info.ContentTypeOptions)))
		for _, option := range apiutil.Deref(info.ContentTypeOptions) {
			values = append(values, apiutil.Deref(option.Value))
		}
		if !slices.Contains(values, string(libraries.CollectionTypeMovies)) {
			t.Errorf("content type options = %v, want the supported collection types", values)
		}
		if len(values) != len(libraries.CollectionTypes) {
			t.Errorf("content type options = %v, want %d", values, len(libraries.CollectionTypes))
		}
	})

	t.Run("404s for an unknown item", func(t *testing.T) {
		response, err := fixture.server.GetMetadataEditorInfo(context.Background(), api.GetMetadataEditorInfoRequestObject{ItemId: uuid.New()})
		if err != nil {
			t.Fatalf("failed to get the metadata editor info: %v", err)
		}
		if _, ok := response.(api.GetMetadataEditorInfo404JSONResponse); !ok {
			t.Fatalf("response = %T, want GetMetadataEditorInfo404JSONResponse", response)
		}
	})
}

func TestUpdateItemContentType(t *testing.T) {
	fixture := newFixture(t)
	ctx := context.Background()

	cases := []struct {
		name        string
		contentType *string
		want        api.UpdateItemContentTypeResponseObject
	}{
		{name: "supported", contentType: apiutil.Ptr("tvshows"), want: api.UpdateItemContentType204Response{}},
		{name: "inherited", contentType: nil, want: api.UpdateItemContentType204Response{}},
		{name: "unhandleable", contentType: apiutil.Ptr("podcasts"), want: api.UpdateItemContentType403Response{}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			response, err := fixture.server.UpdateItemContentType(ctx, api.UpdateItemContentTypeRequestObject{
				ItemId: fixture.folderID,
				Params: api.UpdateItemContentTypeParams{ContentType: test.contentType},
			})
			if err != nil {
				t.Fatalf("failed to update the content type: %v", err)
			}
			if response != test.want {
				t.Errorf("response = %T, want %T", response, test.want)
			}
		})
	}

	t.Run("404s for an unknown item", func(t *testing.T) {
		response, err := fixture.server.UpdateItemContentType(ctx, api.UpdateItemContentTypeRequestObject{
			ItemId: uuid.New(),
			Params: api.UpdateItemContentTypeParams{ContentType: apiutil.Ptr("movies")},
		})
		if err != nil {
			t.Fatalf("failed to update the content type: %v", err)
		}
		if _, ok := response.(api.UpdateItemContentType404JSONResponse); !ok {
			t.Fatalf("response = %T, want UpdateItemContentType404JSONResponse", response)
		}
	})
}
