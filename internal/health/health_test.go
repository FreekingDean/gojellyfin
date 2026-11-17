package health

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/FreekingDean/gojellyfin/internal/storetest"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/sources"
	"github.com/FreekingDean/gojellyfin/internal/store"
)

type stubJobs struct {
	enabled bool
	pollers map[string]int
	err     error
}

func (s stubJobs) Enabled() bool { return s.enabled }

func (s stubJobs) Pollers(context.Context) (map[string]int, error) { return s.pollers, s.err }

type stubSwitch bool

func (s stubSwitch) Enabled() bool { return bool(s) }

type stubSignOn struct {
	issuer string
	err    error
}

func (s stubSignOn) Enabled() bool { return s.issuer != "" }

func (s stubSignOn) Issuer() string { return s.issuer }

func (s stubSignOn) Discover(context.Context) error { return s.err }

type stubEncoder struct {
	installed   bool
	busy, total int
}

func (s stubEncoder) Enabled() bool { return s.installed }

func (s stubEncoder) Slots() (int, int) { return s.busy, s.total }

type stubSources []sources.Configured

func (s stubSources) List(context.Context) ([]sources.Configured, error) { return s, nil }

type stubCatalogue struct{}

func (stubCatalogue) CountByKind(context.Context) (map[string]int32, error) {
	return map[string]int32{"Movie": 4, "Series": 2, "Episode": 9}, nil
}

func (stubCatalogue) SourcesNeedingProbe(context.Context) ([]uuid.UUID, error) {
	return []uuid.UUID{uuid.New()}, nil
}

func (stubCatalogue) ItemsNeedingMetadata(context.Context, []items.Kind, bool, uuid.UUID) ([]uuid.UUID, error) {
	return []uuid.UUID{uuid.New(), uuid.New()}, nil
}

func client(t *testing.T) *store.Client {
	t.Helper()

	connection := storetest.Open(t)

	return connection.Client()
}

func statuses(report Report) map[string]Check {
	found := map[string]Check{}
	for _, check := range report.Checks {
		found[check.Name] = check
	}

	return found
}

func TestService_Report(t *testing.T) {
	answered := time.Now().Add(-time.Minute)
	synced := time.Now().Add(-time.Hour)
	downloaders := stubSources{
		{Source: sources.Source{Name: "Radarr", Version: "5.14", ContactedAt: &answered}, Libraries: []sources.LibrarySource{{LastSyncedAt: &synced}}},
		{Source: sources.Source{Name: "Sonarr", LastError: "401 Unauthorized", ContactedAt: &answered}},
		{Source: sources.Source{Name: "Lidarr"}},
	}

	tests := []struct {
		name    string
		service *Service
		want    map[string]Status
	}{
		{
			name:    "nothing configured",
			service: New(client(t), stubJobs{}, stubSwitch(false), stubSignOn{}, stubEncoder{}, downloaders, stubSwitch(false), stubCatalogue{}),
			want: map[string]Status{
				"Database": Ok, "Background work": Off, "Metadata": Off, "Single sign-on": Off,
				"Transcoding": Bad, "Radarr": Ok, "Sonarr": Warn, "Lidarr": Off, "Tracing": Off,
			},
		},
		{
			name: "everything answering, every slot busy",
			service: New(client(t), stubJobs{enabled: true, pollers: map[string]int{"a": 1, "b": 2}}, stubSwitch(true),
				stubSignOn{issuer: "https://auth.example.net/realms/home"}, stubEncoder{installed: true, busy: 2, total: 2}, downloaders, stubSwitch(true), stubCatalogue{}),
			want: map[string]Status{
				"Background work": Ok, "Metadata": Ok, "Single sign-on": Ok, "Transcoding": Warn, "Tracing": Ok,
			},
		},
		{
			name: "a queue nobody serves and a provider that is down",
			service: New(client(t), stubJobs{enabled: true, pollers: map[string]int{"a": 1, "b": 0}}, stubSwitch(true),
				stubSignOn{issuer: "https://auth.example.net", err: errors.New("down")}, stubEncoder{installed: true, total: 2}, downloaders, stubSwitch(true), stubCatalogue{}),
			want: map[string]Status{"Background work": Warn, "Single sign-on": Bad, "Transcoding": Ok},
		},
		{
			name: "Temporal not answering",
			service: New(client(t), stubJobs{enabled: true, err: errors.New("unavailable")}, stubSwitch(true),
				stubSignOn{}, stubEncoder{installed: true, total: 2}, downloaders, stubSwitch(true), stubCatalogue{}),
			want: map[string]Status{"Background work": Bad},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			report, err := test.service.Report(context.Background())
			if err != nil {
				t.Fatalf("Report = %v", err)
			}

			found := statuses(report)
			for name, want := range test.want {
				if found[name].Status != want {
					t.Errorf("%s = %s (%s), want %s", name, found[name].Status, found[name].Detail, want)
				}
			}

			library := report.Library
			if library.Movies != 4 || library.Shows != 2 || library.Episodes != 9 || library.Unprobed != 1 || library.Unidentified != 2 {
				t.Errorf("library = %+v", library)
			}
			if library.RefreshedAt == nil || !library.RefreshedAt.Equal(synced) {
				t.Errorf("refreshed = %v, want %v", library.RefreshedAt, synced)
			}
		})
	}
}

func TestService_Report_SignOnNamesTheIssuersHost(t *testing.T) {
	service := New(client(t), stubJobs{}, stubSwitch(false), stubSignOn{issuer: "https://auth.example.net/realms/home"}, stubEncoder{}, stubSources{}, stubSwitch(false), stubCatalogue{})

	report, err := service.Report(context.Background())
	if err != nil {
		t.Fatalf("Report = %v", err)
	}
	if detail := statuses(report)["Single sign-on"].Detail; detail != "auth.example.net" {
		t.Errorf("detail = %q, want the issuer's host", detail)
	}
}
