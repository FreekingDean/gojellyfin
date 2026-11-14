package migration

import (
	"context"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

const (
	usersDatabase   = "jellyfin.db"
	libraryDatabase = "library.db"
	serverIDFile    = "device.txt"
)

type TitleKind string

const (
	KindMovie   TitleKind = "Movie"
	KindSeries  TitleKind = "Series"
	KindSeason  TitleKind = "Season"
	KindEpisode TitleKind = "Episode"
)

var titleKinds = map[string]TitleKind{
	"MediaBrowser.Controller.Entities.Movies.Movie": KindMovie,
	"MediaBrowser.Controller.Entities.TV.Series":    KindSeries,
	"MediaBrowser.Controller.Entities.TV.Season":    KindSeason,
	"MediaBrowser.Controller.Entities.TV.Episode":   KindEpisode,
}

var timeLayouts = []string{
	"2006-01-02 15:04:05.9999999Z07:00",
	"2006-01-02 15:04:05.9999999",
	time.RFC3339Nano,
	"2006-01-02T15:04:05.9999999",
}

type Source struct {
	ServerID string
	Users    []User
	Devices  []Device
	APIKeys  []APIKey
	Watched  []Watched
	Orphaned int
}

type User struct {
	ID           uuid.UUID
	Username     string
	PasswordHash string
	Permissions  map[int]bool

	CastReceiverID             string
	AudioLanguagePreference    string
	SubtitleLanguagePreference string
	SubtitleMode               int
	SyncPlayAccess             int
	PlayDefaultAudioTrack      bool
	DisplayMissingEpisodes     bool
	DisplayCollectionsView     bool
	EnableLocalPassword        bool
	HidePlayedInLatest         bool
	RememberAudioSelections    bool
	RememberSubtitleSelections bool
	EnableNextEpisodeAutoPlay  bool
	EnableUserPreferenceAccess bool

	InvalidLoginAttemptCount   int32
	LoginAttemptsBeforeLockout *int32
	MaxActiveSessions          int32
	RemoteClientBitrateLimit   *int32

	LastLoginAt    *time.Time
	LastActivityAt *time.Time
}

type Device struct {
	UserID         uuid.UUID
	Token          string
	ClientID       string
	Name           string
	AppName        string
	AppVersion     string
	LastActivityAt *time.Time
}

type APIKey struct {
	Name  string
	Token string
}

type Title struct {
	Kind      TitleKind
	Name      string
	Path      string
	Providers map[string]string
	Series    *Title

	Index       *int32
	ParentIndex *int32
}

type Watched struct {
	UserID                uuid.UUID
	Title                 *Title
	Played                bool
	IsFavorite            bool
	PlayCount             int32
	PlaybackPositionTicks int64
	LastPlayedAt          *time.Time
	Rating                *float64
	Likes                 *bool
}

func (w Watched) empty() bool {
	return !w.Played && !w.IsFavorite && w.PlayCount == 0 && w.PlaybackPositionTicks == 0 && w.Rating == nil && w.Likes == nil
}

func Read(ctx context.Context, directory string) (*Source, error) {
	path := filepath.Join(directory, usersDatabase)
	if _, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("no %s under %q, which is the directory Jellyfin keeps its databases in: %w", usersDatabase, directory, err)
	}

	db, err := openReadOnly(ctx, path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = db.Close() }()

	source := &Source{ServerID: readServerID(directory)}

	if source.Users, err = readUsers(ctx, db); err != nil {
		return nil, err
	}
	if source.Devices, err = readDevices(ctx, db); err != nil {
		return nil, err
	}
	if source.APIKeys, err = readAPIKeys(ctx, db); err != nil {
		return nil, err
	}

	consolidated, err := hasTable(ctx, db, "BaseItems")
	if err != nil {
		return nil, err
	}
	if consolidated {
		source.Watched, source.Orphaned, err = readConsolidatedWatched(ctx, db)

		return source, err
	}

	internal, err := readInternalIDs(ctx, db)
	if err != nil {
		return nil, err
	}

	libraryPath := filepath.Join(directory, libraryDatabase)
	if _, err := os.Stat(libraryPath); err != nil {
		return nil, fmt.Errorf("%s holds no items and there is no %s beside it to read watch state from: %w", usersDatabase, libraryDatabase, err)
	}
	library, err := openReadOnly(ctx, libraryPath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = library.Close() }()

	source.Watched, source.Orphaned, err = readLegacyWatched(ctx, library, internal)

	return source, err
}

func openReadOnly(ctx context.Context, path string) (*sql.DB, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve %q: %w", path, err)
	}

	db, err := sql.Open("sqlite", "file:"+absolute+"?mode=ro&_pragma=query_only(1)")
	if err != nil {
		return nil, fmt.Errorf("failed to open %q: %w", path, err)
	}
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()

		return nil, fmt.Errorf("failed to read %q: %w", path, err)
	}

	return db, nil
}

func readServerID(directory string) string {
	content, err := os.ReadFile(filepath.Join(directory, serverIDFile))
	if err != nil {
		return ""
	}

	return strings.ToLower(strings.TrimSpace(string(content)))
}

func hasTable(ctx context.Context, db *sql.DB, name string) (bool, error) {
	var found int
	err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?`, name).Scan(&found)
	if err != nil {
		return false, fmt.Errorf("failed to look for the %s table: %w", name, err)
	}

	return found > 0, nil
}

func readUsers(ctx context.Context, db *sql.DB) ([]User, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT Id, Username, COALESCE(Password, ''), COALESCE(CastReceiverId, ''),
		       COALESCE(AudioLanguagePreference, ''), COALESCE(SubtitleLanguagePreference, ''),
		       SubtitleMode, SyncPlayAccess, PlayDefaultAudioTrack, DisplayMissingEpisodes,
		       DisplayCollectionsView, EnableLocalPassword, HidePlayedInLatest,
		       RememberAudioSelections, RememberSubtitleSelections, EnableNextEpisodeAutoPlay,
		       EnableUserPreferenceAccess, InvalidLoginAttemptCount, LoginAttemptsBeforeLockout,
		       MaxActiveSessions, RemoteClientBitrateLimit, LastLoginDate, LastActivityDate
		FROM Users
		ORDER BY Username`)
	if err != nil {
		return nil, fmt.Errorf("failed to read the users: %w", err)
	}
	defer func() { _ = rows.Close() }()

	found := make([]User, 0)
	for rows.Next() {
		var user User
		var id string
		var lastLogin, lastActivity any

		err := rows.Scan(&id, &user.Username, &user.PasswordHash, &user.CastReceiverID,
			&user.AudioLanguagePreference, &user.SubtitleLanguagePreference,
			&user.SubtitleMode, &user.SyncPlayAccess, &user.PlayDefaultAudioTrack,
			&user.DisplayMissingEpisodes, &user.DisplayCollectionsView,
			&user.EnableLocalPassword, &user.HidePlayedInLatest,
			&user.RememberAudioSelections, &user.RememberSubtitleSelections,
			&user.EnableNextEpisodeAutoPlay, &user.EnableUserPreferenceAccess,
			&user.InvalidLoginAttemptCount, &user.LoginAttemptsBeforeLockout,
			&user.MaxActiveSessions, &user.RemoteClientBitrateLimit, &lastLogin, &lastActivity)
		if err != nil {
			return nil, fmt.Errorf("failed to read a user: %w", err)
		}

		user.ID, err = uuid.Parse(id)
		if err != nil {
			return nil, fmt.Errorf("failed to read the id of %q: %w", user.Username, err)
		}
		user.LastLoginAt = timeOf(lastLogin)
		user.LastActivityAt = timeOf(lastActivity)

		found = append(found, user)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to read the users: %w", err)
	}

	permissions, err := readPermissions(ctx, db)
	if err != nil {
		return nil, err
	}
	for index := range found {
		found[index].Permissions = permissions[found[index].ID]
	}

	return found, nil
}

func readPermissions(ctx context.Context, db *sql.DB) (map[uuid.UUID]map[int]bool, error) {
	rows, err := db.QueryContext(ctx, `SELECT UserId, Kind, Value FROM Permissions WHERE UserId IS NOT NULL`)
	if err != nil {
		return nil, fmt.Errorf("failed to read the permissions: %w", err)
	}
	defer func() { _ = rows.Close() }()

	permissions := map[uuid.UUID]map[int]bool{}
	for rows.Next() {
		var owner string
		var kind int
		var value bool

		if err := rows.Scan(&owner, &kind, &value); err != nil {
			return nil, fmt.Errorf("failed to read a permission: %w", err)
		}

		id, err := uuid.Parse(owner)
		if err != nil {
			return nil, fmt.Errorf("failed to read the user of a permission: %w", err)
		}
		if permissions[id] == nil {
			permissions[id] = map[int]bool{}
		}
		permissions[id][kind] = value
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to read the permissions: %w", err)
	}

	return permissions, nil
}

func readInternalIDs(ctx context.Context, db *sql.DB) (map[int64]uuid.UUID, error) {
	rows, err := db.QueryContext(ctx, `SELECT InternalId, Id FROM Users`)
	if err != nil {
		return nil, fmt.Errorf("failed to read the users' internal ids: %w", err)
	}
	defer func() { _ = rows.Close() }()

	internal := map[int64]uuid.UUID{}
	for rows.Next() {
		var number int64
		var id string

		if err := rows.Scan(&number, &id); err != nil {
			return nil, fmt.Errorf("failed to read a user's internal id: %w", err)
		}
		parsed, err := uuid.Parse(id)
		if err != nil {
			return nil, fmt.Errorf("failed to read the id of user %d: %w", number, err)
		}
		internal[number] = parsed
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to read the users' internal ids: %w", err)
	}

	return internal, nil
}

func readDevices(ctx context.Context, db *sql.DB) ([]Device, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT UserId, AccessToken, DeviceId, DeviceName, AppName, AppVersion, DateLastActivity
		FROM Devices
		WHERE IsActive = 1
		ORDER BY DateLastActivity, Id`)
	if err != nil {
		return nil, fmt.Errorf("failed to read the devices: %w", err)
	}
	defer func() { _ = rows.Close() }()

	devices := make([]Device, 0)
	for rows.Next() {
		var device Device
		var owner string
		var lastActivity any

		err := rows.Scan(&owner, &device.Token, &device.ClientID, &device.Name,
			&device.AppName, &device.AppVersion, &lastActivity)
		if err != nil {
			return nil, fmt.Errorf("failed to read a device: %w", err)
		}

		device.UserID, err = uuid.Parse(owner)
		if err != nil {
			return nil, fmt.Errorf("failed to read the user of the device %q: %w", device.Name, err)
		}
		device.LastActivityAt = timeOf(lastActivity)

		devices = append(devices, device)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to read the devices: %w", err)
	}

	return devices, nil
}

func readAPIKeys(ctx context.Context, db *sql.DB) ([]APIKey, error) {
	rows, err := db.QueryContext(ctx, `SELECT Name, AccessToken FROM ApiKeys ORDER BY Id`)
	if err != nil {
		return nil, fmt.Errorf("failed to read the api keys: %w", err)
	}
	defer func() { _ = rows.Close() }()

	keys := make([]APIKey, 0)
	for rows.Next() {
		var key APIKey
		if err := rows.Scan(&key.Name, &key.Token); err != nil {
			return nil, fmt.Errorf("failed to read an api key: %w", err)
		}
		keys = append(keys, key)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to read the api keys: %w", err)
	}

	return keys, nil
}

func readConsolidatedWatched(ctx context.Context, db *sql.DB) ([]Watched, int, error) {
	titles, err := readConsolidatedTitles(ctx, db)
	if err != nil {
		return nil, 0, err
	}

	rows, err := db.QueryContext(ctx, `
		SELECT ItemId, UserId, Played, PlayCount, IsFavorite, PlaybackPositionTicks, LastPlayedDate, Rating, Likes
		FROM UserData`)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to read the watch state: %w", err)
	}
	defer func() { _ = rows.Close() }()

	collected := newCollector()
	for rows.Next() {
		var item, owner string
		var watched Watched
		var lastPlayed any
		var rating sql.NullFloat64
		var likes sql.NullBool

		err := rows.Scan(&item, &owner, &watched.Played, &watched.PlayCount, &watched.IsFavorite,
			&watched.PlaybackPositionTicks, &lastPlayed, &rating, &likes)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to read a watch state row: %w", err)
		}

		watched.UserID, err = uuid.Parse(owner)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to read the user of a watch state row: %w", err)
		}
		watched.LastPlayedAt = timeOf(lastPlayed)
		watched.Rating = nullable(rating.Float64, rating.Valid)
		watched.Likes = nullable(likes.Bool, likes.Valid)

		collected.add(watched, titles[strings.ToLower(item)])
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("failed to read the watch state: %w", err)
	}

	return collected.watched, collected.orphaned, nil
}

func readConsolidatedTitles(ctx context.Context, db *sql.DB) (map[string]*Title, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT Id, Type, COALESCE(Path, ''), COALESCE(Name, ''), IndexNumber, ParentIndexNumber, COALESCE(SeriesId, '')
		FROM BaseItems`)
	if err != nil {
		return nil, fmt.Errorf("failed to read the items: %w", err)
	}
	defer func() { _ = rows.Close() }()

	titles := map[string]*Title{}
	series := map[*Title]string{}
	for rows.Next() {
		var id, kind, parent string
		title := &Title{Providers: map[string]string{}}

		if err := rows.Scan(&id, &kind, &title.Path, &title.Name, &title.Index, &title.ParentIndex, &parent); err != nil {
			return nil, fmt.Errorf("failed to read an item: %w", err)
		}
		known, ok := titleKinds[kind]
		if !ok {
			continue
		}
		title.Kind = known
		titles[strings.ToLower(id)] = title
		series[title] = strings.ToLower(parent)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to read the items: %w", err)
	}
	for title, parent := range series {
		title.Series = titles[parent]
	}

	providers, err := db.QueryContext(ctx, `SELECT ItemId, ProviderId, ProviderValue FROM BaseItemProviders`)
	if err != nil {
		return nil, fmt.Errorf("failed to read the provider ids: %w", err)
	}
	defer func() { _ = providers.Close() }()

	for providers.Next() {
		var id, provider, value string
		if err := providers.Scan(&id, &provider, &value); err != nil {
			return nil, fmt.Errorf("failed to read a provider id: %w", err)
		}
		if title, ok := titles[strings.ToLower(id)]; ok {
			title.Providers[strings.ToLower(provider)] = value
		}
	}
	if err := providers.Err(); err != nil {
		return nil, fmt.Errorf("failed to read the provider ids: %w", err)
	}

	return titles, nil
}

func readLegacyWatched(ctx context.Context, library *sql.DB, internal map[int64]uuid.UUID) ([]Watched, int, error) {
	keys, err := readLegacyTitles(ctx, library)
	if err != nil {
		return nil, 0, err
	}

	rows, err := library.QueryContext(ctx, `
		SELECT key, userId, played, playCount, isFavorite, playbackPositionTicks, lastPlayedDate, rating
		FROM UserDatas`)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to read the watch state: %w", err)
	}
	defer func() { _ = rows.Close() }()

	collected := newCollector()
	for rows.Next() {
		var key string
		var owner int64
		var watched Watched
		var lastPlayed any
		var rating sql.NullFloat64

		err := rows.Scan(&key, &owner, &watched.Played, &watched.PlayCount, &watched.IsFavorite,
			&watched.PlaybackPositionTicks, &lastPlayed, &rating)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to read a watch state row: %w", err)
		}

		user, ok := internal[owner]
		if !ok {
			continue
		}
		watched.UserID = user
		watched.LastPlayedAt = timeOf(lastPlayed)
		watched.Rating = nullable(rating.Float64, rating.Valid)

		titles := keys[key]
		if len(titles) == 0 {
			collected.add(watched, nil)
		}
		for _, title := range titles {
			collected.add(watched, title)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("failed to read the watch state: %w", err)
	}

	return collected.watched, collected.orphaned, nil
}

func readLegacyTitles(ctx context.Context, library *sql.DB) (map[string][]*Title, error) {
	rows, err := library.QueryContext(ctx, `
		SELECT guid, type, COALESCE(Path, ''), COALESCE(Name, ''), COALESCE(ProviderIds, ''),
		       IndexNumber, ParentIndexNumber, SeriesId, COALESCE(UserDataKey, '')
		FROM TypedBaseItems`)
	if err != nil {
		return nil, fmt.Errorf("failed to read the items: %w", err)
	}
	defer func() { _ = rows.Close() }()

	byID := map[uuid.UUID]*Title{}
	series := map[*Title]uuid.UUID{}
	keys := map[string][]*Title{}
	for rows.Next() {
		var id, parent any
		var kind, providers, key string
		title := &Title{}

		if err := rows.Scan(&id, &kind, &title.Path, &title.Name, &providers, &title.Index, &title.ParentIndex, &parent, &key); err != nil {
			return nil, fmt.Errorf("failed to read an item: %w", err)
		}
		known, ok := titleKinds[kind]
		if !ok {
			continue
		}
		title.Kind = known
		title.Providers = parseProviders(providers)

		guid, ok := guidOf(id)
		if !ok {
			continue
		}
		byID[guid] = title
		if seriesID, ok := guidOf(parent); ok {
			series[title] = seriesID
		}

		own := hex.EncodeToString(guid[:])
		keys[own] = append(keys[own], title)
		if key != "" && key != own {
			keys[key] = append(keys[key], title)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to read the items: %w", err)
	}
	for title, parent := range series {
		title.Series = byID[parent]
	}

	return keys, nil
}

type collector struct {
	watched  []Watched
	index    map[uuid.UUID]map[*Title]int
	orphaned int
}

func newCollector() *collector {
	return &collector{index: map[uuid.UUID]map[*Title]int{}}
}

func (c *collector) add(watched Watched, title *Title) {
	if watched.empty() {
		return
	}
	if title == nil {
		c.orphaned++

		return
	}
	watched.Title = title

	if c.index[watched.UserID] == nil {
		c.index[watched.UserID] = map[*Title]int{}
	}
	position, seen := c.index[watched.UserID][title]
	if !seen {
		c.index[watched.UserID][title] = len(c.watched)
		c.watched = append(c.watched, watched)

		return
	}
	c.watched[position] = further(c.watched[position], watched)
}

func further(a, b Watched) Watched {
	a.Played = a.Played || b.Played
	a.IsFavorite = a.IsFavorite || b.IsFavorite
	a.PlayCount = max(a.PlayCount, b.PlayCount)
	a.PlaybackPositionTicks = max(a.PlaybackPositionTicks, b.PlaybackPositionTicks)
	a.LastPlayedAt = later(a.LastPlayedAt, b.LastPlayedAt)
	if a.Rating == nil {
		a.Rating = b.Rating
	}
	if a.Likes == nil {
		a.Likes = b.Likes
	}

	return a
}

func later(a, b *time.Time) *time.Time {
	if a == nil || (b != nil && b.After(*a)) {
		return b
	}

	return a
}

func parseProviders(serialized string) map[string]string {
	providers := map[string]string{}
	for _, pair := range strings.Split(serialized, "|") {
		name, value, found := strings.Cut(pair, "=")
		if found && value != "" {
			providers[strings.ToLower(name)] = value
		}
	}

	return providers
}

func guidOf(value any) (uuid.UUID, bool) {
	switch stored := value.(type) {
	case []byte:
		if len(stored) == len(uuid.UUID{}) {
			return uuid.UUID{
				stored[3], stored[2], stored[1], stored[0],
				stored[5], stored[4],
				stored[7], stored[6],
				stored[8], stored[9], stored[10], stored[11], stored[12], stored[13], stored[14], stored[15],
			}, true
		}

		return guidOf(string(stored))
	case string:
		parsed, err := uuid.Parse(stored)

		return parsed, err == nil
	}

	return uuid.UUID{}, false
}

func timeOf(value any) *time.Time {
	switch stored := value.(type) {
	case time.Time:
		utc := stored.UTC()

		return &utc
	case []byte:
		return timeOf(string(stored))
	case string:
		for _, layout := range timeLayouts {
			if parsed, err := time.Parse(layout, stored); err == nil {
				utc := parsed.UTC()

				return &utc
			}
		}
	}

	return nil
}

func nullable[T any](value T, valid bool) *T {
	if !valid {
		return nil
	}

	return &value
}
