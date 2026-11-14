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

	configured := make([]Configured, len(records))
	for i, record := range records {
		configured[i] = fromRecord(record)
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

	return fromRecord(record), nil
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
		if err := validate(ctx, tx, uuid.Nil, entry); err != nil {
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

		return children(ctx, tx, id, entry)
	})
	if err != nil {
		return Configured{}, err
	}

	return s.Get(ctx, id)
}

func (s *Service) Update(ctx context.Context, id uuid.UUID, entry Configured) (Configured, error) {
	err := s.store.WithTx(ctx, func(tx *store.Tx) error {
		if err := validate(ctx, tx, id, entry); err != nil {
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

		err := update.Exec(ctx)
		if store.IsNotFound(err) {
			return ErrNotFound
		}
		if err != nil {
			return conflict(err)
		}

		if _, err := tx.SourcePath.Delete().Where(sourcepathmodel.SourceID(id)).Exec(ctx); err != nil {
			return err
		}
		bound := make([]uuid.UUID, len(entry.Libraries))
		for i, library := range entry.Libraries {
			bound[i] = library.LibraryID
		}
		if _, err := tx.LibrarySource.Delete().
			Where(librarysourcemodel.SourceID(id), librarysourcemodel.LibraryIDNotIn(bound...)).
			Exec(ctx); err != nil {
			return err
		}
		if _, err := tx.LibraryItem.Delete().
			Where(libraryitemmodel.SourceID(id), libraryitemmodel.LibraryIDNotIn(bound...)).
			Exec(ctx); err != nil {
			return err
		}

		return children(ctx, tx, id, entry)
	})
	if err != nil {
		return Configured{}, err
	}

	return s.Get(ctx, id)
}

func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	err := s.store.Source.DeleteOneID(id).Exec(ctx)
	if store.IsNotFound(err) {
		return ErrNotFound
	}

	return err
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

func validate(ctx context.Context, tx *store.Tx, id uuid.UUID, entry Configured) error {
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

	folders, err := libraryFolders(ctx, tx)
	if err != nil {
		return err
	}
	for _, mapping := range source.Paths {
		if !slices.ContainsFunc(folders, func(folder string) bool { return within(mapping.LocalPath, folder) }) {
			return invalid("%s maps %s onto %s, which is in no library's folders: add it to a library first", source.Name, mapping.RemotePath, mapping.LocalPath)
		}
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

func libraryFolders(ctx context.Context, tx *store.Tx) ([]string, error) {
	libraries, err := tx.Library.Query().Select(librarymodel.FieldLocations).All(ctx)
	if err != nil {
		return nil, err
	}

	folders := []string{}
	for _, library := range libraries {
		folders = append(folders, library.Locations...)
	}

	return folders, nil
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

func (s *Service) Test(ctx context.Context, apiURL, variable string) (Instance, error) {
	if err := checkURL(apiURL); err != nil {
		return Instance{}, err
	}

	source := Source{URL: apiURL, APIKeyVariable: variable}

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
