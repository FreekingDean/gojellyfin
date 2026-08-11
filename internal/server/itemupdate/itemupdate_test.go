package itemupdate

import (
	"context"
	"encoding/json"
	"maps"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/config"
	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
	serverconfig "github.com/FreekingDean/gojellyfin/internal/server/configuration"
	"github.com/FreekingDean/gojellyfin/internal/store"
	configmodal "github.com/FreekingDean/gojellyfin/internal/store/configuration"
	itemmodal "github.com/FreekingDean/gojellyfin/internal/store/item"
)

type fixture struct {
	server *Server
	client *store.Client
	itemID uuid.UUID
}

func newFixture(t *testing.T) *fixture {
	t.Helper()

	connection, err := store.NewStore()
	if err != nil {
		t.Fatalf("failed to open the database: %v", err)
	}
	if err := connection.Start(); err != nil {
		t.Fatalf("failed to reach the database, set DATABASE_URL: %v", err)
	}

	ctx := context.Background()
	client := connection.Client()
	library, err := client.Library.Create().SetName(t.Name() + "-" + uuid.NewString()).Save(ctx)
	if err != nil {
		t.Fatalf("failed to create the library: %v", err)
	}

	settings := config.New(client)
	stored, err := settings.Configuration(ctx, serverconfig.SystemConfigurationKey)
	if err != nil {
		t.Fatalf("failed to read the server configuration: %v", err)
	}

	t.Cleanup(func() {
		if _, err := client.Item.Delete().Where(itemmodal.LibraryID(library.ID)).Exec(ctx); err != nil {
			t.Errorf("failed to delete the items: %v", err)
		}
		if err := client.Library.DeleteOne(library).Exec(ctx); err != nil {
			t.Errorf("failed to delete the library: %v", err)
		}
		if stored == nil {
			_, err = client.Configuration.Delete().
				Where(configmodal.Key(serverconfig.SystemConfigurationKey)).
				Exec(ctx)
		} else {
			err = settings.SetConfiguration(ctx, serverconfig.SystemConfigurationKey, stored)
		}
		if err != nil {
			t.Errorf("failed to restore the server configuration: %v", err)
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
		SetPath("/" + library.ID.String() + "/movie.mkv").
		SetContainer("mkv").
		SetRunTimeTicks(72_000_000_000).
		SetProbedAt(probedAt).
		SetDateModified(dateModified).
		Save(ctx)
	if err != nil {
		t.Fatalf("failed to create the item: %v", err)
	}

	return &fixture{
		server: New(items.New(client), settings),
		client: client,
		itemID: record.ID,
	}
}

var (
	probedAt     = time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)
	dateModified = time.Date(2021, 2, 3, 4, 5, 6, 0, time.UTC)
)

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

func TestUpdateItemKeepsProbedColumns(t *testing.T) {
	fixture := newFixture(t)

	fixture.update(t, fixture.itemID, api.BaseItemDto{
		Name:         apiutil.Ptr("Renamed"),
		Container:    apiutil.Ptr("mp4"),
		RunTimeTicks: apiutil.Ptr(int64(1)),
		Path:         apiutil.Ptr("/somewhere/else.mp4"),
	})

	record := fixture.item(t)
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
	if record.Path == "/somewhere/else.mp4" {
		t.Error("path was written by the metadata editor")
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
	ctx := context.Background()

	response, err := fixture.server.GetMetadataEditorInfo(ctx, api.GetMetadataEditorInfoRequestObject{ItemId: fixture.itemID})
	if err != nil {
		t.Fatalf("failed to get the metadata editor info: %v", err)
	}

	info, ok := response.(api.GetMetadataEditorInfo200JSONResponse)
	if !ok {
		t.Fatalf("response = %T, want GetMetadataEditorInfo200JSONResponse", response)
	}
	if len(apiutil.Deref(info.ContentTypeOptions)) == 0 {
		t.Error("content type options are empty")
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
	if info.ContentType != nil {
		t.Errorf("content type = %v, want none", *info.ContentType)
	}

	unknown, err := fixture.server.GetMetadataEditorInfo(ctx, api.GetMetadataEditorInfoRequestObject{ItemId: uuid.New()})
	if err != nil {
		t.Fatalf("failed to get the metadata editor info: %v", err)
	}
	if _, ok := unknown.(api.GetMetadataEditorInfo404JSONResponse); !ok {
		t.Fatalf("response = %T, want GetMetadataEditorInfo404JSONResponse", unknown)
	}
}

func TestUpdateItemContentType(t *testing.T) {
	fixture := newFixture(t)
	ctx := context.Background()

	response, err := fixture.server.UpdateItemContentType(ctx, api.UpdateItemContentTypeRequestObject{
		ItemId: fixture.itemID,
		Params: api.UpdateItemContentTypeParams{ContentType: apiutil.Ptr("tvshows")},
	})
	if err != nil {
		t.Fatalf("failed to update the content type: %v", err)
	}
	if _, ok := response.(api.UpdateItemContentType204Response); !ok {
		t.Fatalf("response = %T, want UpdateItemContentType204Response", response)
	}

	editor, err := fixture.server.GetMetadataEditorInfo(ctx, api.GetMetadataEditorInfoRequestObject{ItemId: fixture.itemID})
	if err != nil {
		t.Fatalf("failed to get the metadata editor info: %v", err)
	}
	info := editor.(api.GetMetadataEditorInfo200JSONResponse)
	if info.ContentType == nil || *info.ContentType != api.CollectionTypeTvshows {
		t.Fatalf("content type = %v, want tvshows", info.ContentType)
	}

	if _, err := fixture.server.UpdateItemContentType(ctx, api.UpdateItemContentTypeRequestObject{
		ItemId: fixture.itemID,
		Params: api.UpdateItemContentTypeParams{},
	}); err != nil {
		t.Fatalf("failed to clear the content type: %v", err)
	}

	stored, err := config.New(fixture.client).Configuration(ctx, serverconfig.SystemConfigurationKey)
	if err != nil {
		t.Fatalf("failed to read the server configuration: %v", err)
	}
	var configured api.ServerConfiguration
	if err := json.Unmarshal(stored, &configured); err != nil {
		t.Fatalf("failed to decode the server configuration: %v", err)
	}
	if len(apiutil.Deref(configured.ContentTypes)) != 0 {
		t.Errorf("content types = %v, want none", *configured.ContentTypes)
	}
}
