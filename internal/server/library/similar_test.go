package library

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	itemmodel "github.com/FreekingDean/gojellyfin/internal/store/item"
)

func TestServer_Similar(t *testing.T) {
	fixture := newFixture(t)
	ctx := fixture.signIn(t, "similar", true)
	s := fixture.server
	unknown := uuid.New()

	for _, tc := range []struct {
		name string
		call func() (any, error)
	}{
		{"items", func() (any, error) { return s.GetSimilarItems(ctx, api.GetSimilarItemsRequestObject{ItemId: unknown}) }},
		{"movies", func() (any, error) {
			return s.GetSimilarMovies(ctx, api.GetSimilarMoviesRequestObject{ItemId: unknown})
		}},
		{"shows", func() (any, error) { return s.GetSimilarShows(ctx, api.GetSimilarShowsRequestObject{ItemId: unknown}) }},
		{"trailers", func() (any, error) {
			return s.GetSimilarTrailers(ctx, api.GetSimilarTrailersRequestObject{ItemId: unknown})
		}},
		{"albums", func() (any, error) {
			return s.GetSimilarAlbums(ctx, api.GetSimilarAlbumsRequestObject{ItemId: unknown})
		}},
		{"artists", func() (any, error) {
			return s.GetSimilarArtists(ctx, api.GetSimilarArtistsRequestObject{ItemId: unknown})
		}},
	} {
		t.Run(tc.name+" of an id nobody holds is empty", func(t *testing.T) {
			response, err := tc.call()
			if err != nil {
				t.Fatal(err)
			}

			encoded, err := json.Marshal(response)
			if err != nil {
				t.Fatal(err)
			}

			for _, want := range []string{`"Items":[]`, `"TotalRecordCount":0`} {
				if !strings.Contains(string(encoded), want) {
					t.Errorf("got %s, want it to contain %s", encoded, want)
				}
			}
		})
	}

	t.Run("movies are ranked by what they share", moviesAreRankedByWhatTheyShare(fixture, s, ctx))
}

func moviesAreRankedByWhatTheyShare(fixture *fixture, s *Server, ctx context.Context) func(*testing.T) {
	return func(t *testing.T) {
		genre := "Drama " + uuid.NewString()
		catalogue := items.New(fixture.client)
		seedID := fixture.add(t, seed{kind: itemmodel.KindMovie, name: "Seed"})
		peerID := fixture.add(t, seed{kind: itemmodel.KindMovie, name: "Peer"})
		strangerID := fixture.add(t, seed{kind: itemmodel.KindMovie, name: "Stranger"})
		t.Cleanup(func() {
			if _, err := fixture.client.Item.Delete().Where(itemmodel.IDIn(seedID, peerID, strangerID)).Exec(context.Background()); err != nil {
				t.Errorf("failed to delete the films: %v", err)
			}
		})
		for _, id := range []uuid.UUID{seedID, peerID} {
			if _, err := catalogue.UpdateMetadata(context.Background(), id, items.Metadata{Genres: &[]string{genre}}); err != nil {
				t.Fatalf("failed to describe %s: %v", id, err)
			}
		}

		response, err := s.GetSimilarMovies(ctx, api.GetSimilarMoviesRequestObject{ItemId: seedID})
		if err != nil {
			t.Fatal(err)
		}
		found := response.(api.GetSimilarMovies200JSONResponse)
		if got := names(*found.Items); len(got) != 1 || got[0] != "Peer" {
			t.Errorf("similar = %v, want Peer", got)
		}
	}
}
