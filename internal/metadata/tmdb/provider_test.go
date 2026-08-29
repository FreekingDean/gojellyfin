package tmdb

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/FreekingDean/gojellyfin/internal/env"
	"github.com/FreekingDean/gojellyfin/internal/items"
	imagemodal "github.com/FreekingDean/gojellyfin/internal/store/image"
)

type stubbed struct {
	client *Client

	mutex sync.Mutex
	asked []string
}

func (s *stubbed) requests() []string {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	return append([]string(nil), s.asked...)
}

func stub(t *testing.T) *Client {
	t.Helper()

	return newStub(t).client
}

func newStub(t *testing.T) *stubbed {
	t.Helper()

	stubbing := &stubbed{}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		stubbing.mutex.Lock()
		stubbing.asked = append(stubbing.asked, request.URL.Path)
		stubbing.mutex.Unlock()

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
	stubbing.client = client

	return stubbing
}

func artworkURL(t *testing.T, found []items.RemoteImage, kind items.ImageKind) string {
	t.Helper()

	for _, reference := range found {
		if reference.Kind == kind {
			return reference.URL
		}
	}

	return ""
}

func released(value int32) *int32 {
	return &value
}

func TestClient_Movie(t *testing.T) {
	t.Run("maps what Tmdb returns", func(t *testing.T) {
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
	})

	t.Run("answers a miss without an error", func(t *testing.T) {
		if _, matched, err := stub(t).Movie(context.Background(), "A Film Nobody Carries", nil); err != nil || matched {
			t.Errorf("matched = %v, err = %v, want a clean miss", matched, err)
		}
	})
}

func TestClient_Series(t *testing.T) {
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

func TestSeriesStatus(t *testing.T) {
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

func TestClient_Season(t *testing.T) {
	client := stub(t)

	t.Run("maps what Tmdb returns", func(t *testing.T) {
		found, matched, err := client.Season(context.Background(), map[string]string{providerTmdb: "1396"}, 1)
		if err != nil {
			t.Fatalf("the lookup failed: %v", err)
		}
		if !matched {
			t.Fatal("a season the stub carries did not match")
		}
		if found.Name == nil || *found.Name != "Season 1" {
			t.Errorf("Name = %v, want Season 1", found.Name)
		}
		if found.Overview == nil || !strings.HasPrefix(*found.Overview, "High school chemistry teacher") {
			t.Errorf("Overview = %v, want the fetched one", found.Overview)
		}
		if found.PremiereDate == nil || found.PremiereDate.Year() != 2008 {
			t.Errorf("PremiereDate = %v, want 2008-01-20", found.PremiereDate)
		}
		if found.ProductionYear == nil || *found.ProductionYear != 2008 {
			t.Errorf("ProductionYear = %v, want 2008", found.ProductionYear)
		}
		if ids := *found.ProviderIds; ids[providerTmdb] != "3572" {
			t.Errorf("ProviderIds = %v, want the season Tmdb id", ids)
		}
	})

	t.Run("asks for specials as season zero", func(t *testing.T) {
		found, matched, err := client.Season(context.Background(), map[string]string{providerTmdb: "1396"}, 0)
		if err != nil {
			t.Fatalf("the lookup failed: %v", err)
		}
		if !matched {
			t.Fatal("specials did not match")
		}
		if found.Name == nil || *found.Name != "Specials" {
			t.Errorf("Name = %v, want Specials", found.Name)
		}
	})

	t.Run("answers a miss without an error", func(t *testing.T) {
		if _, matched, err := client.Season(context.Background(), map[string]string{providerTmdb: "1396"}, 9); err != nil || matched {
			t.Errorf("matched = %v, err = %v, want a clean miss", matched, err)
		}
		if _, matched, err := client.Season(context.Background(), map[string]string{"Imdb": "tt0903747"}, 1); err != nil || matched {
			t.Errorf("matched = %v, err = %v, want a miss without a Tmdb id", matched, err)
		}
	})
}

func TestClient_Episode(t *testing.T) {
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

func TestNewClient(t *testing.T) {
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

func TestClient_Artwork(t *testing.T) {
	t.Run("names a movie's poster and backdrop", func(t *testing.T) {
		found, _, err := stub(t).Movie(context.Background(), "The Matrix", released(1999))
		if err != nil {
			t.Fatalf("the lookup failed: %v", err)
		}

		poster := "https://image.tmdb.org/t/p/w780/f89U3ADr1oiB1s9GkdPOEpXUk5H.jpg"
		if got := artworkURL(t, found.Images, imagemodal.KindPrimary); got != poster {
			t.Errorf("poster = %q, want %q", got, poster)
		}

		backdrop := "https://image.tmdb.org/t/p/w1280/ByDf0zjLSumz1MP1cDEo2JmHkrn.jpg"
		if got := artworkURL(t, found.Images, imagemodal.KindBackdrop); got != backdrop {
			t.Errorf("backdrop = %q, want %q", got, backdrop)
		}
	})

	t.Run("names an episode's still as its poster", func(t *testing.T) {
		series := map[string]string{providerTmdb: "1396"}
		found, _, err := stub(t).Episode(context.Background(), series, 1, 1)
		if err != nil {
			t.Fatalf("the lookup failed: %v", err)
		}

		still := "https://image.tmdb.org/t/p/w300/ydlY3iPfeOAvu8gVqrxPoMvzNCn.jpg"
		if got := artworkURL(t, found.Images, imagemodal.KindPrimary); got != still {
			t.Errorf("still = %q, want %q", got, still)
		}
	})

	t.Run("names nothing for a title carrying no artwork", func(t *testing.T) {
		series := map[string]string{providerTmdb: "1396"}
		found, _, err := stub(t).Season(context.Background(), series, 0)
		if err != nil {
			t.Fatalf("the lookup failed: %v", err)
		}
		if len(found.Images) != 1 {
			t.Errorf("Images = %v, want the poster alone", found.Images)
		}
	})

	t.Run("reads the image configuration once a process", func(t *testing.T) {
		stubbing := newStub(t)

		if _, _, err := stubbing.client.Movie(context.Background(), "The Matrix", released(1999)); err != nil {
			t.Fatalf("the first lookup failed: %v", err)
		}
		if _, _, err := stubbing.client.Series(context.Background(), "Breaking Bad", released(2008)); err != nil {
			t.Fatalf("the second lookup failed: %v", err)
		}

		read := 0
		for _, path := range stubbing.requests() {
			if path == "/3/configuration" {
				read++
			}
		}
		if read != 1 {
			t.Errorf("configuration reads = %d, want one for the whole process", read)
		}
	})
}
