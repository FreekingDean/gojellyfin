package webhook

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/http/mux"
	"github.com/FreekingDean/gojellyfin/internal/jobs"
	"github.com/FreekingDean/gojellyfin/internal/sources"
)

const secret = "secret"

type held struct {
	source sources.Source
}

func (h held) Authenticate(_ context.Context, id uuid.UUID, username, password string) (sources.Source, error) {
	if id != h.source.ID || username != sources.WebhookUsername || password != secret {
		return sources.Source{}, sources.ErrUnauthorized
	}

	return h.source, nil
}

type queue struct {
	names  []string
	params [][]jobs.Param
	err    error
}

func (q *queue) Enqueue(_ context.Context, name string, params ...jobs.Param) error {
	q.names = append(q.names, name)
	q.params = append(q.params, params)

	return q.err
}

func TestHandler_Serve(t *testing.T) {
	source := sources.Source{ID: uuid.New(), Name: "Radarr", Kind: sources.KindRadarr}
	imported := `{"eventType":"Download","movie":{"id":7,"tmdbId":603}}`

	for _, test := range []struct {
		name     string
		path     string
		password string
		body     string
		refused  error
		want     int
		enqueued int
	}{
		{"an import refreshes the movie", source.ID.String(), secret, imported, nil, http.StatusAccepted, 1},
		{"the test a save sends is answered", source.ID.String(), secret, `{"eventType":"Test"}`, nil, http.StatusNoContent, 0},
		{"a wrong password is refused", source.ID.String(), "guess", imported, nil, http.StatusUnauthorized, 0},
		{"another source's id is refused", uuid.NewString(), secret, imported, nil, http.StatusUnauthorized, 0},
		{"a path that is not an id is not found", "radarr", secret, imported, nil, http.StatusNotFound, 0},
		{"a body that is not an event is refused", source.ID.String(), secret, "{", nil, http.StatusBadRequest, 0},
		{"no background work is unavailable", source.ID.String(), secret, imported, errors.New("off"), http.StatusServiceUnavailable, 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			enqueued := &queue{err: test.refused}
			m := mux.New()
			(&Handler{sources: held{source: source}, jobs: enqueued}).Register(m)

			req := httptest.NewRequest(http.MethodPost, "/webhooks/"+test.path, strings.NewReader(test.body))
			req.SetBasicAuth(sources.WebhookUsername, test.password)
			recorder := httptest.NewRecorder()
			m.ServeHTTP(recorder, req)

			if recorder.Code != test.want {
				t.Errorf("status = %d, want %d", recorder.Code, test.want)
			}
			if len(enqueued.names) != test.enqueued {
				t.Fatalf("enqueued = %v, want %d", enqueued.names, test.enqueued)
			}
			if test.enqueued > 0 && enqueued.names[0] != jobs.RefreshSourceTitle {
				t.Errorf("enqueued %s, want %s", enqueued.names[0], jobs.RefreshSourceTitle)
			}
		})
	}
}
