package requests

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/requests"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/sources"
	"github.com/FreekingDean/gojellyfin/internal/store"
)

type stubQueue struct {
	queue    []requests.Asked
	askers   []requests.Asker
	failing  error
	approved []uuid.UUID
	declined []uuid.UUID
	allowed  map[uuid.UUID]requests.Allowance
}

func (s *stubQueue) Queue(context.Context) ([]requests.Asked, error) { return s.queue, nil }

func (s *stubQueue) Approve(_ context.Context, requestID uuid.UUID) error {
	s.approved = append(s.approved, requestID)

	return s.failing
}

func (s *stubQueue) Decline(_ context.Context, requestID uuid.UUID) error {
	s.declined = append(s.declined, requestID)

	return s.failing
}

func (s *stubQueue) Askers(context.Context) ([]requests.Asker, error) { return s.askers, nil }

func (s *stubQueue) Allow(_ context.Context, userID uuid.UUID, allowance requests.Allowance) error {
	if s.allowed == nil {
		s.allowed = map[uuid.UUID]requests.Allowance{}
	}
	s.allowed[userID] = allowance

	return s.failing
}

func TestServer_GoJellyfinListRequests(t *testing.T) {
	accepted := time.Date(2026, 11, 14, 9, 0, 0, 0, time.UTC)
	queue := &stubQueue{queue: []requests.Asked{{
		Key:    "movie:tmdb:603",
		Name:   "The Matrix",
		Kind:   "Movie",
		TmdbID: 603,
		Requests: []*requests.Request{
			{ID: uuid.New(), UserID: uuid.New(), Status: requests.StatusPending, User: &store.UserModel{Name: "Dean"}},
		},
		Submissions: []*requests.Submission{
			{AcceptedAt: &accepted, Source: &sources.Source{Name: "Radarr"}},
			{LastError: "no such profile", Source: &sources.Source{Name: "Radarr 4K"}},
		},
	}}}

	response, err := (&Server{requests: queue}).GoJellyfinListRequests(context.Background(), api.GoJellyfinListRequestsRequestObject{})
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(response)
	if err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{
		`"ItemId":"d15c07e2-71b1-4e5a-9f00-00010000025b"`,
		`"Name":"The Matrix"`,
		`"Kind":"Movie"`,
		`"UserName":"Dean"`,
		`"Status":"Pending"`,
		`"AcceptedAt":"2026-11-14T09:00:00Z","SourceName":"Radarr"`,
		`"Error":"no such profile"`,
	} {
		if !strings.Contains(string(encoded), want) {
			t.Errorf("got %s, want it to contain %s", encoded, want)
		}
	}
}

func TestServer_Decide(t *testing.T) {
	ctx := context.Background()
	id := uuid.New()

	tests := []struct {
		name    string
		failing error
		approve any
		decline any
	}{
		{name: "a decision that lands", approve: api.GoJellyfinApproveRequest204Response{}, decline: api.GoJellyfinDeclineRequest204Response{}},
		{
			name:    "a request nobody made",
			failing: requests.ErrNotFound,
			approve: api.GoJellyfinApproveRequest404Response{},
			decline: api.GoJellyfinDeclineRequest404Response{},
		},
		{
			name:    "no downloader to ask",
			failing: requests.ErrNowhere,
			approve: api.GoJellyfinApproveRequest400JSONResponse{Message: requests.ErrNowhere.Error()},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			queue := &stubQueue{failing: test.failing}
			server := &Server{requests: queue}

			approved, err := server.GoJellyfinApproveRequest(ctx, api.GoJellyfinApproveRequestRequestObject{RequestId: id})
			if err != nil || approved != test.approve {
				t.Errorf("approve = %#v, %v, want %#v", approved, err, test.approve)
			}
			if test.decline == nil {
				return
			}

			declined, err := server.GoJellyfinDeclineRequest(ctx, api.GoJellyfinDeclineRequestRequestObject{RequestId: id})
			if err != nil || declined != test.decline {
				t.Errorf("decline = %#v, %v, want %#v", declined, err, test.decline)
			}
			if len(queue.approved) != 1 || len(queue.declined) != 1 || queue.approved[0] != id || queue.declined[0] != id {
				t.Errorf("approved %v, declined %v", queue.approved, queue.declined)
			}
		})
	}
}

func TestServer_RequestUsers(t *testing.T) {
	ctx := context.Background()
	id := uuid.New()
	five, limit := 5, int32(3)

	t.Run("lists each user's allowance", func(t *testing.T) {
		queue := &stubQueue{askers: []requests.Asker{
			{ID: id, Name: "Dean", Used: 2, Allowance: requests.Allowance{CanRequest: true, Quota: &five}},
		}}

		response, err := (&Server{requests: queue}).GoJellyfinListRequestUsers(ctx, api.GoJellyfinListRequestUsersRequestObject{})
		if err != nil {
			t.Fatal(err)
		}
		encoded, _ := json.Marshal(response)
		for _, want := range []string{`"Name":"Dean"`, `"Used":2`, `"CanRequest":true`, `"NeedsApproval":false`, `"Quota":5`} {
			if !strings.Contains(string(encoded), want) {
				t.Errorf("got %s, want it to contain %s", encoded, want)
			}
		}
	})

	t.Run("saves what was sent, and no limit when none was", func(t *testing.T) {
		queue := &stubQueue{}
		server := &Server{requests: queue}

		for _, body := range []api.RequestAllowance{
			{CanRequest: true, NeedsApproval: true, Quota: &limit},
			{CanRequest: false},
		} {
			response, err := server.GoJellyfinUpdateRequestUser(ctx, api.GoJellyfinUpdateRequestUserRequestObject{UserId: id, Body: &body})
			if err != nil || response != (api.GoJellyfinUpdateRequestUser204Response{}) {
				t.Fatalf("response = %#v, %v", response, err)
			}

			saved := queue.allowed[id]
			if saved.CanRequest != body.CanRequest || saved.NeedsApproval != body.NeedsApproval || (saved.Quota == nil) != (body.Quota == nil) {
				t.Errorf("saved %+v for %+v", saved, body)
			}
		}
	})

	t.Run("a user nobody knows is a 404", func(t *testing.T) {
		server := &Server{requests: &stubQueue{failing: requests.ErrNotFound}}

		response, err := server.GoJellyfinUpdateRequestUser(ctx, api.GoJellyfinUpdateRequestUserRequestObject{UserId: id, Body: &api.RequestAllowance{}})
		if err != nil || response != (api.GoJellyfinUpdateRequestUser404Response{}) {
			t.Errorf("response = %#v, %v", response, err)
		}
	})
}
