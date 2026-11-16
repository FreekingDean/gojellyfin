package stream

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/FreekingDean/gojellyfin/internal/auth"
	"github.com/google/uuid"
)

func TestHandler_Tagged(t *testing.T) {
	fixture := newFixture(t)
	id := fixture.add(t, "tagged.mp3", "mp3")
	files, err := fixture.items.MediaSources(context.Background(), id)
	if err != nil || len(files) != 1 {
		t.Fatalf("failed to read the file: %v", err)
	}
	source := files[0]
	session, err := fixture.handler.sessions.ByToken(context.Background(), fixture.token)
	if err != nil {
		t.Fatalf("failed to read the session: %v", err)
	}
	tag := auth.StreamTag(session.ID, fixture.token, source.ID)
	serve := fixture.handler.Tagged(fixture.handler.Serve)

	tests := []struct {
		name   string
		query  string
		status int
	}{
		{"serves the file the tag was minted for", "static=true&mediaSourceId=" + source.ID.String() + "&tag=" + tag, http.StatusOK},
		{"reads the parameters in any case", "Static=true&MediaSourceId=" + source.ID.String() + "&Tag=" + tag, http.StatusOK},
		{"refuses a tag minted for another file", "static=true&mediaSourceId=" + uuid.NewString() + "&tag=" + tag, http.StatusUnauthorized},
		{"refuses a forged tag", "static=true&mediaSourceId=" + source.ID.String() + "&tag=" + tag[:33] + "00", http.StatusUnauthorized},
		{"refuses a tag without the file it names", "static=true&tag=" + tag, http.StatusUnauthorized},
		{"refuses no credential at all", "static=true&mediaSourceId=" + source.ID.String(), http.StatusUnauthorized},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/Videos/"+id.String()+"/stream?"+test.query, nil)
			request.SetPathValue("itemId", id.String())
			recorder := httptest.NewRecorder()

			serve(recorder, request)

			if recorder.Code != test.status {
				t.Errorf("status = %d, want %d", recorder.Code, test.status)
			}
		})
	}

}
