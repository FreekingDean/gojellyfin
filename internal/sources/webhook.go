package sources

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"slices"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/jobs"
	"github.com/FreekingDean/gojellyfin/internal/sources/arr"
	"github.com/FreekingDean/gojellyfin/internal/sources/lidarr"
	"github.com/FreekingDean/gojellyfin/internal/sources/radarr"
	"github.com/FreekingDean/gojellyfin/internal/sources/sonarr"
	"github.com/FreekingDean/gojellyfin/internal/store"
	librarysourcemodel "github.com/FreekingDean/gojellyfin/internal/store/librarysource"
	sourcemodel "github.com/FreekingDean/gojellyfin/internal/store/source"
)

const (
	WebhookPath     = "/webhooks"
	WebhookUsername = "gojellyfin"
)

var (
	ErrUnauthorized = errors.New("the webhook does not carry this source's secret")

	refreshingEvents = []string{"Download", "Rename", "MovieDelete", "MovieFileDelete", "SeriesDelete", "EpisodeFileDelete", "ArtistDelete", "AlbumDelete"}
)

type Hooked struct {
	ID            int64
	TmdbID        int
	MusicBrainzID string
	ReceivedAt    time.Time
}

func (s *Service) Authenticate(ctx context.Context, id uuid.UUID, username, password string) (Source, error) {
	source, err := s.store.Source.Query().Where(sourcemodel.ID(id)).OnlyModel(ctx)
	if store.IsNotFound(err) {
		return Source{}, ErrUnauthorized
	}
	if err != nil {
		return Source{}, err
	}

	if subtle.ConstantTimeCompare([]byte(username), []byte(WebhookUsername)) != 1 ||
		subtle.ConstantTimeCompare([]byte(password), []byte(source.WebhookSecret)) != 1 {
		return Source{}, ErrUnauthorized
	}

	return *source, nil
}

func Heard(source Source, body io.Reader) (Hooked, bool, error) {
	var event arr.Webhook
	if err := json.NewDecoder(body).Decode(&event); err != nil {
		return Hooked{}, false, fmt.Errorf("%w: the webhook body is not an event: %w", ErrInvalid, err)
	}

	if !slices.Contains(refreshingEvents, event.EventType) {
		return Hooked{}, false, nil
	}

	subject := event.Movie
	switch source.Kind {
	case KindSonarr:
		subject = event.Series
	case KindLidarr:
		subject = event.Artist
	}
	if subject == nil || subject.ID == 0 {
		return Hooked{}, false, nil
	}

	return Hooked{ID: subject.ID, TmdbID: subject.TmdbID, MusicBrainzID: subject.MBID, ReceivedAt: time.Now()}, true, nil
}

func (h Hooked) Params(sourceID uuid.UUID) []jobs.Param {
	return []jobs.Param{
		jobs.With(jobs.ParamSource, sourceID),
		jobs.With(jobs.ParamTitle, h),
	}
}

func (s *Service) RefreshTitleJob() jobs.Job {
	return jobs.Job{
		Name:        jobs.RefreshSourceTitle,
		Category:    "Library",
		Description: "Reads one movie, series or artist a Sonarr, Radarr or Lidarr webhook named.",
		Queue:       jobs.RefreshQueue,
		Run:         s.refreshTitle,
	}
}

func (s *Service) refreshTitle(ctx context.Context) error {
	sourceID, err := jobs.GetParam[uuid.UUID](ctx, jobs.ParamSource)
	if err != nil {
		return err
	}

	hooked, err := jobs.GetParam[Hooked](ctx, jobs.ParamTitle)
	if err != nil {
		return err
	}

	return s.RefreshTitle(ctx, sourceID, hooked)
}

func (s *Service) RefreshTitle(ctx context.Context, sourceID uuid.UUID, hooked Hooked) error {
	bindings, err := s.bindingsOf(ctx, sourceID)
	if err != nil || len(bindings) == 0 {
		return err
	}
	source := bindings[0].Source

	held, err := s.reported(ctx, source, hooked)
	if err != nil {
		return err
	}
	if !held.known() {
		log.Printf("%s named %d, which carries no TMDB or MusicBrainz id: the next refresh reads it", source.Name, hooked.ID)

		return nil
	}

	everything, err := held.titles(source, func([]int) bool { return true })
	if err != nil {
		return err
	}

	paths := make([]string, 0, len(everything))
	reported := make([]string, 0, len(everything))
	for _, title := range everything {
		reported = append(reported, title.Key)
		for _, file := range title.Files {
			paths = append(paths, file.Path)
		}
	}

	dropped, err := s.items.DeleteScopedSourcesNotInPaths(ctx, sourceID, held.scope(), paths)
	if err != nil {
		return err
	}

	for _, binding := range bindings {
		if err := s.refreshBound(ctx, binding, held); err != nil {
			return err
		}
	}

	return s.items.SweepUnreported(ctx, dropped, reported)
}

func (s *Service) refreshBound(ctx context.Context, binding Binding, held report) error {
	tagged, err := s.tagFilter(ctx, binding)
	if err != nil {
		return err
	}

	titles, err := held.titles(binding.Source, tagged)
	if err != nil {
		return err
	}

	keys := make([]string, 0, len(titles))
	for _, title := range titles {
		if title.Tagged {
			keys = append(keys, title.Key)
		}
	}

	libraryID, sourceID := binding.Library.LibraryID, binding.Source.ID
	if _, err := s.items.DeleteScopedMembershipNotInKeys(ctx, libraryID, sourceID, held.scope(), keys); err != nil {
		return err
	}

	for _, title := range titles {
		if err := jobs.Enqueue(ctx, jobs.RefreshItem,
			jobs.With(jobs.ParamItem, scanned(title)),
			jobs.With(jobs.ParamLibrary, libraryID),
			jobs.With(jobs.ParamSource, sourceID),
		); err != nil {
			return fmt.Errorf("failed to enqueue %s: %w", title.Key, err)
		}
	}

	return nil
}

type report struct {
	tmdbID        int
	musicBrainzID string
	series        bool
	movie         *radarr.Movie
	show          *sonarr.Series
	episodes      []sonarr.Episode
	artist        *lidarr.Artist
	music         []Title
}

func (r report) known() bool {
	return r.tmdbID != 0 || r.musicBrainzID != ""
}

func (r report) scope() items.Scope {
	if r.musicBrainzID != "" {
		return items.ArtistScope(r.musicBrainzID)
	}
	if r.series {
		return items.SeriesScope(r.tmdbID)
	}

	return items.MovieScope(r.tmdbID)
}

func (r report) titles(source Source, tagged func([]int) bool) ([]Title, error) {
	switch {
	case r.movie != nil:
		return movieTitles(source, tagged, *r.movie)
	case r.show != nil:
		return seriesTitles(source, tagged, *r.show, r.episodes), nil
	case r.artist != nil:
		return retagged(r.music, tagged(r.artist.Tags)), nil
	default:
		return nil, nil
	}
}

func (s *Service) reported(ctx context.Context, source Source, hooked Hooked) (report, error) {
	id := strconv.FormatInt(hooked.ID, 10)

	if source.Kind == KindLidarr {
		return s.reportedArtist(ctx, source, hooked)
	}

	if source.Kind == KindSonarr {
		show, err := get[sonarr.Series](ctx, s, source, sonarr.SeriesPath+"/"+id, nil)
		if errors.Is(err, errAbsent) {
			return report{tmdbID: hooked.TmdbID, series: true}, nil
		}
		if err != nil {
			return report{}, err
		}

		episodes, err := s.episodes(ctx, source, show)
		if err != nil {
			return report{}, err
		}

		return report{tmdbID: show.TmdbID, series: true, show: &show, episodes: episodes}, nil
	}

	movie, err := get[radarr.Movie](ctx, s, source, radarr.MoviesPath+"/"+id, nil)
	if errors.Is(err, errAbsent) {
		return report{tmdbID: hooked.TmdbID}, nil
	}
	if err != nil {
		return report{}, err
	}

	return report{tmdbID: movie.TmdbID, movie: &movie}, nil
}

func (s *Service) reportedArtist(ctx context.Context, source Source, hooked Hooked) (report, error) {
	artist, err := get[lidarr.Artist](ctx, s, source, lidarr.ArtistsPath+"/"+strconv.FormatInt(hooked.ID, 10), nil)
	if errors.Is(err, errAbsent) {
		return report{musicBrainzID: hooked.MusicBrainzID}, nil
	}
	if err != nil {
		return report{}, err
	}
	if artist.ForeignArtistID == "" {
		return report{}, nil
	}

	music, err := s.artistTitles(ctx, source, artist, true)
	if err != nil {
		return report{}, err
	}

	return report{musicBrainzID: artist.ForeignArtistID, artist: &artist, music: music}, nil
}

func retagged(titles []Title, tagged bool) []Title {
	out := make([]Title, len(titles))
	for i, title := range titles {
		title.Tagged = tagged
		out[i] = title
	}

	return out
}

func (s *Service) bindingsOf(ctx context.Context, sourceID uuid.UUID) ([]Binding, error) {
	records, err := s.store.LibrarySource.Query().
		Where(librarysourcemodel.SourceID(sourceID)).
		WithSource(func(query *store.SourceQuery) { query.WithPaths() }).
		AllModels(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to find what %s feeds: %w", sourceID, err)
	}

	bindings := make([]Binding, 0, len(records))
	for _, record := range records {
		if record.Source != nil {
			bindings = append(bindings, Binding{Source: *record.Source, Library: *record})
		}
	}

	return bindings, nil
}
