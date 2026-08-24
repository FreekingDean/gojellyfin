package persons

import (
	"context"
	"slices"
	"testing"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/env"
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
		Path:      "/" + prefix + "Movie",
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
	if err := service.SaveProbe(ctx, movie, probe); err != nil {
		t.Fatalf("failed to save the probe: %v", err)
	}

	server := New(service)

	writerOnly := []string{string(creditmodal.KindWriter)}
	unknownKind := []string{"NotAKind"}
	term := "director"
	otherItem := uuid.New()

	tests := []struct {
		name   string
		params api.GetPersonsParams
		want   []string
	}{
		{
			name:   "returns the credited people",
			params: api.GetPersonsParams{AppearsInItemId: &movie.ID},
			want:   []string{director, writer},
		},
		{
			name:   "filters by person type",
			params: api.GetPersonsParams{AppearsInItemId: &movie.ID, PersonTypes: &writerOnly},
			want:   []string{writer},
		},
		{
			name:   "ignores unknown person types",
			params: api.GetPersonsParams{AppearsInItemId: &movie.ID, PersonTypes: &unknownKind},
			want:   []string{director, writer},
		},
		{
			name:   "filters by search term",
			params: api.GetPersonsParams{AppearsInItemId: &movie.ID, SearchTerm: &term},
			want:   []string{director},
		},
		{
			name:   "ignores other items",
			params: api.GetPersonsParams{AppearsInItemId: &otherItem},
			want:   nil,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := getPersons(t, server, test.params)

			if got := namesOf(result); !slices.Equal(got, test.want) {
				t.Fatalf("people = %v, want %v", got, test.want)
			}
			if result.TotalRecordCount == nil || int(*result.TotalRecordCount) != len(test.want) {
				t.Errorf("total = %v, want %d", result.TotalRecordCount, len(test.want))
			}
			for _, dto := range *result.Items {
				if dto.Type == nil || *dto.Type != api.BaseItemKindPerson {
					t.Errorf("type = %v, want %s", dto.Type, api.BaseItemKindPerson)
				}
				if dto.Id == nil || *dto.Id == uuid.Nil {
					t.Errorf("id = %v, want a person id", dto.Id)
				}
			}
		})
	}
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
