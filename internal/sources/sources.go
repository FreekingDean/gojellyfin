package sources

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/activity"
	"github.com/FreekingDean/gojellyfin/internal/env"
	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/sources/arr"
	"github.com/FreekingDean/gojellyfin/internal/store"
	itemsourcemodel "github.com/FreekingDean/gojellyfin/internal/store/itemsource"
	librarymodel "github.com/FreekingDean/gojellyfin/internal/store/library"
	libraryitemmodel "github.com/FreekingDean/gojellyfin/internal/store/libraryitem"
	librarysourcemodel "github.com/FreekingDean/gojellyfin/internal/store/librarysource"
	sourcemodel "github.com/FreekingDean/gojellyfin/internal/store/source"
	sourcepathmodel "github.com/FreekingDean/gojellyfin/internal/store/sourcepath"
)

const (
	testTimeout = 10 * time.Second
	listTimeout = 2 * time.Minute

	KindRadarr = sourcemodel.KindRadarr
	KindSonarr = sourcemodel.KindSonarr
	KindLidarr = sourcemodel.KindLidarr
)

var (
	ErrInvalid  = errors.New("invalid source")
	ErrNotFound = errors.New("no such source")
)

type Service struct {
	client   *http.Client
	lister   *http.Client
	store    *store.Client
	config   env.Config
	items    *items.Service
	activity *activity.Service
}

func New(store *store.Client, config env.Config, records *items.Service, activities *activity.Service) *Service {
	return &Service{
		client:   &http.Client{Timeout: testTimeout},
		lister:   &http.Client{Timeout: listTimeout},
		store:    store,
		config:   config,
		items:    records,
		activity: activities,
	}
}

func (s *Service) APIKey(variable string) (string, error) {
	if !env.ValidSourceAPIKeyVariable(variable) {
		return "", fmt.Errorf("%q is not an API key variable: the name must begin with %s", variable, env.SourceAPIKeyPrefix)
	}

	key := s.config.SourceAPIKeys[variable]
	if key == "" {
		return "", fmt.Errorf("%s is not set on this server", variable)
	}

	return key, nil
}

func (s *Service) APIKeyVariables() []string {
	names := make([]string, 0, len(s.config.SourceAPIKeys))
	for name := range s.config.SourceAPIKeys {
		names = append(names, name)
	}
	slices.Sort(names)

	return names
}

type (
	Kind          = sourcemodel.Kind
	Source        = store.SourceModel
	Path          = store.SourcePathModel
	LibrarySource = store.LibrarySourceModel
)

type Configured struct {
	Source    Source
	Libraries []LibrarySource
	Files     int
}

func (s *Service) List(ctx context.Context) ([]Configured, error) {
	records, err := s.store.Source.Query().
		WithPaths(byRemotePath).
		WithLibraries().
		Order(sourcemodel.ByName()).
		AllModels(ctx)
	if err != nil {
		return nil, err
	}

	counts, err := s.fileCounts(ctx)
	if err != nil {
		return nil, err
	}

	configured := make([]Configured, len(records))
	for i, record := range records {
		configured[i] = fromRecord(record)
		configured[i].Files = counts[record.ID]
	}

	return configured, nil
}

func (s *Service) Get(ctx context.Context, id uuid.UUID) (Configured, error) {
	record, err := s.store.Source.Query().
		Where(sourcemodel.ID(id)).
		WithPaths(byRemotePath).
		WithLibraries().
		OnlyModel(ctx)
	if store.IsNotFound(err) {
		return Configured{}, ErrNotFound
	}
	if err != nil {
		return Configured{}, err
	}

	counts, err := s.fileCounts(ctx, id)
	if err != nil {
		return Configured{}, err
	}

	configured := fromRecord(record)
	configured.Files = counts[id]

	return configured, nil
}

func (s *Service) fileCounts(ctx context.Context, ids ...uuid.UUID) (map[uuid.UUID]int, error) {
	query := s.store.ItemSource.Query()
	if len(ids) > 0 {
		query.Where(itemsourcemodel.SourceIDIn(ids...))
	}

	var rows []struct {
		SourceID uuid.UUID `json:"source_id"`
		Count    int       `json:"count"`
	}
	if err := query.GroupBy(itemsourcemodel.FieldSourceID).Aggregate(store.Count()).Scan(ctx, &rows); err != nil {
		return nil, err
	}

	counts := make(map[uuid.UUID]int, len(rows))
	for _, row := range rows {
		counts[row.SourceID] = row.Count
	}

	return counts, nil
}

func byRemotePath(query *store.SourcePathQuery) {
	query.Order(sourcepathmodel.ByRemotePath())
}

func fromRecord(record *Source) Configured {
	return Configured{
		Source:    *record,
		Libraries: libraries(record.Libraries),
	}
}

func (s *Service) BindingsFor(ctx context.Context, id uuid.UUID) ([]Binding, error) {
	records, err := s.store.LibrarySource.Query().
		Where(librarysourcemodel.LibraryID(id)).
		WithSource(func(query *store.SourceQuery) { query.WithPaths() }).
		AllModels(ctx)
	if err != nil {
		return nil, err
	}

	bindings := make([]Binding, 0, len(records))
	for _, record := range records {
		if record.Source == nil {
			continue
		}

		bindings = append(bindings, Binding{
			Source:  *record.Source,
			Library: *record,
		})
	}

	slices.SortFunc(bindings, func(first, second Binding) int {
		return strings.Compare(first.Source.Name, second.Source.Name)
	})

	return bindings, nil
}

func libraries(records []*LibrarySource) []LibrarySource {
	bound := make([]LibrarySource, len(records))
	for i, record := range records {
		bound[i] = *record
	}

	return bound
}

func (s *Service) Create(ctx context.Context, entry Configured) (Configured, error) {
	var id uuid.UUID
	err := s.store.WithTx(ctx, func(tx *store.Tx) error {
		if err := validate(ctx, tx, s.config.MediaDirectories, uuid.Nil, entry); err != nil {
			return err
		}

		created, err := tx.Source.Create().
			SetName(entry.Source.Name).
			SetURL(entry.Source.URL).
			SetAPIKeyVariable(entry.Source.APIKeyVariable).
			SetKind(entry.Source.Kind).
			SetNillableRequestProfileID(entry.Source.RequestProfileID).
			SetRequestRootFolder(entry.Source.RequestRootFolder).
			SetRequestTag(entry.Source.RequestTag).
			Save(ctx)
		if err != nil {
			return conflict(err)
		}
		id = created.ID

		if err := children(ctx, tx, id, entry); err != nil {
			return err
		}

		return relocate(ctx, tx, bound(entry))
	})
	if err != nil {
		return Configured{}, err
	}

	return s.Get(ctx, id)
}

func (s *Service) Update(ctx context.Context, id uuid.UUID, entry Configured) (Configured, error) {
	err := s.store.WithTx(ctx, func(tx *store.Tx) error {
		if err := validate(ctx, tx, s.config.MediaDirectories, id, entry); err != nil {
			return err
		}

		before, err := boundLibraries(ctx, tx, id)
		if err != nil {
			return err
		}

		update := tx.Source.UpdateOneID(id).
			SetName(entry.Source.Name).
			SetURL(entry.Source.URL).
			SetAPIKeyVariable(entry.Source.APIKeyVariable).
			SetKind(entry.Source.Kind).
			SetRequestRootFolder(entry.Source.RequestRootFolder).
			SetRequestTag(entry.Source.RequestTag)
		if entry.Source.RequestProfileID == nil {
			update.ClearRequestProfileID()
		} else {
			update.SetRequestProfileID(*entry.Source.RequestProfileID)
		}

		err = update.Exec(ctx)
		if store.IsNotFound(err) {
			return ErrNotFound
		}
		if err != nil {
			return conflict(err)
		}

		if _, err := tx.SourcePath.Delete().Where(sourcepathmodel.SourceID(id)).Exec(ctx); err != nil {
			return err
		}
		after := bound(entry)
		if _, err := tx.LibrarySource.Delete().
			Where(librarysourcemodel.SourceID(id), librarysourcemodel.LibraryIDNotIn(after...)).
			Exec(ctx); err != nil {
			return err
		}
		if _, err := tx.LibraryItem.Delete().
			Where(libraryitemmodel.SourceID(id), libraryitemmodel.LibraryIDNotIn(after...)).
			Exec(ctx); err != nil {
			return err
		}

		if err := children(ctx, tx, id, entry); err != nil {
			return err
		}

		return relocate(ctx, tx, append(before, after...))
	})
	if err != nil {
		return Configured{}, err
	}

	return s.Get(ctx, id)
}

func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	return s.store.WithTx(ctx, func(tx *store.Tx) error {
		before, err := boundLibraries(ctx, tx, id)
		if err != nil {
			return err
		}

		err = tx.Source.DeleteOneID(id).Exec(ctx)
		if store.IsNotFound(err) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}

		return relocate(ctx, tx, before)
	})
}

func bound(entry Configured) []uuid.UUID {
	ids := make([]uuid.UUID, len(entry.Libraries))
	for i, library := range entry.Libraries {
		ids[i] = library.LibraryID
	}

	return ids
}

func boundLibraries(ctx context.Context, tx *store.Tx, id uuid.UUID) ([]uuid.UUID, error) {
	bindings, err := tx.LibrarySource.Query().Where(librarysourcemodel.SourceID(id)).All(ctx)
	if err != nil {
		return nil, err
	}

	ids := make([]uuid.UUID, len(bindings))
	for i, binding := range bindings {
		ids[i] = binding.LibraryID
	}

	return ids, nil
}

func distinct(ids []uuid.UUID) []uuid.UUID {
	sorted := slices.Clone(ids)
	slices.SortFunc(sorted, func(one, other uuid.UUID) int { return strings.Compare(one.String(), other.String()) })

	return slices.Compact(sorted)
}

func relocate(ctx context.Context, tx *store.Tx, ids []uuid.UUID) error {
	for _, id := range distinct(ids) {
		folders, err := tx.SourcePath.Query().
			Where(sourcepathmodel.HasSourceWith(sourcemodel.HasLibrariesWith(librarysourcemodel.LibraryID(id)))).
			Select(sourcepathmodel.FieldLocalPath).
			Strings(ctx)
		if err != nil {
			return err
		}
		slices.Sort(folders)

		err = tx.Library.UpdateOneID(id).SetLocations(slices.Compact(folders)).Exec(ctx)
		if err != nil && !store.IsNotFound(err) {
			return err
		}
	}

	return nil
}

func children(ctx context.Context, tx *store.Tx, id uuid.UUID, entry Configured) error {
	for _, mapping := range entry.Source.Paths {
		if err := tx.SourcePath.Create().
			SetSourceID(id).
			SetRemotePath(filepath.Clean(mapping.RemotePath)).
			SetLocalPath(filepath.Clean(mapping.LocalPath)).
			Exec(ctx); err != nil {
			return conflict(err)
		}
	}

	for _, bound := range entry.Libraries {
		if err := tx.LibrarySource.Create().
			SetSourceID(id).
			SetLibraryID(bound.LibraryID).
			SetTagFilter(bound.TagFilter).
			OnConflictColumns(librarysourcemodel.FieldLibraryID, librarysourcemodel.FieldSourceID).
			UpdateTagFilter().
			UpdateUpdatedAt().
			Exec(ctx); err != nil {
			return conflict(err)
		}
	}

	return nil
}

func conflict(err error) error {
	if store.IsConstraintError(err) {
		return fmt.Errorf("%w: %w", ErrInvalid, err)
	}

	return err
}

func invalid(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalid, fmt.Sprintf(format, args...))
}

func validate(ctx context.Context, tx *store.Tx, roots []string, id uuid.UUID, entry Configured) error {
	if _, err := tx.ExecContext(ctx, "SELECT pg_advisory_xact_lock(hashtext('source_paths'))"); err != nil {
		return err
	}

	source := entry.Source
	if strings.TrimSpace(source.Name) == "" {
		return invalid("a source needs a name")
	}
	if err := checkURL(source.URL); err != nil {
		return invalid("%s: %v", source.Name, err)
	}
	if !env.ValidSourceAPIKeyVariable(source.APIKeyVariable) {
		return invalid(
			"%s names %q as its API key variable, which must begin with %s",
			source.Name, source.APIKeyVariable, env.SourceAPIKeyPrefix,
		)
	}
	if len(source.Paths) == 0 {
		return invalid("%s maps no paths, so nothing it reports can be opened", source.Name)
	}

	for i, mapping := range source.Paths {
		if !filepath.IsAbs(mapping.RemotePath) || !filepath.IsAbs(mapping.LocalPath) {
			return invalid("%s maps %q to %q: both paths must be absolute", source.Name, mapping.RemotePath, mapping.LocalPath)
		}

		for _, other := range source.Paths[i+1:] {
			if overlaps(mapping.RemotePath, other.RemotePath) {
				return invalid("%s maps %s twice: one remote folder has one place on this server", source.Name, mapping.RemotePath)
			}
			if overlaps(mapping.LocalPath, other.LocalPath) {
				return invalid("%s maps two folders onto %s", source.Name, mapping.LocalPath)
			}
		}
	}

	if err := validateRequests(source); err != nil {
		return err
	}

	for _, mapping := range source.Paths {
		if !slices.ContainsFunc(roots, func(root string) bool { return within(mapping.LocalPath, root) }) {
			return invalid("%s maps %s onto %s, which is outside MEDIA_DIRECTORIES", source.Name, mapping.RemotePath, mapping.LocalPath)
		}
	}

	if err := validateLibraries(ctx, tx, entry); err != nil {
		return err
	}

	claimed, err := tx.SourcePath.Query().
		Where(sourcepathmodel.SourceIDNEQ(id)).
		WithSource().
		AllModels(ctx)
	if err != nil {
		return err
	}

	for _, mapping := range source.Paths {
		for _, other := range claimed {
			if overlaps(mapping.LocalPath, other.LocalPath) {
				owner := ""
				if other.Source != nil {
					owner = other.Source.Name
				}

				return invalid(
					"%s and %s both claim %s: a downloader owns a directory of its own",
					source.Name, owner, other.LocalPath,
				)
			}
		}
	}

	return nil
}

func validateRequests(source Source) error {
	if source.RequestProfileID == nil {
		if source.RequestRootFolder != "" || source.RequestTag != "" {
			return invalid("%s names where a request goes but no quality profile to give it", source.Name)
		}

		return nil
	}

	if *source.RequestProfileID <= 0 {
		return invalid("%s names quality profile %d, which no instance defines", source.Name, *source.RequestProfileID)
	}
	if !slices.ContainsFunc(source.Paths, func(mapping *Path) bool { return within(source.RequestRootFolder, mapping.RemotePath) }) {
		return invalid(
			"%s adds requests under %q, which is in none of its folders: a requested title has to land where this server opens it",
			source.Name, source.RequestRootFolder,
		)
	}

	return nil
}

func validateLibraries(ctx context.Context, tx *store.Tx, entry Configured) error {
	ids := distinct(bound(entry))
	libraries, err := tx.Library.Query().Where(librarymodel.IDIn(ids...)).All(ctx)
	if err != nil {
		return err
	}
	if len(libraries) != len(ids) {
		return invalid("%s names a library that does not exist", entry.Source.Name)
	}

	for _, library := range libraries {
		if library.CollectionType != Fills(entry.Source.Kind) {
			return invalid("%s cannot fill %s: Radarr fills a movies library, Sonarr a shows one and Lidarr a music one", entry.Source.Name, library.Name)
		}
	}

	return nil
}

func Fills(kind Kind) librarymodel.CollectionType {
	switch kind {
	case KindSonarr:
		return librarymodel.CollectionTypeTvshows
	case KindLidarr:
		return librarymodel.CollectionTypeMusic
	default:
		return librarymodel.CollectionTypeMovies
	}
}

func overlaps(one, other string) bool {
	one = filepath.Clean(one)
	other = filepath.Clean(other)

	return one == other || within(one, other) || within(other, one)
}

func within(path, root string) bool {
	relative, err := filepath.Rel(root, path)

	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func checkURL(apiURL string) error {
	parsed, err := url.Parse(apiURL)
	if err != nil {
		return err
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("unsupported scheme %q", parsed.Scheme)
	}
	if parsed.Host == "" {
		return fmt.Errorf("no host in %q", apiURL)
	}

	return nil
}

type Instance struct {
	Version     string
	RootFolders []string
	Profiles    []arr.QualityProfile
	Tags        []string
}

func (s *Service) Test(ctx context.Context, kind Kind, apiURL, variable string) (Instance, error) {
	if err := checkURL(apiURL); err != nil {
		return Instance{}, err
	}

	source := Source{Kind: kind, URL: apiURL, APIKeyVariable: variable}

	status, err := fetch[arr.Status](ctx, s, s.client, source, arr.StatusPath, nil)
	if err != nil {
		return Instance{}, err
	}

	folders, err := fetch[[]arr.RootFolder](ctx, s, s.client, source, arr.RootFolderPath, nil)
	if err != nil {
		return Instance{}, err
	}

	profiles, err := fetch[[]arr.QualityProfile](ctx, s, s.client, source, arr.ProfilesPath, nil)
	if err != nil {
		return Instance{}, err
	}

	tags, err := fetch[[]arr.Tag](ctx, s, s.client, source, arr.TagsPath, nil)
	if err != nil {
		return Instance{}, err
	}

	found := Instance{
		Version:     status.Version,
		RootFolders: make([]string, len(folders)),
		Profiles:    profiles,
		Tags:        make([]string, len(tags)),
	}
	for i, folder := range folders {
		found.RootFolders[i] = folder.Path
	}
	for i, tag := range tags {
		found.Tags[i] = tag.Label
	}

	return found, nil
}

func (s *Service) contact(ctx context.Context, source Source) error {
	status, err := fetch[arr.Status](ctx, s, s.client, source, arr.StatusPath, nil)

	update := s.store.Source.UpdateOneID(source.ID)
	if err != nil {
		update.SetLastError(err.Error())
	} else {
		update.SetVersion(status.Version).SetContactedAt(time.Now()).SetLastError("")
	}

	if saveErr := update.Exec(ctx); saveErr != nil {
		return errors.Join(err, saveErr)
	}

	return err
}
