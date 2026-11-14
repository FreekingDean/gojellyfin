package migration

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/store"
	apikeymodel "github.com/FreekingDean/gojellyfin/internal/store/apikey"
	devicemodel "github.com/FreekingDean/gojellyfin/internal/store/device"
	sessionmodel "github.com/FreekingDean/gojellyfin/internal/store/session"
	configurationmodel "github.com/FreekingDean/gojellyfin/internal/store/userconfiguration"
	datamodel "github.com/FreekingDean/gojellyfin/internal/store/useritemdata"
	policymodel "github.com/FreekingDean/gojellyfin/internal/store/userpolicy"
)

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

const watchedBatch = 500

var (
	subtitleModes = []configurationmodel.SubtitleMode{
		configurationmodel.SubtitleModeDefault,
		configurationmodel.SubtitleModeAlways,
		configurationmodel.SubtitleModeOnlyForced,
		configurationmodel.SubtitleModeNone,
		configurationmodel.SubtitleModeSmart,
	}
	syncPlayAccess = []policymodel.SyncPlayAccess{
		policymodel.SyncPlayAccessCreateAndJoinGroups,
		policymodel.SyncPlayAccessJoinGroups,
		policymodel.SyncPlayAccessNone,
	}
)

func Run(ctx context.Context, client *store.Client, source *Source, dryRun bool) (*Report, error) {
	tx, err := client.Tx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	report := &Report{DryRun: dryRun, ServerID: source.ServerID, Orphaned: source.Orphaned}

	migrated, err := writeUsers(ctx, tx, source.Users, report)
	if err != nil {
		return nil, err
	}
	if err := writeSessions(ctx, tx, source.Devices, migrated, report); err != nil {
		return nil, err
	}
	if err := writeAPIKeys(ctx, tx, source.APIKeys, report); err != nil {
		return nil, err
	}
	if err := writeWatched(ctx, tx, source.Watched, migrated, report); err != nil {
		return nil, err
	}

	if dryRun {
		return report, nil
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit: %w", err)
	}

	return report, nil
}

func writeUsers(ctx context.Context, tx *store.Tx, found []User, report *Report) (map[uuid.UUID]bool, error) {
	present, err := tx.User.Query().All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to read the users already here: %w", err)
	}
	byID := map[uuid.UUID]bool{}
	byUsername := map[string]bool{}
	for _, user := range present {
		byID[user.ID] = true
		byUsername[strings.ToLower(user.Username)] = true
	}

	migrated := map[uuid.UUID]bool{}
	for _, user := range found {
		outcome := UserOutcome{Username: user.Username, Restricted: !granted(user, permissionEnableAllFolders, true)}

		switch {
		case byID[user.ID]:
			outcome.State = UserAlreadyHere
			migrated[user.ID] = true
		case byUsername[strings.ToLower(user.Username)]:
			outcome.State = UserNameTaken
		default:
			outcome.State = UserCreated
			outcome.NoPassword = user.PasswordHash == ""
			migrated[user.ID] = true
			byUsername[strings.ToLower(user.Username)] = true
			if err := createUser(ctx, tx, user); err != nil {
				return nil, err
			}
		}

		report.Users = append(report.Users, outcome)
	}

	return migrated, nil
}

func granted(user User, kind int, otherwise bool) bool {
	value, ok := user.Permissions[kind]
	if !ok {
		return otherwise
	}

	return value
}

func permission(user User, kind int) *bool {
	value, ok := user.Permissions[kind]
	if !ok {
		return nil
	}

	return &value
}

func createUser(ctx context.Context, tx *store.Tx, user User) error {
	err := tx.User.Create().
		SetID(user.ID).
		SetName(user.Username).
		SetUsername(user.Username).
		SetPasswordHash(user.PasswordHash).
		SetNillableLastLoginAt(user.LastLoginAt).
		SetNillableLastActivityAt(user.LastActivityAt).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to create %q: %w", user.Username, err)
	}

	err = tx.UserPolicy.Create().
		SetUserID(user.ID).
		SetNillableIsAdministrator(permission(user, permissionIsAdministrator)).
		SetNillableIsHidden(permission(user, permissionIsHidden)).
		SetNillableIsDisabled(permission(user, permissionIsDisabled)).
		SetNillableEnableSharedDeviceControl(permission(user, permissionEnableSharedDeviceControl)).
		SetNillableEnableRemoteAccess(permission(user, permissionEnableRemoteAccess)).
		SetNillableEnableLiveTvManagement(permission(user, permissionEnableLiveTvManagement)).
		SetNillableEnableLiveTvAccess(permission(user, permissionEnableLiveTvAccess)).
		SetNillableEnableMediaPlayback(permission(user, permissionEnableMediaPlayback)).
		SetNillableEnableAudioPlaybackTranscoding(permission(user, permissionEnableAudioPlaybackTranscoding)).
		SetNillableEnableVideoPlaybackTranscoding(permission(user, permissionEnableVideoPlaybackTranscoding)).
		SetNillableEnableContentDeletion(permission(user, permissionEnableContentDeletion)).
		SetNillableEnableContentDownloading(permission(user, permissionEnableContentDownloading)).
		SetNillableEnableSyncTranscoding(permission(user, permissionEnableSyncTranscoding)).
		SetNillableEnableMediaConversion(permission(user, permissionEnableMediaConversion)).
		SetNillableEnableAllDevices(permission(user, permissionEnableAllDevices)).
		SetNillableEnableAllChannels(permission(user, permissionEnableAllChannels)).
		SetNillableEnableAllFolders(permission(user, permissionEnableAllFolders)).
		SetNillableEnablePublicSharing(permission(user, permissionEnablePublicSharing)).
		SetNillableEnableRemoteControlOfOtherUsers(permission(user, permissionEnableRemoteControlOfOtherUsers)).
		SetNillableEnablePlaybackRemuxing(permission(user, permissionEnablePlaybackRemuxing)).
		SetNillableForceRemoteSourceTranscoding(permission(user, permissionForceRemoteSourceTranscoding)).
		SetNillableEnableCollectionManagement(permission(user, permissionEnableCollectionManagement)).
		SetNillableEnableSubtitleManagement(permission(user, permissionEnableSubtitleManagement)).
		SetNillableEnableLyricManagement(permission(user, permissionEnableLyricManagement)).
		SetEnableUserPreferenceAccess(user.EnableUserPreferenceAccess).
		SetInvalidLoginAttemptCount(user.InvalidLoginAttemptCount).
		SetNillableLoginAttemptsBeforeLockout(user.LoginAttemptsBeforeLockout).
		SetMaxActiveSessions(user.MaxActiveSessions).
		SetNillableRemoteClientBitrateLimit(user.RemoteClientBitrateLimit).
		SetNillableSyncPlayAccess(ordinal(syncPlayAccess, user.SyncPlayAccess)).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to create the policy of %q: %w", user.Username, err)
	}

	err = tx.UserConfiguration.Create().
		SetUserID(user.ID).
		SetCastReceiverID(user.CastReceiverID).
		SetAudioLanguagePreference(user.AudioLanguagePreference).
		SetSubtitleLanguagePreference(user.SubtitleLanguagePreference).
		SetNillableSubtitleMode(ordinal(subtitleModes, user.SubtitleMode)).
		SetPlayDefaultAudioTrack(user.PlayDefaultAudioTrack).
		SetDisplayMissingEpisodes(user.DisplayMissingEpisodes).
		SetDisplayCollectionsView(user.DisplayCollectionsView).
		SetEnableLocalPassword(user.EnableLocalPassword).
		SetHidePlayedInLatest(user.HidePlayedInLatest).
		SetRememberAudioSelections(user.RememberAudioSelections).
		SetRememberSubtitleSelections(user.RememberSubtitleSelections).
		SetEnableNextEpisodeAutoPlay(user.EnableNextEpisodeAutoPlay).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to create the configuration of %q: %w", user.Username, err)
	}

	return nil
}

func ordinal[T any](values []T, index int) *T {
	if index < 0 || index >= len(values) {
		return nil
	}

	return &values[index]
}

func writeSessions(ctx context.Context, tx *store.Tx, devices []Device, migrated map[uuid.UUID]bool, report *Report) error {
	present, err := tx.Session.Query().Select(sessionmodel.FieldAccessToken).Strings(ctx)
	if err != nil {
		return fmt.Errorf("failed to read the sessions already here: %w", err)
	}
	tokens := map[string]bool{}
	for _, token := range present {
		tokens[token] = true
	}

	for _, device := range devices {
		switch {
		case !migrated[device.UserID] || device.Token == "":
			report.SessionsSkipped++

			continue
		case tokens[device.Token]:
			report.SessionsAlreadyHere++

			continue
		}
		tokens[device.Token] = true
		report.SessionsCarried++

		deviceID, err := tx.Device.Create().
			SetClientID(device.ClientID).
			SetName(device.Name).
			SetAppName(device.AppName).
			SetAppVersion(device.AppVersion).
			SetSupportsMediaControl(false).
			SetSupportsPersistentIdentifier(false).
			SetNillableLastActivityAt(device.LastActivityAt).
			OnConflictColumns(devicemodel.FieldClientID).
			UpdateName().
			UpdateAppName().
			UpdateAppVersion().
			ID(ctx)
		if err != nil {
			return fmt.Errorf("failed to carry the device %q: %w", device.Name, err)
		}

		err = tx.Session.Create().
			SetUserID(device.UserID).
			SetDeviceID(deviceID).
			SetAccessToken(device.Token).
			SetNillableLastActivityAt(device.LastActivityAt).
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("failed to carry the session of the device %q: %w", device.Name, err)
		}
	}

	return nil
}

func writeAPIKeys(ctx context.Context, tx *store.Tx, keys []APIKey, report *Report) error {
	present, err := tx.ApiKey.Query().Select(apikeymodel.FieldAccessToken).Strings(ctx)
	if err != nil {
		return fmt.Errorf("failed to read the api keys already here: %w", err)
	}
	tokens := map[string]bool{}
	for _, token := range present {
		tokens[token] = true
	}

	for _, key := range keys {
		if tokens[key.Token] {
			report.APIKeysAlreadyHere++

			continue
		}
		tokens[key.Token] = true
		report.APIKeysCarried++

		if err := tx.ApiKey.Create().SetAppName(key.Name).SetAccessToken(key.Token).Exec(ctx); err != nil {
			return fmt.Errorf("failed to carry the api key %q: %w", key.Name, err)
		}
	}

	return nil
}

func writeWatched(ctx context.Context, tx *store.Tx, watched []Watched, migrated map[uuid.UUID]bool, report *Report) error {
	library, err := readLibrary(ctx, tx)
	if err != nil {
		return err
	}

	present, err := tx.UserItemData.Query().All(ctx)
	if err != nil {
		return fmt.Errorf("failed to read the watch state already here: %w", err)
	}
	existing := map[[2]uuid.UUID]*store.UserItemData{}
	for _, row := range present {
		existing[[2]uuid.UUID{row.UserID, row.ItemID}] = row
	}

	creates := make([]*store.UserItemDataCreate, 0)
	for _, state := range watched {
		if !migrated[state.UserID] {
			report.WatchedSkipped++

			continue
		}

		itemID, ok := library.match(state.Title)
		if !ok {
			report.unmatched(state.Title)

			continue
		}

		pair := [2]uuid.UUID{state.UserID, itemID}
		row, found := existing[pair]
		if !found {
			report.WatchedCarried++
			existing[pair] = &store.UserItemData{}
			creates = append(creates, tx.UserItemData.Create().
				SetUserID(state.UserID).
				SetItemID(itemID).
				SetPlayed(state.Played).
				SetIsFavorite(state.IsFavorite).
				SetPlayCount(state.PlayCount).
				SetPlaybackPositionTicks(state.PlaybackPositionTicks).
				SetNillableLastPlayedAt(state.LastPlayedAt).
				SetNillableRating(state.Rating).
				SetNillableLikes(state.Likes))

			continue
		}
		if row.ID == uuid.Nil {
			continue
		}

		merged := further(Watched{
			Played:                row.Played,
			IsFavorite:            row.IsFavorite,
			PlayCount:             row.PlayCount,
			PlaybackPositionTicks: row.PlaybackPositionTicks,
			LastPlayedAt:          row.LastPlayedAt,
			Rating:                row.Rating,
			Likes:                 row.Likes,
		}, state)
		if unchanged(row, merged) {
			report.WatchedAlreadyHere++

			continue
		}
		report.WatchedMerged++

		err := tx.UserItemData.Update().
			Where(datamodel.ID(row.ID)).
			SetPlayed(merged.Played).
			SetIsFavorite(merged.IsFavorite).
			SetPlayCount(merged.PlayCount).
			SetPlaybackPositionTicks(merged.PlaybackPositionTicks).
			SetNillableLastPlayedAt(merged.LastPlayedAt).
			SetNillableRating(merged.Rating).
			SetNillableLikes(merged.Likes).
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("failed to merge the watch state of %q: %w", state.Title.Name, err)
		}
	}

	for start := 0; start < len(creates); start += watchedBatch {
		batch := creates[start:min(start+watchedBatch, len(creates))]
		if err := tx.UserItemData.CreateBulk(batch...).Exec(ctx); err != nil {
			return fmt.Errorf("failed to carry the watch state: %w", err)
		}
	}

	return nil
}

func unchanged(row *store.UserItemData, merged Watched) bool {
	return row.Played == merged.Played &&
		row.IsFavorite == merged.IsFavorite &&
		row.PlayCount == merged.PlayCount &&
		row.PlaybackPositionTicks == merged.PlaybackPositionTicks &&
		sameMoment(row.LastPlayedAt, merged.LastPlayedAt) &&
		(row.Rating == nil) == (merged.Rating == nil) &&
		(row.Likes == nil) == (merged.Likes == nil)
}

type library struct {
	byKey      map[string]uuid.UUID
	keys       map[uuid.UUID]string
	byProvider map[string]uuid.UUID
	byPath     map[string]uuid.UUID
}

func readLibrary(ctx context.Context, tx *store.Tx) (*library, error) {
	found := &library{
		byKey:      map[string]uuid.UUID{},
		keys:       map[uuid.UUID]string{},
		byProvider: map[string]uuid.UUID{},
		byPath:     map[string]uuid.UUID{},
	}

	rows, err := tx.Item.Query().All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to read the items here: %w", err)
	}
	for _, item := range rows {
		if item.Key == "" {
			continue
		}
		found.byKey[item.Key] = item.ID
		found.keys[item.ID] = item.Key
		for provider, value := range item.ProviderIds {
			found.byProvider[providerKey(TitleKind(item.Kind), provider, value)] = item.ID
		}
	}

	files, err := tx.ItemSource.Query().All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to read the files here: %w", err)
	}
	for _, file := range files {
		found.byPath[file.Path] = file.ItemID
	}

	return found, nil
}

func providerKey(kind TitleKind, provider, value string) string {
	return string(kind) + "|" + strings.ToLower(provider) + "|" + value
}

func (l *library) match(title *Title) (uuid.UUID, bool) {
	if id, ok := l.byKey[l.key(title)]; ok {
		return id, true
	}
	if title.Kind == KindMovie || title.Kind == KindSeries {
		if id, ok := l.byIdentity(title); ok {
			return id, true
		}
	}
	if title.Path == "" {
		return uuid.Nil, false
	}
	id, ok := l.byPath[title.Path]

	return id, ok
}

func (l *library) byIdentity(title *Title) (uuid.UUID, bool) {
	for provider, value := range title.Providers {
		if id, ok := l.byProvider[providerKey(title.Kind, provider, value)]; ok {
			return id, true
		}
	}

	return uuid.Nil, false
}

func (l *library) key(title *Title) string {
	switch title.Kind {
	case KindMovie:
		if id, ok := l.tmdb(title); ok {
			return items.MovieKey(id)
		}
	case KindSeries:
		if id, ok := l.tmdb(title); ok {
			return items.SeriesKey(id)
		}
	case KindSeason:
		if id, ok := l.seriesTmdb(title); ok && title.Index != nil {
			return items.SeasonKey(id, *title.Index)
		}
	case KindEpisode:
		if id, ok := l.seriesTmdb(title); ok && title.ParentIndex != nil && title.Index != nil {
			return items.EpisodeKey(id, *title.ParentIndex, *title.Index)
		}
	}

	return ""
}

func (l *library) seriesTmdb(title *Title) (int, bool) {
	if title.Series == nil {
		return 0, false
	}

	return l.tmdb(title.Series)
}

func (l *library) tmdb(title *Title) (int, bool) {
	if id, err := strconv.Atoi(title.Providers["tmdb"]); err == nil && id > 0 {
		return id, true
	}
	if id, ok := l.byIdentity(title); ok {
		return items.TmdbID(l.keys[id])
	}

	return 0, false
}

func sameMoment(a, b *time.Time) bool {
	if a == nil || b == nil {
		return a == b
	}

	return a.Truncate(time.Microsecond).Equal(b.Truncate(time.Microsecond))
}
