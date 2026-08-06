package dtos

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/FreekingDean/gojellyfin/internal/auth"
	"github.com/FreekingDean/gojellyfin/internal/config"
	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/libraries"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/users"
	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

var FolderTypes = map[string]bool{
	"Series":           true,
	"Season":           true,
	"Folder":           true,
	"CollectionFolder": true,
}

func ItemDto(item *items.Item, childCount int32) api.BaseItemDto {
	kind := api.BaseItemKind(item.Type)
	isFolder := FolderTypes[item.Type]

	dto := api.BaseItemDto{
		Id:                &item.ID,
		ServerId:          Ptr(config.ServerID),
		Name:              Ptr(item.Name),
		SortName:          Ptr(item.SortName),
		Type:              &kind,
		Path:              Ptr(item.Path),
		IsFolder:          Ptr(isFolder),
		ParentId:          item.ParentID,
		IndexNumber:       item.IndexNumber,
		ParentIndexNumber: item.ParentIndexNumber,
		ProductionYear:    item.ProductionYear,
		PremiereDate:      item.PremiereDate,
		RunTimeTicks:      item.RunTimeTicks,
		DateCreated:       Ptr(item.CreatedAt),
		LocationType:      Ptr(api.FileSystem),
		ImageTags:         &map[string]string{},
		BackdropImageTags: &[]string{},
	}

	if item.Overview != "" {
		dto.Overview = Ptr(item.Overview)
	}
	if isFolder {
		dto.ChildCount = Ptr(childCount)
	} else {
		dto.MediaType = Ptr(api.MediaTypeVideo)
	}

	return dto
}

func LibraryView(library *libraries.Library) api.BaseItemDto {
	collectionType := api.CollectionType(library.CollectionType)

	return api.BaseItemDto{
		Id:                &library.ID,
		ServerId:          Ptr(config.ServerID),
		Name:              Ptr(library.Name),
		SortName:          Ptr(strings.ToLower(library.Name)),
		Type:              Ptr(api.BaseItemKindCollectionFolder),
		CollectionType:    &collectionType,
		IsFolder:          Ptr(true),
		LocationType:      Ptr(api.FileSystem),
		ImageTags:         &map[string]string{},
		BackdropImageTags: &[]string{},
	}
}

func Descending(order *[]api.SortOrder) bool {
	if order == nil {
		return false
	}

	for _, value := range *order {
		if value == api.Descending {
			return true
		}
	}

	return false
}

func SortFields(sortBy *[]api.ItemSortBy) []string {
	if sortBy == nil {
		return nil
	}

	fields := make([]string, 0, len(*sortBy))
	for _, field := range *sortBy {
		fields = append(fields, string(field))
	}

	return fields
}

func UserItemDataDto(datum *items.Datum) api.UserItemDataDto {
	return api.UserItemDataDto{
		ItemId:                &datum.ItemID,
		Key:                   Ptr(datum.ItemID.String()),
		Played:                Ptr(datum.Played),
		PlayCount:             Ptr(datum.PlayCount),
		IsFavorite:            Ptr(datum.IsFavorite),
		PlaybackPositionTicks: Ptr(datum.PlaybackPositionTicks),
		LastPlayedDate:        datum.LastPlayedDate,
	}
}

func UserDto(u *users.User) (api.UserDto, error) {
	if u == nil {
		return api.UserDto{}, nil
	}

	configuration := DefaultConfiguration()
	if len(u.Configuration) > 0 {
		if err := json.Unmarshal(u.Configuration, &configuration); err != nil {
			return api.UserDto{}, err
		}
	}

	policy := DefaultPolicy(u.IsAdministrator)
	if len(u.Policy) > 0 {
		if err := json.Unmarshal(u.Policy, &policy); err != nil {
			return api.UserDto{}, err
		}
	}

	return api.UserDto{
		Id:                        &u.ID,
		Name:                      Ptr(u.Name),
		ServerId:                  Ptr(config.ServerID),
		EnableAutoLogin:           Ptr(false),
		HasConfiguredEasyPassword: Ptr(false),
		HasConfiguredPassword:     Ptr(u.PasswordHash != ""),
		HasPassword:               Ptr(u.PasswordHash != ""),
		LastLoginDate:             u.LastLoginDate,
		LastActivityDate:          u.LastActivityDate,
		Configuration:             &configuration,
		Policy:                    &policy,
	}, nil
}

func DefaultConfiguration() api.UserConfiguration {
	subtitleMode := api.SubtitlePlaybackModeOnlyForced

	return api.UserConfiguration{
		CastReceiverId:             Ptr(""),
		DisplayCollectionsView:     Ptr(false),
		DisplayMissingEpisodes:     Ptr(false),
		EnableLocalPassword:        Ptr(false),
		EnableNextEpisodeAutoPlay:  Ptr(true),
		GroupedFolders:             &[]openapi_types.UUID{},
		HidePlayedInLatest:         Ptr(true),
		LatestItemsExcludes:        &[]openapi_types.UUID{},
		MyMediaExcludes:            &[]openapi_types.UUID{},
		OrderedViews:               &[]openapi_types.UUID{},
		PlayDefaultAudioTrack:      Ptr(true),
		RememberAudioSelections:    Ptr(true),
		RememberSubtitleSelections: Ptr(true),
		SubtitleLanguagePreference: Ptr(""),
		SubtitleMode:               &subtitleMode,
	}
}

func DefaultPolicy(isAdministrator bool) api.UserPolicy {
	syncPlayAccess := api.SyncPlayUserAccessTypeCreateAndJoinGroups

	return api.UserPolicy{
		AccessSchedules:                  &[]api.AccessSchedule{},
		AllowedTags:                      &[]string{},
		AuthenticationProviderId:         "authentik",
		BlockUnratedItems:                &[]api.UnratedItem{},
		BlockedChannels:                  &[]openapi_types.UUID{},
		BlockedMediaFolders:              &[]openapi_types.UUID{},
		BlockedTags:                      &[]string{},
		EnableAllChannels:                Ptr(true),
		EnableAllDevices:                 Ptr(true),
		EnableAllFolders:                 Ptr(true),
		EnableAudioPlaybackTranscoding:   Ptr(true),
		EnableCollectionManagement:       Ptr(isAdministrator),
		EnableContentDeletion:            Ptr(isAdministrator),
		EnableContentDeletionFromFolders: &[]string{},
		EnableContentDownloading:         Ptr(true),
		EnableLiveTvAccess:               Ptr(false),
		EnableLiveTvManagement:           Ptr(false),
		EnableLyricManagement:            Ptr(false),
		EnableMediaConversion:            Ptr(true),
		EnableMediaPlayback:              Ptr(true),
		EnablePlaybackRemuxing:           Ptr(true),
		EnablePublicSharing:              Ptr(true),
		EnableRemoteAccess:               Ptr(true),
		EnableRemoteControlOfOtherUsers:  Ptr(isAdministrator),
		EnableSharedDeviceControl:        Ptr(true),
		EnableSubtitleManagement:         Ptr(true),
		EnableSyncTranscoding:            Ptr(true),
		EnableUserPreferenceAccess:       Ptr(true),
		EnableVideoPlaybackTranscoding:   Ptr(true),
		EnabledChannels:                  &[]openapi_types.UUID{},
		EnabledDevices:                   &[]string{},
		EnabledFolders:                   &[]openapi_types.UUID{},
		ForceRemoteSourceTranscoding:     Ptr(false),
		InvalidLoginAttemptCount:         Ptr(int32(0)),
		IsAdministrator:                  Ptr(isAdministrator),
		IsDisabled:                       Ptr(false),
		IsHidden:                         Ptr(false),
		LoginAttemptsBeforeLockout:       Ptr(int32(-1)),
		MaxActiveSessions:                Ptr(int32(0)),
		PasswordResetProviderId:          "Jellyfin.Server.Implementations.Users.DefaultPasswordResetProvider",
		RemoteClientBitrateLimit:         Ptr(int32(0)),
		SyncPlayAccess:                   &syncPlayAccess,
	}
}

func SessionDto(session *auth.Session, user *users.User) *api.SessionInfoDto {
	dto := &api.SessionInfoDto{
		Id:                    Ptr(session.ID.String()),
		ServerId:              Ptr(config.ServerID),
		Client:                Ptr(session.Client),
		DeviceId:              Ptr(session.DeviceID),
		DeviceName:            Ptr(session.DeviceName),
		ApplicationVersion:    Ptr(session.AppVersion),
		LastActivityDate:      Ptr(session.LastActivityDate),
		IsActive:              Ptr(true),
		SupportsRemoteControl: Ptr(false),
		PlayableMediaTypes:    &[]api.MediaType{},
		SupportedCommands:     &[]api.GeneralCommandType{},
		AdditionalUsers:       &[]api.SessionUserInfo{},
	}

	if user != nil {
		dto.UserId = &user.ID
		dto.UserName = Ptr(user.Name)
	}

	return dto
}

// ItemDtos fills in child counts and per-user data, which every tag that
// returns items needs.
func ItemDtos(ctx context.Context, store *items.Service, records []items.Item) ([]api.BaseItemDto, error) {
	folderIDs := make([]uuid.UUID, 0, len(records))
	itemIDs := make([]uuid.UUID, 0, len(records))
	for _, item := range records {
		itemIDs = append(itemIDs, item.ID)
		if FolderTypes[item.Type] {
			folderIDs = append(folderIDs, item.ID)
		}
	}

	counts, err := store.CountChildren(ctx, folderIDs)
	if err != nil {
		return nil, err
	}

	userData := map[uuid.UUID]items.Datum{}
	if userID := auth.UserID(ctx); userID != uuid.Nil {
		if userData, err = store.ListUserItemData(ctx, userID, itemIDs); err != nil {
			return nil, err
		}
	}

	converted := make([]api.BaseItemDto, 0, len(records))
	for _, item := range records {
		dto := ItemDto(&item, counts[item.ID])
		datum, ok := userData[item.ID]
		if !ok {
			datum = items.Datum{ItemID: item.ID}
		}
		dto.UserData = Ptr(UserItemDataDto(&datum))
		converted = append(converted, dto)
	}

	return converted, nil
}
