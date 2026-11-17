package reports

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/auth"
	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/reports"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/sessions"
	"github.com/FreekingDean/gojellyfin/internal/store"
	itemmodel "github.com/FreekingDean/gojellyfin/internal/store/item"
	"github.com/FreekingDean/gojellyfin/internal/users"
)

type stubQueue struct {
	queue   []*reports.Report
	filed   []reports.Filing
	decided map[string]uuid.UUID
	failing error
}

func (s *stubQueue) File(_ context.Context, filing reports.Filing) error {
	s.filed = append(s.filed, filing)

	return nil
}

func (s *stubQueue) Queue(context.Context) ([]*reports.Report, error) { return s.queue, nil }

func (s *stubQueue) decide(decision string, reportID uuid.UUID) error {
	if s.decided == nil {
		s.decided = map[string]uuid.UUID{}
	}
	s.decided[decision] = reportID

	return s.failing
}

func (s *stubQueue) Resolve(_ context.Context, id uuid.UUID) error { return s.decide("resolve", id) }
func (s *stubQueue) Dismiss(_ context.Context, id uuid.UUID) error { return s.decide("dismiss", id) }
func (s *stubQueue) Replace(_ context.Context, id uuid.UUID) error { return s.decide("replace", id) }

type stubLibrary struct {
	held uuid.UUID
}

func (s stubLibrary) ItemByID(_ context.Context, _ items.Viewer, id uuid.UUID) (*items.Item, error) {
	if id != s.held {
		return nil, errors.New("no such item")
	}

	return &items.Item{ID: id}, nil
}

type everything struct{}

func (everything) Access(context.Context, uuid.UUID) (users.Access, error) {
	return users.Access{All: true}, nil
}

func signedIn() (context.Context, uuid.UUID) {
	userID := uuid.New()

	return auth.ContextWithSession(context.Background(), &sessions.Session{UserID: &userID}), userID
}

func TestServer_GoJellyfinListReports(t *testing.T) {
	season, episode := int32(2), int32(5)
	fileID := uuid.New()
	title, file := int64(12), int64(34)
	queue := &stubQueue{queue: []*reports.Report{{
		ID:           uuid.New(),
		Kind:         reports.KindSubtitles,
		Origin:       reports.OriginUser,
		Status:       reports.StatusOpen,
		Note:         "out of sync",
		ItemSourceID: &fileID,
		User:         &store.UserModel{Name: "Dean"},
		File:         &store.ItemSourceModel{ID: fileID, Path: "/media/show/s02e05.mkv"},
		Item: &items.Item{
			Kind:              itemmodel.KindEpisode,
			Name:              "Fly",
			IndexNumber:       &episode,
			ParentIndexNumber: &season,
			Parent:            &items.Item{Parent: &items.Item{Name: "Breaking Bad"}},
			ItemSources:       []*store.ItemSourceModel{{ID: fileID, ExternalID: &title, ExternalFileID: &file}},
		},
		Attempt: &reports.Attempt{
			UserName:      "Dean",
			DeviceName:    "Living Room",
			Client:        "Android TV",
			ClientVersion: "0.19.0",
			Build:         "0.0.1",
			Playback: &items.Playback{
				VideoDirect: true,
				Container:   "mp4",
				VideoCodec:  "h264",
				AudioCodec:  "aac",
				Reasons:     []string{"AudioCodecNotSupported"},
			},
		},
	}}}

	serverGoJellyfinListReportsRest(t, queue)
}

func serverGoJellyfinListReportsRest(t *testing.T, queue *stubQueue) {
	t.Helper()

	response, err := (&Server{reports: queue}).GoJellyfinListReports(context.Background(), api.GoJellyfinListReportsRequestObject{})
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(response)
	if err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{
		`"ItemName":"Breaking Bad S2E5, Fly"`,
		`"Kind":"Subtitles"`,
		`"Origin":"User"`,
		`"Status":"Open"`,
		`"Note":"out of sync"`,
		`"UserName":"Dean"`,
		`"Path":"/media/show/s02e05.mkv"`,
		`"Replaceable":true`,
		`"DeviceName":"Living Room"`,
		`"Client":"Android TV"`,
		`"ClientVersion":"0.19.0"`,
		`"Direct":false`,
		`"VideoDirect":true`,
		`"AudioDirect":false`,
		`"AudioCodec":"aac"`,
		`"Reasons":["AudioCodecNotSupported"]`,
	} {
		if !strings.Contains(string(encoded), want) {
			t.Errorf("%s is missing %s", encoded, want)
		}
	}
}

func TestServer_GoJellyfinFileReport(t *testing.T) {
	held := uuid.New()
	note := "  no sound  "

	tests := []struct {
		name   string
		body   *api.ReportFiling
		signed bool
		want   any
		filed  int
	}{
		{name: "a report of a title the user can see is filed", body: &api.ReportFiling{ItemId: held, Kind: "Audio", Note: &note}, signed: true, want: api.GoJellyfinFileReport204Response{}, filed: 1},
		{name: "a kind nobody defined is refused", body: &api.ReportFiling{ItemId: held, Kind: "Boring"}, signed: true, want: api.GoJellyfinFileReport400JSONResponse{}},
		{name: "a title the user cannot see is not found", body: &api.ReportFiling{ItemId: uuid.New(), Kind: "Audio"}, signed: true, want: api.GoJellyfinFileReport404Response{}},
		{name: "no body is refused", signed: true, want: api.GoJellyfinFileReport400JSONResponse{}},
		{name: "nobody signed in files nothing", body: &api.ReportFiling{ItemId: held, Kind: "Audio"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			queue := &stubQueue{}
			s := &Server{reports: queue, items: stubLibrary{held: held}, policies: everything{}}
			ctx, userID := context.Background(), uuid.Nil
			if test.signed {
				ctx, userID = signedIn()
			}

			response, err := s.GoJellyfinFileReport(ctx, api.GoJellyfinFileReportRequestObject{Body: test.body})
			if !test.signed {
				if !errors.Is(err, auth.ErrUnauthorized) {
					t.Fatalf("err = %v, want ErrUnauthorized", err)
				}
			} else if err != nil {
				t.Fatal(err)
			}

			if test.want != nil {
				if got, want := typeOf(response), typeOf(test.want); got != want {
					t.Errorf("got %s, want %s", got, want)
				}
			}
			if len(queue.filed) != test.filed {
				t.Fatalf("filed %d reports, want %d", len(queue.filed), test.filed)
			}
			if test.filed == 0 {
				return
			}

			filing := queue.filed[0]
			if filing.ItemID != held || filing.Kind != reports.KindAudio || filing.Origin != reports.OriginUser || filing.Note != "no sound" {
				t.Errorf("filing = %+v", filing)
			}
			if filing.UserID == nil || *filing.UserID != userID {
				t.Errorf("filed by %v, want %s", filing.UserID, userID)
			}
		})
	}
}

func TestServer_Decisions(t *testing.T) {
	reportID := uuid.New()

	t.Run("each decision reaches the queue", func(t *testing.T) {
		queue := &stubQueue{}
		s := &Server{reports: queue}
		ctx := context.Background()

		resolved, err := s.GoJellyfinResolveReport(ctx, api.GoJellyfinResolveReportRequestObject{ReportId: reportID})
		if _, ok := resolved.(api.GoJellyfinResolveReport204Response); err != nil || !ok {
			t.Errorf("resolve = %T, %v", resolved, err)
		}
		dismissed, err := s.GoJellyfinDismissReport(ctx, api.GoJellyfinDismissReportRequestObject{ReportId: reportID})
		if _, ok := dismissed.(api.GoJellyfinDismissReport204Response); err != nil || !ok {
			t.Errorf("dismiss = %T, %v", dismissed, err)
		}
		replaced, err := s.GoJellyfinReplaceReported(ctx, api.GoJellyfinReplaceReportedRequestObject{ReportId: reportID})
		if _, ok := replaced.(api.GoJellyfinReplaceReported204Response); err != nil || !ok {
			t.Errorf("replace = %T, %v", replaced, err)
		}

		for _, decision := range []string{"resolve", "dismiss", "replace"} {
			if queue.decided[decision] != reportID {
				t.Errorf("%s reached %s, want %s", decision, queue.decided[decision], reportID)
			}
		}
	})

	t.Run("a report that is not there is a 404", func(t *testing.T) {
		s := &Server{reports: &stubQueue{failing: reports.ErrNotFound}}
		ctx := context.Background()

		resolved, _ := s.GoJellyfinResolveReport(ctx, api.GoJellyfinResolveReportRequestObject{ReportId: reportID})
		dismissed, _ := s.GoJellyfinDismissReport(ctx, api.GoJellyfinDismissReportRequestObject{ReportId: reportID})
		replaced, _ := s.GoJellyfinReplaceReported(ctx, api.GoJellyfinReplaceReportedRequestObject{ReportId: reportID})

		_, first := resolved.(api.GoJellyfinResolveReport404Response)
		_, second := dismissed.(api.GoJellyfinDismissReport404Response)
		_, third := replaced.(api.GoJellyfinReplaceReported404Response)
		if !first || !second || !third {
			t.Errorf("got %T, %T and %T, want three 404s", resolved, dismissed, replaced)
		}
	})

	t.Run("a replacement the downloader refused says why", func(t *testing.T) {
		s := &Server{reports: &stubQueue{failing: reports.ErrNothingToReplace}}

		response, err := s.GoJellyfinReplaceReported(context.Background(), api.GoJellyfinReplaceReportedRequestObject{ReportId: reportID})
		if err != nil {
			t.Fatal(err)
		}
		refused, ok := response.(api.GoJellyfinReplaceReported400JSONResponse)
		if !ok || refused.Message != reports.ErrNothingToReplace.Error() {
			t.Errorf("got %T %+v, want a 400 carrying the reason", response, response)
		}
	})
}

func typeOf(value any) string { return fmt.Sprintf("%T", value) }
