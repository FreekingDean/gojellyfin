package library

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/FreekingDean/gojellyfin/internal/server/api"
)

func TestServer_Similar(t *testing.T) {
	s := New(nil, nil, nil, nil, nil)
	ctx := context.Background()

	for _, tc := range []struct {
		name string
		call func() (any, error)
	}{
		{"items", func() (any, error) { return s.GetSimilarItems(ctx, api.GetSimilarItemsRequestObject{}) }},
		{"movies", func() (any, error) { return s.GetSimilarMovies(ctx, api.GetSimilarMoviesRequestObject{}) }},
		{"shows", func() (any, error) { return s.GetSimilarShows(ctx, api.GetSimilarShowsRequestObject{}) }},
		{"trailers", func() (any, error) { return s.GetSimilarTrailers(ctx, api.GetSimilarTrailersRequestObject{}) }},
		{"albums", func() (any, error) { return s.GetSimilarAlbums(ctx, api.GetSimilarAlbumsRequestObject{}) }},
		{"artists", func() (any, error) { return s.GetSimilarArtists(ctx, api.GetSimilarArtistsRequestObject{}) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
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
}
