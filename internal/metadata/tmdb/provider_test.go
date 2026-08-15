package tmdb

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/FreekingDean/gojellyfin/internal/env"
)

func stub(t *testing.T) *Client {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		body, found := details[request.URL.Path]
		if strings.HasPrefix(request.URL.Path, "/3/search/") {
			body, found = searches[request.URL.Query().Get("query")]
		}
		if !found {
			writer.Header().Set("Content-Type", "application/json")
			writer.WriteHeader(http.StatusNotFound)
			_, _ = writer.Write([]byte(`{"success":false,"status_code":34,"status_message":"The resource you requested could not be found."}`))

			return
		}

		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(body))
	}))
	t.Cleanup(server.Close)

	client, err := newClient(server.URL+"/3", "test-key")
	if err != nil {
		t.Fatalf("failed to build the client: %v", err)
	}

	return client
}

func released(value int32) *int32 {
	return &value
}

func TestMovieMapsWhatTmdbReturns(t *testing.T) {
	found, matched, err := stub(t).Movie(context.Background(), "The Matrix", released(1999))
	if err != nil {
		t.Fatalf("the lookup failed: %v", err)
	}
	if !matched {
		t.Fatal("a title the stub carries did not match")
	}

	if ids := *found.ProviderIds; ids[providerTmdb] != "603" || ids[providerImdb] != "tt0133093" {
		t.Errorf("ProviderIds = %v, want the Tmdb and Imdb ids", ids)
	}
	if found.OfficialRating == nil || *found.OfficialRating != "R" {
		t.Errorf("OfficialRating = %v, want R", found.OfficialRating)
	}
	if found.CommunityRating == nil || *found.CommunityRating < 8.19 || *found.CommunityRating > 8.21 {
		t.Errorf("CommunityRating = %v, want 8.2", found.CommunityRating)
	}
	if found.PremiereDate == nil || found.PremiereDate.Year() != 1999 {
		t.Errorf("PremiereDate = %v, want 1999-03-30", found.PremiereDate)
	}
	if found.Taglines == nil || (*found.Taglines)[0] != "Welcome to the Real World." {
		t.Errorf("Taglines = %v, want the fetched one", found.Taglines)
	}
	if found.ProductionLocations == nil || (*found.ProductionLocations)[0] != "United States of America" {
		t.Errorf("ProductionLocations = %v, want the fetched one", found.ProductionLocations)
	}
	if found.Status != nil {
		t.Errorf("Status = %v, want none written for a movie", *found.Status)
	}
}

func TestSeriesMapsStatusToJellyfinsVocabulary(t *testing.T) {
	found, matched, err := stub(t).Series(context.Background(), "Breaking Bad", released(2008))
	if err != nil {
		t.Fatalf("the lookup failed: %v", err)
	}
	if !matched {
		t.Fatal("a title the stub carries did not match")
	}

	if found.Status == nil || *found.Status != "Ended" {
		t.Errorf("Status = %v, want Ended", found.Status)
	}
	if found.OfficialRating == nil || *found.OfficialRating != "TV-MA" {
		t.Errorf("OfficialRating = %v, want TV-MA", found.OfficialRating)
	}
	if found.EndDate == nil || found.EndDate.Year() != 2013 {
		t.Errorf("EndDate = %v, want 2013-09-29", found.EndDate)
	}
	if ids := *found.ProviderIds; ids[providerTmdb] != "1396" || ids[providerImdb] != "tt0903747" {
		t.Errorf("ProviderIds = %v, want the Tmdb and Imdb ids", ids)
	}
}

func TestSeriesStatusMapping(t *testing.T) {
	for tmdbStatus, want := range map[string]string{
		"Returning Series": "Continuing",
		"Ended":            "Ended",
		"Canceled":         "Ended",
		"In Production":    "Unreleased",
		"Planned":          "Unreleased",
		"Pilot":            "Unreleased",
		"Something New":    "",
	} {
		if got := seriesStatus(tmdbStatus); got != want {
			t.Errorf("seriesStatus(%q) = %q, want %q", tmdbStatus, got, want)
		}
	}
}

func TestEpisodeReadsTheSeriesIdThisProviderWrote(t *testing.T) {
	client := stub(t)

	found, matched, err := client.Episode(context.Background(), map[string]string{providerTmdb: "1396"}, 1, 1)
	if err != nil {
		t.Fatalf("the lookup failed: %v", err)
	}
	if !matched {
		t.Fatal("an episode the stub carries did not match")
	}
	if found.Name == nil || *found.Name != "Pilot" {
		t.Errorf("Name = %v, want Pilot", found.Name)
	}
	if ids := *found.ProviderIds; ids[providerImdb] != "tt0959621" {
		t.Errorf("ProviderIds = %v, want the episode Imdb id", ids)
	}

	if _, matched, err := client.Episode(context.Background(), map[string]string{"Imdb": "tt0903747"}, 1, 1); err != nil || matched {
		t.Errorf("matched = %v, err = %v, want a miss without a Tmdb id", matched, err)
	}
}

func TestMissIsNotAnError(t *testing.T) {
	if _, matched, err := stub(t).Movie(context.Background(), "A Film Nobody Carries", nil); err != nil || matched {
		t.Errorf("matched = %v, err = %v, want a clean miss", matched, err)
	}
}

func TestClientIsOffWithoutAConfiguredKey(t *testing.T) {
	off, err := NewClient(env.Config{})
	if err != nil {
		t.Fatalf("an unconfigured client failed to build: %v", err)
	}
	if off.Enabled() {
		t.Error("a client with no key configured reported itself enabled")
	}

	if _, _, err := off.Movie(context.Background(), "The Matrix", nil); err == nil {
		t.Error("an unconfigured client answered a lookup")
	}

	on, err := NewClient(env.Config{TMDB: env.TMDB{APIKey: "not-a-real-key"}})
	if err != nil {
		t.Fatalf("a configured client failed to build: %v", err)
	}
	if !on.Enabled() {
		t.Error("a configured key did not reach the client")
	}
}
