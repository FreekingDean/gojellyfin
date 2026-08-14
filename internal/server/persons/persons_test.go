package persons

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/store"
	creditmodal "github.com/FreekingDean/gojellyfin/internal/store/credit"
	itemmodal "github.com/FreekingDean/gojellyfin/internal/store/item"
	sourcemodal "github.com/FreekingDean/gojellyfin/internal/store/mediasource"
	personmodal "github.com/FreekingDean/gojellyfin/internal/store/person"
)

func TestGetPersons(t *testing.T) {
	ctx := context.Background()

	connection, err := store.NewStore()
	if err != nil {
		t.Fatalf("failed to open the database: %v", err)
	}
	if err := connection.Start(); err != nil {
		t.Fatalf("failed to reach the database, set DATABASE_URL: %v", err)
	}

	client := connection.Client()
	prefix := t.Name() + "-" + uuid.NewString() + " "
	library, err := client.Library.Create().SetName(prefix).Save(ctx)
	if err != nil {
		t.Fatalf("failed to create the library: %v", err)
	}

	t.Cleanup(func() {
		owned := itemmodal.LibraryID(library.ID)
		if _, err := client.Credit.Delete().Where(creditmodal.HasItemWith(owned)).Exec(ctx); err != nil {
			t.Errorf("failed to delete the credits: %v", err)
		}
		if _, err := client.MediaSource.Delete().Where(sourcemodal.HasItemWith(owned)).Exec(ctx); err != nil {
			t.Errorf("failed to delete the media sources: %v", err)
		}
		if _, err := client.Item.Delete().Where(owned).Exec(ctx); err != nil {
			t.Errorf("failed to delete the items: %v", err)
		}
		if err := client.Library.DeleteOne(library).Exec(ctx); err != nil {
			t.Errorf("failed to delete the library: %v", err)
		}
		if _, err := client.Person.Delete().Where(personmodal.NameHasPrefix(prefix)).Exec(ctx); err != nil {
			t.Errorf("failed to delete the people: %v", err)
		}
		if err := connection.Stop(); err != nil {
			t.Errorf("failed to close the database: %v", err)
		}
	})

	service := items.New(client)
	movie, err := service.SaveScanned(ctx, items.Scanned{
		LibraryID: library.ID,
		Kind:      itemmodal.KindMovie,
		Name:      prefix + "Movie",
		SortName:  prefix + "Movie",
		Key:       "test:" + prefix + "movie",
	})
	if err != nil {
		t.Fatalf("failed to save the item: %v", err)
	}

	director := prefix + "Director"
	writer := prefix + "Writer"
	probe := items.Probe{Metadata: items.ContainerMetadata{People: []items.Person{
		{Name: director, Kind: creditmodal.KindDirector},
		{Name: writer, Kind: creditmodal.KindWriter},
	}}}
	source, err := service.SaveSource(ctx, items.ScannedSource{
		LibraryID: library.ID,
		ItemID:    movie.ID,
		Path:      "/media/" + prefix + "Movie.mkv",
		Name:      prefix + "Movie",
	})
	if err != nil {
		t.Fatalf("failed to save the media source: %v", err)
	}
	if err := service.SaveProbe(ctx, movie, source, probe); err != nil {
		t.Fatalf("failed to save the probe: %v", err)
	}

	server := New(service)

	t.Run("returns the credited people", func(t *testing.T) {
		result := getPersons(t, server, api.GetPersonsParams{AppearsInItemId: &movie.ID})

		if got := namesOf(result); len(got) != 2 || got[0] != director || got[1] != writer {
			t.Fatalf("people = %v, want [%s %s]", got, director, writer)
		}
		if dto := (*result.Items)[0]; dto.Type == nil || *dto.Type != api.BaseItemKindPerson {
			t.Errorf("type = %v, want %s", dto.Type, api.BaseItemKindPerson)
		}
		if dto := (*result.Items)[0]; dto.Id == nil || *dto.Id == uuid.Nil {
			t.Errorf("id = %v, want a person id", dto.Id)
		}
	})

	t.Run("filters by person type", func(t *testing.T) {
		types := []string{string(creditmodal.KindWriter)}
		result := getPersons(t, server, api.GetPersonsParams{AppearsInItemId: &movie.ID, PersonTypes: &types})

		if got := namesOf(result); len(got) != 1 || got[0] != writer {
			t.Errorf("people = %v, want [%s]", got, writer)
		}
	})

	t.Run("ignores unknown person types", func(t *testing.T) {
		types := []string{"NotAKind"}
		result := getPersons(t, server, api.GetPersonsParams{AppearsInItemId: &movie.ID, PersonTypes: &types})

		if got := namesOf(result); len(got) != 2 {
			t.Errorf("people = %v, want both", got)
		}
	})

	t.Run("filters by search term", func(t *testing.T) {
		term := "director"
		result := getPersons(t, server, api.GetPersonsParams{AppearsInItemId: &movie.ID, SearchTerm: &term})

		if got := namesOf(result); len(got) != 1 || got[0] != director {
			t.Errorf("people = %v, want [%s]", got, director)
		}
	})

	t.Run("ignores other items", func(t *testing.T) {
		other := uuid.New()
		result := getPersons(t, server, api.GetPersonsParams{AppearsInItemId: &other})

		if got := namesOf(result); len(got) != 0 {
			t.Errorf("people = %v, want none", got)
		}
		if result.TotalRecordCount == nil || *result.TotalRecordCount != 0 {
			t.Errorf("total = %v, want 0", result.TotalRecordCount)
		}
	})
}

func getPersons(t *testing.T, server *Server, params api.GetPersonsParams) api.GetPersons200JSONResponse {
	t.Helper()

	response, err := server.GetPersons(context.Background(), api.GetPersonsRequestObject{Params: params})
	if err != nil {
		t.Fatalf("failed to get the people: %v", err)
	}

	result, ok := response.(api.GetPersons200JSONResponse)
	if !ok {
		t.Fatalf("response = %T, want api.GetPersons200JSONResponse", response)
	}

	return result
}

func namesOf(result api.GetPersons200JSONResponse) []string {
	found := make([]string, 0, len(*result.Items))
	for _, dto := range *result.Items {
		if dto.Name != nil {
			found = append(found, *dto.Name)
		}
	}

	return found
}
