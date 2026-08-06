package user

import (
	"encoding/json"

	"github.com/FreekingDean/gojellyfin/internal/config"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
	"github.com/FreekingDean/gojellyfin/internal/users"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

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
		Name:                      apiutil.Ptr(u.Name),
		ServerId:                  apiutil.Ptr(config.ServerID),
		EnableAutoLogin:           apiutil.Ptr(false),
		HasConfiguredEasyPassword: apiutil.Ptr(false),
		HasConfiguredPassword:     apiutil.Ptr(u.PasswordHash != ""),
		HasPassword:               apiutil.Ptr(u.PasswordHash != ""),
		LastLoginDate:             u.LastLoginDate,
		LastActivityDate:          u.LastActivityDate,
		Configuration:             &configuration,
		Policy:                    &policy,
	}, nil
}

func DefaultConfiguration() api.UserConfiguration {
	subtitleMode := api.SubtitlePlaybackModeOnlyForced

	return api.UserConfiguration{
		CastReceiverId:             apiutil.Ptr(""),
		DisplayCollectionsView:     apiutil.Ptr(false),
		DisplayMissingEpisodes:     apiutil.Ptr(false),
		EnableLocalPassword:        apiutil.Ptr(false),
		EnableNextEpisodeAutoPlay:  apiutil.Ptr(true),
		GroupedFolders:             &[]openapi_types.UUID{},
		HidePlayedInLatest:         apiutil.Ptr(true),
		LatestItemsExcludes:        &[]openapi_types.UUID{},
		MyMediaExcludes:            &[]openapi_types.UUID{},
		OrderedViews:               &[]openapi_types.UUID{},
		PlayDefaultAudioTrack:      apiutil.Ptr(true),
		RememberAudioSelections:    apiutil.Ptr(true),
		RememberSubtitleSelections: apiutil.Ptr(true),
		SubtitleLanguagePreference: apiutil.Ptr(""),
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
		EnableAllChannels:                apiutil.Ptr(true),
		EnableAllDevices:                 apiutil.Ptr(true),
		EnableAllFolders:                 apiutil.Ptr(true),
		EnableAudioPlaybackTranscoding:   apiutil.Ptr(true),
		EnableCollectionManagement:       apiutil.Ptr(isAdministrator),
		EnableContentDeletion:            apiutil.Ptr(isAdministrator),
		EnableContentDeletionFromFolders: &[]string{},
		EnableContentDownloading:         apiutil.Ptr(true),
		EnableLiveTvAccess:               apiutil.Ptr(false),
		EnableLiveTvManagement:           apiutil.Ptr(false),
		EnableLyricManagement:            apiutil.Ptr(false),
		EnableMediaConversion:            apiutil.Ptr(true),
		EnableMediaPlayback:              apiutil.Ptr(true),
		EnablePlaybackRemuxing:           apiutil.Ptr(true),
		EnablePublicSharing:              apiutil.Ptr(true),
		EnableRemoteAccess:               apiutil.Ptr(true),
		EnableRemoteControlOfOtherUsers:  apiutil.Ptr(isAdministrator),
		EnableSharedDeviceControl:        apiutil.Ptr(true),
		EnableSubtitleManagement:         apiutil.Ptr(true),
		EnableSyncTranscoding:            apiutil.Ptr(true),
		EnableUserPreferenceAccess:       apiutil.Ptr(true),
		EnableVideoPlaybackTranscoding:   apiutil.Ptr(true),
		EnabledChannels:                  &[]openapi_types.UUID{},
		EnabledDevices:                   &[]string{},
		EnabledFolders:                   &[]openapi_types.UUID{},
		ForceRemoteSourceTranscoding:     apiutil.Ptr(false),
		InvalidLoginAttemptCount:         apiutil.Ptr(int32(0)),
		IsAdministrator:                  apiutil.Ptr(isAdministrator),
		IsDisabled:                       apiutil.Ptr(false),
		IsHidden:                         apiutil.Ptr(false),
		LoginAttemptsBeforeLockout:       apiutil.Ptr(int32(-1)),
		MaxActiveSessions:                apiutil.Ptr(int32(0)),
		PasswordResetProviderId:          "Jellyfin.Server.Implementations.Users.DefaultPasswordResetProvider",
		RemoteClientBitrateLimit:         apiutil.Ptr(int32(0)),
		SyncPlayAccess:                   &syncPlayAccess,
	}
}
