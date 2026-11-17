package userdata

import (
	"context"
	"slices"
	"testing"

	"github.com/FreekingDean/gojellyfin/internal/storetest"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/auth"
	"github.com/FreekingDean/gojellyfin/internal/discover"
	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/requests"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/sessions"
	itemmodel "github.com/FreekingDean/gojellyfin/internal/store/item"
)

type stubUnowned struct {
	title discover.Title
}

func (s stubUnowned) Title(_ context.Context, id uuid.UUID) (discover.Title, bool, error) {
	return s.title, id == s.title.ID, nil
}

type stubRequests struct {
	refusal   error
	requested []string
	withdrawn []string
}

func (s *stubRequests) Request(_ context.Context, _ uuid.UUID, title discover.Title) error {
	if s.refusal != nil {
		return s.refusal
	}
	s.requested = append(s.requested, title.Key)

	return nil
}

func (s *stubRequests) Withdraw(_ context.Context, _ uuid.UUID, key string) error {
	s.withdrawn = append(s.withdrawn, key)

	return nil
}

func library(t *testing.T) *items.Service {
	t.Helper()

	connection := storetest.Open(t)
	t.Cleanup(func() {
	})

	return items.New(connection.Client())
}

func TestServer_FavoriteAnUnownedTitle(t *testing.T) {
	tmdbID := uuid.New().ID()
	title := discover.Title{
		ID:     discover.ID(itemmodel.KindMovie, tmdbID),
		Kind:   itemmodel.KindMovie,
		TmdbID: tmdbID,
		Key:    discover.Key(itemmodel.KindMovie, tmdbID),
	}
	userID := uuid.New()
	signedIn := auth.ContextWithSession(context.Background(), &sessions.Session{UserID: &userID})
	held := library(t)

	t.Run("a heart is a request", func(t *testing.T) {
		asked := &stubRequests{}
		s := &Server{items: held, unowned: stubUnowned{title}, requests: asked}

		response, err := s.MarkFavoriteItem(signedIn, api.MarkFavoriteItemRequestObject{ItemId: title.ID})
		if err != nil {
			t.Fatal(err)
		}

		answered, ok := response.(api.MarkFavoriteItem200JSONResponse)
		if !ok {
			t.Fatalf("got %T, want a 200", response)
		}
		if answered.IsFavorite == nil || !*answered.IsFavorite {
			t.Errorf("got IsFavorite %v, want true", answered.IsFavorite)
		}
		if !slices.Equal(asked.requested, []string{title.Key}) {
			t.Errorf("got requests %v, want %s", asked.requested, title.Key)
		}
	})

	t.Run("taking the heart off withdraws it", func(t *testing.T) {
		asked := &stubRequests{}
		s := &Server{items: held, unowned: stubUnowned{title}, requests: asked}

		_, err := s.UnmarkFavoriteItem(signedIn, api.UnmarkFavoriteItemRequestObject{ItemId: title.ID})
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(asked.withdrawn, []string{title.Key}) {
			t.Errorf("got withdrawals %v, want %s", asked.withdrawn, title.Key)
		}
	})

	t.Run("no source to ask is a refusal", func(t *testing.T) {
		s := &Server{items: held, unowned: stubUnowned{title}, requests: &stubRequests{refusal: requests.ErrNowhere}}

		response, err := s.MarkFavoriteItem(signedIn, api.MarkFavoriteItemRequestObject{ItemId: title.ID})
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := response.(api.MarkFavoriteItem403Response); !ok {
			t.Errorf("got %T, want a 403", response)
		}
	})

	t.Run("nobody signed in asks for nothing", func(t *testing.T) {
		asked := &stubRequests{}
		s := &Server{items: held, unowned: stubUnowned{title}, requests: asked}

		_, err := s.MarkFavoriteItem(context.Background(), api.MarkFavoriteItemRequestObject{ItemId: title.ID})
		if err == nil || len(asked.requested) != 0 {
			t.Errorf("got error %v and requests %v, want a refusal and none", err, asked.requested)
		}
	})
}
