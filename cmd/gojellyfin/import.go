package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
	_ "modernc.org/sqlite"

	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
	"github.com/FreekingDean/gojellyfin/internal/sessions"
	"github.com/FreekingDean/gojellyfin/internal/store"
	"github.com/FreekingDean/gojellyfin/internal/users"
)

const jellyfinDatabase = "jellyfin.db"

const (
	permissionIsAdministrator = iota
	permissionIsHidden
	permissionIsDisabled
	permissionEnableSharedDeviceControl
	permissionEnableRemoteAccess
	permissionEnableLiveTvManagement
	permissionEnableLiveTvAccess
	permissionEnableMediaPlayback
	permissionEnableAudioPlaybackTranscoding
	permissionEnableVideoPlaybackTranscoding
	permissionEnableContentDeletion
	permissionEnableContentDownloading
	permissionEnableSyncTranscoding
	permissionEnableMediaConversion
	permissionEnableAllDevices
	permissionEnableAllChannels
	permissionEnableAllFolders
	permissionEnablePublicSharing
	permissionEnableRemoteControlOfOtherUsers
	permissionEnablePlaybackRemuxing
	permissionForceRemoteSourceTranscoding
	permissionEnableCollectionManagement
	permissionEnableSubtitleManagement
	permissionEnableLyricManagement
)

var (
	subtitleModes   = []users.SubtitleMode{"Default", "Always", "OnlyForced", "None", "Smart"}
	syncPlayAccess  = []users.SyncPlayAccess{"CreateAndJoinGroups", "JoinGroups", "None"}
	jellyfinLayouts = []string{
		"2006-01-02 15:04:05.999999999",
		"2006-01-02T15:04:05.999999999",
		time.RFC3339Nano,
	}
)

func importCommand() *cobra.Command {
	var (
		from       string
		dryRun     bool
		activeDays int
	)

	command := &cobra.Command{
		Use:   "import",
		Short: "Import users, devices and sessions from a Jellyfin data directory",
		Long: "Reads jellyfin.db out of a Jellyfin data directory and writes its users,\n" +
			"their devices and their access tokens here. The file is opened read only\n" +
			"and never written to.\n\n" +
			"A user is matched by username, a device by its client id and a session by\n" +
			"its access token, so running this twice imports nothing twice. A user who\n" +
			"already exists here is left exactly as they are and their devices are\n" +
			"skipped, because a name that matches is not proof the two accounts are\n" +
			"the same person and a carried token would sign one in as the other.\n\n" +
			"An imported token keeps working, which is what keeps a signed in\n" +
			"television signed in — and means a token taken from the old install signs\n" +
			"in here too. Jellyfin expires none of them, so --active-days is where\n" +
			"that line gets drawn.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			path := filepath.Join(from, jellyfinDatabase)
			if _, err := os.Stat(path); err != nil {
				return fmt.Errorf("no %s under %q: --from wants a Jellyfin data directory: %w", jellyfinDatabase, from, err)
			}

			source, err := openReadOnly(path)
			if err != nil {
				return err
			}
			defer func() { _ = source.Close() }()

			return withStore(func(client *store.Client) error {
				report, err := runImport(cmd.Context(), source, client, activeDays, dryRun)
				if err != nil {
					return err
				}
				fmt.Println(strings.Join(report.lines(dryRun), "\n"))

				return nil
			})
		},
	}

	command.Flags().StringVar(&from, "from", "", "the Jellyfin data directory holding "+jellyfinDatabase)
	command.Flags().BoolVar(&dryRun, "dry-run", false, "report what would be imported without writing anything")
	command.Flags().IntVar(&activeDays, "active-days", 0, "skip devices last seen more than this many days ago, 0 for all of them")
	_ = command.MarkFlagRequired("from")

	return command
}

func openReadOnly(path string) (*sql.DB, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve %q: %w", path, err)
	}

	db, err := sql.Open("sqlite", "file:"+absolute+"?mode=ro&_pragma=query_only(1)")
	if err != nil {
		return nil, fmt.Errorf("failed to open %q: %w", path, err)
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()

		return nil, fmt.Errorf("failed to read %q: %w", path, err)
	}

	return db, nil
}

type jellyfinUser struct {
	id           uuid.UUID
	username     string
	passwordHash string
	permissions  map[int]bool

	castReceiverID             string
	audioLanguagePreference    string
	subtitleLanguagePreference string
	subtitleMode               int
	syncPlayAccess             int
	playDefaultAudioTrack      bool
	displayMissingEpisodes     bool
	displayCollectionsView     bool
	enableLocalPassword        bool
	hidePlayedInLatest         bool
	rememberAudioSelections    bool
	rememberSubtitleSelections bool
	enableNextEpisodeAutoPlay  bool
	enableUserPreferenceAccess bool

	invalidLoginAttemptCount   int32
	loginAttemptsBeforeLockout *int32
	maxActiveSessions          int32
	remoteClientBitrateLimit   *int32
}

func (u jellyfinUser) permission(kind int) *bool {
	value, ok := u.permissions[kind]
	if !ok {
		return nil
	}

	return &value
}

type jellyfinDevice struct {
	userID       uuid.UUID
	token        string
	info         sessions.DeviceInfo
	lastActivity time.Time
}

type importReport struct {
	created      []string
	existing     []string
	needingReset []string
	sessions     int
	stale        int
	claimed      []string
	orphaned     []string
}

func runImport(ctx context.Context, source *sql.DB, client *store.Client, activeDays int, dryRun bool) (*importReport, error) {
	found, err := readUsers(ctx, source)
	if err != nil {
		return nil, err
	}
	devices, err := readDevices(ctx, source)
	if err != nil {
		return nil, err
	}

	service := users.New(client)
	present, err := service.Users(ctx)
	if err != nil {
		return nil, err
	}
	byUsername := make(map[string]bool, len(present))
	for _, user := range present {
		byUsername[strings.ToLower(user.Username)] = true
	}

	report := &importReport{}
	imported := make(map[uuid.UUID]uuid.UUID, len(found))
	claimed := make(map[uuid.UUID]bool, len(found))

	for _, user := range found {
		if byUsername[strings.ToLower(user.username)] {
			report.existing = append(report.existing, user.username)
			claimed[user.id] = true

			continue
		}

		report.created = append(report.created, user.username)
		if user.passwordHash == "" {
			report.needingReset = append(report.needingReset, user.username)
		}
		if dryRun {
			imported[user.id] = uuid.Nil

			continue
		}

		id, err := createUser(ctx, service, user)
		if err != nil {
			return nil, err
		}
		imported[user.id] = id
	}

	cutoff := time.Time{}
	if activeDays > 0 {
		cutoff = time.Now().AddDate(0, 0, -activeDays)
	}

	sessionService := sessions.New(client)
	for _, device := range devices {
		if device.lastActivity.Before(cutoff) {
			report.stale++

			continue
		}
		if claimed[device.userID] {
			report.claimed = append(report.claimed, device.info.Name)

			continue
		}

		id, ok := imported[device.userID]
		if !ok {
			report.orphaned = append(report.orphaned, device.info.Name)

			continue
		}

		report.sessions++
		if dryRun {
			continue
		}

		if err := sessionService.Import(ctx, id, device.token, device.info, device.lastActivity); err != nil {
			return nil, err
		}
	}

	return report, nil
}

func createUser(ctx context.Context, service *users.Service, user jellyfinUser) (uuid.UUID, error) {
	created, err := service.CreateUser(ctx, user.username, user.passwordHash, user.permissions[permissionIsAdministrator])
	if err != nil {
		return uuid.Nil, err
	}

	err = service.UpdatePolicy(created.ID).
		SetNillableIsAdministrator(user.permission(permissionIsAdministrator)).
		SetNillableIsHidden(user.permission(permissionIsHidden)).
		SetNillableIsDisabled(user.permission(permissionIsDisabled)).
		SetNillableEnableSharedDeviceControl(user.permission(permissionEnableSharedDeviceControl)).
		SetNillableEnableRemoteAccess(user.permission(permissionEnableRemoteAccess)).
		SetNillableEnableLiveTvManagement(user.permission(permissionEnableLiveTvManagement)).
		SetNillableEnableLiveTvAccess(user.permission(permissionEnableLiveTvAccess)).
		SetNillableEnableMediaPlayback(user.permission(permissionEnableMediaPlayback)).
		SetNillableEnableAudioPlaybackTranscoding(user.permission(permissionEnableAudioPlaybackTranscoding)).
		SetNillableEnableVideoPlaybackTranscoding(user.permission(permissionEnableVideoPlaybackTranscoding)).
		SetNillableEnableContentDeletion(user.permission(permissionEnableContentDeletion)).
		SetNillableEnableContentDownloading(user.permission(permissionEnableContentDownloading)).
		SetNillableEnableSyncTranscoding(user.permission(permissionEnableSyncTranscoding)).
		SetNillableEnableMediaConversion(user.permission(permissionEnableMediaConversion)).
		SetNillableEnableAllDevices(user.permission(permissionEnableAllDevices)).
		SetNillableEnableAllChannels(user.permission(permissionEnableAllChannels)).
		SetNillableEnableAllFolders(user.permission(permissionEnableAllFolders)).
		SetNillableEnablePublicSharing(user.permission(permissionEnablePublicSharing)).
		SetNillableEnableRemoteControlOfOtherUsers(user.permission(permissionEnableRemoteControlOfOtherUsers)).
		SetNillableEnablePlaybackRemuxing(user.permission(permissionEnablePlaybackRemuxing)).
		SetNillableForceRemoteSourceTranscoding(user.permission(permissionForceRemoteSourceTranscoding)).
		SetNillableEnableCollectionManagement(user.permission(permissionEnableCollectionManagement)).
		SetNillableEnableSubtitleManagement(user.permission(permissionEnableSubtitleManagement)).
		SetNillableEnableLyricManagement(user.permission(permissionEnableLyricManagement)).
		SetEnableUserPreferenceAccess(user.enableUserPreferenceAccess).
		SetInvalidLoginAttemptCount(user.invalidLoginAttemptCount).
		SetNillableLoginAttemptsBeforeLockout(user.loginAttemptsBeforeLockout).
		SetMaxActiveSessions(user.maxActiveSessions).
		SetNillableRemoteClientBitrateLimit(user.remoteClientBitrateLimit).
		SetNillableSyncPlayAccess(ordinal(syncPlayAccess, user.syncPlayAccess)).
		Exec(ctx)
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to import the policy of %q: %w", user.username, err)
	}

	err = service.UpdateConfiguration(created.ID).
		SetCastReceiverID(user.castReceiverID).
		SetAudioLanguagePreference(user.audioLanguagePreference).
		SetSubtitleLanguagePreference(user.subtitleLanguagePreference).
		SetNillableSubtitleMode(ordinal(subtitleModes, user.subtitleMode)).
		SetPlayDefaultAudioTrack(user.playDefaultAudioTrack).
		SetDisplayMissingEpisodes(user.displayMissingEpisodes).
		SetDisplayCollectionsView(user.displayCollectionsView).
		SetEnableLocalPassword(user.enableLocalPassword).
		SetHidePlayedInLatest(user.hidePlayedInLatest).
		SetRememberAudioSelections(user.rememberAudioSelections).
		SetRememberSubtitleSelections(user.rememberSubtitleSelections).
		SetEnableNextEpisodeAutoPlay(user.enableNextEpisodeAutoPlay).
		Exec(ctx)
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to import the configuration of %q: %w", user.username, err)
	}

	return created.ID, nil
}

func ordinal[T any](values []T, index int) *T {
	if index < 0 || index >= len(values) {
		return nil
	}

	return &values[index]
}

func readUsers(ctx context.Context, source *sql.DB) ([]jellyfinUser, error) {
	rows, err := source.QueryContext(ctx, `
		SELECT Id, Username, COALESCE(Password, ''), COALESCE(CastReceiverId, ''),
		       COALESCE(AudioLanguagePreference, ''), COALESCE(SubtitleLanguagePreference, ''),
		       SubtitleMode, SyncPlayAccess, PlayDefaultAudioTrack, DisplayMissingEpisodes,
		       DisplayCollectionsView, EnableLocalPassword, HidePlayedInLatest,
		       RememberAudioSelections, RememberSubtitleSelections, EnableNextEpisodeAutoPlay,
		       EnableUserPreferenceAccess, InvalidLoginAttemptCount, LoginAttemptsBeforeLockout,
		       MaxActiveSessions, RemoteClientBitrateLimit
		FROM Users`)
	if err != nil {
		return nil, fmt.Errorf("failed to read the users: %w", err)
	}
	defer func() { _ = rows.Close() }()

	found := make([]jellyfinUser, 0)
	for rows.Next() {
		var user jellyfinUser
		var id string
		var lockout, bitrate sql.NullInt64

		err := rows.Scan(&id, &user.username, &user.passwordHash, &user.castReceiverID,
			&user.audioLanguagePreference, &user.subtitleLanguagePreference,
			&user.subtitleMode, &user.syncPlayAccess, &user.playDefaultAudioTrack,
			&user.displayMissingEpisodes, &user.displayCollectionsView,
			&user.enableLocalPassword, &user.hidePlayedInLatest,
			&user.rememberAudioSelections, &user.rememberSubtitleSelections,
			&user.enableNextEpisodeAutoPlay, &user.enableUserPreferenceAccess,
			&user.invalidLoginAttemptCount, &lockout, &user.maxActiveSessions, &bitrate)
		if err != nil {
			return nil, fmt.Errorf("failed to read a user: %w", err)
		}

		user.id, err = uuid.Parse(id)
		if err != nil {
			return nil, fmt.Errorf("failed to read the id of %q: %w", user.username, err)
		}
		if lockout.Valid {
			user.loginAttemptsBeforeLockout = apiutil.Ptr(int32(lockout.Int64))
		}
		if bitrate.Valid {
			user.remoteClientBitrateLimit = apiutil.Ptr(int32(bitrate.Int64))
		}

		found = append(found, user)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to read the users: %w", err)
	}

	permissions, err := readPermissions(ctx, source)
	if err != nil {
		return nil, err
	}
	for index := range found {
		found[index].permissions = permissions[found[index].id]
	}

	slices.SortFunc(found, func(a, b jellyfinUser) int {
		return strings.Compare(a.username, b.username)
	})

	return found, nil
}

func readPermissions(ctx context.Context, source *sql.DB) (map[uuid.UUID]map[int]bool, error) {
	rows, err := source.QueryContext(ctx, `SELECT UserId, Kind, Value FROM Permissions WHERE UserId IS NOT NULL`)
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

func readDevices(ctx context.Context, source *sql.DB) ([]jellyfinDevice, error) {
	rows, err := source.QueryContext(ctx, `
		SELECT UserId, AccessToken, DeviceId, DeviceName, AppName, AppVersion, DateLastActivity
		FROM Devices
		WHERE IsActive = 1`)
	if err != nil {
		return nil, fmt.Errorf("failed to read the devices: %w", err)
	}
	defer func() { _ = rows.Close() }()

	devices := make([]jellyfinDevice, 0)
	for rows.Next() {
		var device jellyfinDevice
		var owner, lastActivity string

		err := rows.Scan(&owner, &device.token, &device.info.ID, &device.info.Name,
			&device.info.AppName, &device.info.AppVersion, &lastActivity)
		if err != nil {
			return nil, fmt.Errorf("failed to read a device: %w", err)
		}

		device.userID, err = uuid.Parse(owner)
		if err != nil {
			return nil, fmt.Errorf("failed to read the user of the device %q: %w", device.info.Name, err)
		}
		device.lastActivity, err = jellyfinTime(lastActivity)
		if err != nil {
			return nil, fmt.Errorf("failed to read when the device %q was last seen: %w", device.info.Name, err)
		}

		devices = append(devices, device)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to read the devices: %w", err)
	}

	return devices, nil
}

func jellyfinTime(value string) (time.Time, error) {
	for _, layout := range jellyfinLayouts {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed.UTC(), nil
		}
	}

	return time.Time{}, fmt.Errorf("%q matches none of %v", value, jellyfinLayouts)
}

func (r *importReport) lines(dryRun bool) []string {
	verb := "imported"
	if dryRun {
		verb = "would import"
	}

	lines := []string{
		fmt.Sprintf("users:    %s %d, left %d already here alone", verb, len(r.created), len(r.existing)),
		fmt.Sprintf("sessions: %s %d, skipped %d last seen too long ago", verb, r.sessions, r.stale),
	}
	lines = append(lines, listed("these users had no password and cannot sign in until `gojellyfin resetpassword` is run for them:", r.needingReset)...)
	lines = append(lines, listed("these users were already here and were left exactly as they are, password included:", r.existing)...)
	lines = append(lines, listed("these devices belong to a user who was already here, so their tokens were not carried and the device has to sign in again:", r.claimed)...)
	lines = append(lines, listed("these devices name a user the source database does not hold, so their sessions were skipped:", r.orphaned)...)

	return lines
}

func listed(heading string, names []string) []string {
	if len(names) == 0 {
		return nil
	}

	lines := []string{"", heading}
	for _, name := range names {
		lines = append(lines, "  "+name)
	}

	return lines
}
