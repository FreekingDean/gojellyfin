package health

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/sources"
	"github.com/FreekingDean/gojellyfin/internal/store"
	itemmodel "github.com/FreekingDean/gojellyfin/internal/store/item"
	"github.com/FreekingDean/gojellyfin/internal/store/migrations"
)

const checkTimeout = 5 * time.Second

const appliedRevision = `SELECT version FROM atlas_schema_revisions.atlas_schema_revisions WHERE applied = total ORDER BY version DESC LIMIT 1`

type Status string

const (
	Ok   Status = "Ok"
	Warn Status = "Warn"
	Bad  Status = "Bad"
	Off  Status = "Off"
)

type Check struct {
	Name   string
	Status Status
	Detail string
	At     *time.Time
}

type Library struct {
	Movies       int
	Shows        int
	Episodes     int
	Unprobed     int
	Unidentified int
	RefreshedAt  *time.Time
}

type Report struct {
	Checks  []Check
	Library Library
}

type Jobs interface {
	Enabled() bool
	Pollers(ctx context.Context) (map[string]int, error)
}

type Metadata interface {
	Enabled() bool
}

type SignOn interface {
	Enabled() bool
	Issuer() string
	Discover(ctx context.Context) error
}

type Encoder interface {
	Enabled() bool
	Slots() (busy, total int)
}

type Sources interface {
	List(ctx context.Context) ([]sources.Configured, error)
}

type Tracing interface {
	Enabled() bool
}

type Catalogue interface {
	CountByKind(ctx context.Context) (map[string]int32, error)
	SourcesNeedingProbe(ctx context.Context) ([]uuid.UUID, error)
	ItemsNeedingMetadata(ctx context.Context, kinds []items.Kind, force bool, scope uuid.UUID) ([]uuid.UUID, error)
}

type Service struct {
	store     *store.Client
	jobs      Jobs
	metadata  Metadata
	signOn    SignOn
	encoder   Encoder
	sources   Sources
	tracing   Tracing
	catalogue Catalogue
}

func New(client *store.Client, jobs Jobs, metadata Metadata, signOn SignOn, encoder Encoder, sources Sources, tracing Tracing, catalogue Catalogue) *Service {
	return &Service{
		store:     client,
		jobs:      jobs,
		metadata:  metadata,
		signOn:    signOn,
		encoder:   encoder,
		sources:   sources,
		tracing:   tracing,
		catalogue: catalogue,
	}
}

func (s *Service) Report(ctx context.Context) (Report, error) {
	configured, err := s.sources.List(ctx)
	if err != nil {
		return Report{}, err
	}

	library, err := s.library(ctx, configured)
	if err != nil {
		return Report{}, err
	}

	probes := []func(context.Context) Check{s.database, s.background, s.metadataCheck, s.signOnCheck, s.transcoding}
	checks := make([]Check, len(probes))
	var wait sync.WaitGroup
	for i, probe := range probes {
		wait.Go(func() {
			bounded, cancel := context.WithTimeout(ctx, checkTimeout)
			defer cancel()
			checks[i] = probe(bounded)
		})
	}
	wait.Wait()

	for _, entry := range configured {
		checks = append(checks, downloader(entry))
	}
	checks = append(checks, s.tracingCheck())

	return Report{Checks: checks, Library: library}, nil
}

func (s *Service) database(ctx context.Context) Check {
	check := Check{Name: "Database"}

	latest, err := latestMigration()
	if err != nil {
		check.Status, check.Detail = Bad, err.Error()
		return check
	}

	rows, err := s.store.QueryContext(ctx, appliedRevision)
	if err != nil {
		check.Status, check.Detail = Warn, "Postgres answers, and holds no record of which migrations ran"
		return check
	}
	defer func() { _ = rows.Close() }()

	var applied string
	if rows.Next() {
		if err := rows.Scan(&applied); err != nil {
			check.Status, check.Detail = Bad, err.Error()
			return check
		}
	}
	if err := rows.Err(); err != nil && !errors.Is(err, sql.ErrNoRows) {
		check.Status, check.Detail = Bad, err.Error()
		return check
	}

	switch {
	case applied == latest:
		check.Status, check.Detail = Ok, "Postgres, schema current"
	case applied < latest:
		check.Status, check.Detail = Bad, fmt.Sprintf("Schema is at %s and this build needs %s: run gojellyfin migrate", orNone(applied), latest)
	default:
		check.Status, check.Detail = Warn, fmt.Sprintf("Schema is at %s, newer than this build's %s", applied, latest)
	}

	return check
}

func latestMigration() (string, error) {
	names, err := fs.Glob(migrations.FS, "*.sql")
	if err != nil || len(names) == 0 {
		return "", errors.New("this build carries no migrations")
	}
	sort.Strings(names)
	version, _, _ := strings.Cut(names[len(names)-1], "_")

	return version, nil
}

func orNone(version string) string {
	if version == "" {
		return "nothing"
	}

	return version
}

func (s *Service) background(ctx context.Context) Check {
	check := Check{Name: "Background work"}
	if !s.jobs.Enabled() {
		check.Status, check.Detail = Off, "Off. Set TEMPORAL_HOSTPORT"
		return check
	}

	pollers, err := s.jobs.Pollers(ctx)
	if err != nil {
		check.Status, check.Detail = Bad, "Temporal did not answer: "+err.Error()
		return check
	}

	unserved := []string{}
	for queue, count := range pollers {
		if count == 0 {
			unserved = append(unserved, queue)
		}
	}
	slices.Sort(unserved)

	switch {
	case len(unserved) == len(pollers):
		check.Status, check.Detail = Bad, "Temporal answers, and no worker is running"
	case len(unserved) > 0:
		check.Status, check.Detail = Warn, fmt.Sprintf("No worker serves %s", strings.Join(unserved, ", "))
	default:
		check.Status, check.Detail = Ok, fmt.Sprintf("Temporal, %d queues served", len(pollers))
	}

	return check
}

func (s *Service) metadataCheck(context.Context) Check {
	if !s.metadata.Enabled() {
		return Check{Name: "Metadata", Status: Off, Detail: "Off. Set TMDB_API_KEY"}
	}

	return Check{Name: "Metadata", Status: Ok, Detail: "TMDB_API_KEY set"}
}

func (s *Service) signOnCheck(ctx context.Context) Check {
	check := Check{Name: "Single sign-on"}
	if !s.signOn.Enabled() {
		check.Status, check.Detail = Off, "Off. Set OIDC_ISSUER_URL"
		return check
	}

	if err := s.signOn.Discover(ctx); err != nil {
		check.Status, check.Detail = Bad, err.Error()
		return check
	}

	check.Status, check.Detail = Ok, host(s.signOn.Issuer())

	return check
}

func host(address string) string {
	parsed, err := url.Parse(address)
	if err != nil || parsed.Host == "" {
		return address
	}

	return parsed.Host
}

func (s *Service) transcoding(context.Context) Check {
	check := Check{Name: "Transcoding"}
	if !s.encoder.Enabled() {
		check.Status, check.Detail = Bad, "ffmpeg is not installed, so nothing that needs converting plays"
		return check
	}

	busy, total := s.encoder.Slots()
	check.Status, check.Detail = Ok, fmt.Sprintf("ffmpeg found, %d of %d slots busy on this server", busy, total)
	if busy == total {
		check.Status = Warn
	}

	return check
}

func downloader(entry sources.Configured) Check {
	source := entry.Source
	check := Check{Name: source.Name, At: source.ContactedAt}

	switch {
	case source.LastError != "":
		check.Status, check.Detail = Warn, source.LastError
	case source.ContactedAt == nil:
		check.Status, check.Detail = Off, "Not read yet"
	default:
		check.Status, check.Detail = Ok, "v"+source.Version+", answered"
	}

	return check
}

func (s *Service) tracingCheck() Check {
	if !s.tracing.Enabled() {
		return Check{Name: "Tracing", Status: Off, Detail: "Off. Set OTEL_EXPORTER_OTLP_ENDPOINT"}
	}

	return Check{Name: "Tracing", Status: Ok, Detail: "Exporting a span per request"}
}

func (s *Service) library(ctx context.Context, configured []sources.Configured) (Library, error) {
	counts, err := s.catalogue.CountByKind(ctx)
	if err != nil {
		return Library{}, err
	}

	unprobed, err := s.catalogue.SourcesNeedingProbe(ctx)
	if err != nil {
		return Library{}, err
	}

	unidentified, err := s.catalogue.ItemsNeedingMetadata(ctx, []items.Kind{itemmodel.KindMovie, itemmodel.KindSeries, itemmodel.KindSeason, itemmodel.KindEpisode}, false, uuid.Nil)
	if err != nil {
		return Library{}, err
	}

	library := Library{
		Movies:       int(counts[string(itemmodel.KindMovie)]),
		Shows:        int(counts[string(itemmodel.KindSeries)]),
		Episodes:     int(counts[string(itemmodel.KindEpisode)]),
		Unprobed:     len(unprobed),
		Unidentified: len(unidentified),
	}
	for _, entry := range configured {
		for _, binding := range entry.Libraries {
			if binding.LastSyncedAt != nil && (library.RefreshedAt == nil || binding.LastSyncedAt.After(*library.RefreshedAt)) {
				library.RefreshedAt = binding.LastSyncedAt
			}
		}
	}

	return library, nil
}
