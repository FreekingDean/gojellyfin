package server

import (
	"time"

	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/FreekingDean/gojellyfin/internal/server/api"
)

const serverId = "e10a32fca79342d7b8b9d96e255ce1bc"

func adminUser() *api.UserDto {
	subtitleMode := api.SubtitlePlaybackModeOnlyForced
	syncPlayAccess := api.SyncPlayUserAccessTypeCreateAndJoinGroups

	return &api.UserDto{
		Id:                        uid("4cb0ebf115cb44068837635374d3a6ea"),
		Name:                      ptr("admin"),
		ServerId:                  ptr(serverId),
		EnableAutoLogin:           ptr(false),
		HasConfiguredEasyPassword: ptr(false),
		HasConfiguredPassword:     ptr(true),
		HasPassword:               ptr(true),
		LastActivityDate:          tim("2026-08-04T21:12:34.8984389Z"),
		LastLoginDate:             tim("2026-03-11T01:56:40.1986963Z"),
		Configuration: &api.UserConfiguration{
			CastReceiverId:             ptr(""),
			DisplayCollectionsView:     ptr(false),
			DisplayMissingEpisodes:     ptr(false),
			EnableLocalPassword:        ptr(false),
			EnableNextEpisodeAutoPlay:  ptr(true),
			GroupedFolders:             &[]openapi_types.UUID{},
			HidePlayedInLatest:         ptr(true),
			LatestItemsExcludes:        &[]openapi_types.UUID{},
			MyMediaExcludes:            &[]openapi_types.UUID{},
			OrderedViews:               &[]openapi_types.UUID{},
			PlayDefaultAudioTrack:      ptr(true),
			RememberAudioSelections:    ptr(true),
			RememberSubtitleSelections: ptr(true),
			SubtitleLanguagePreference: ptr(""),
			SubtitleMode:               &subtitleMode,
		},
		Policy: &api.UserPolicy{
			AccessSchedules:                  &[]api.AccessSchedule{},
			AllowedTags:                      &[]string{},
			AuthenticationProviderId:         "authentik",
			BlockUnratedItems:                &[]api.UnratedItem{},
			BlockedChannels:                  &[]openapi_types.UUID{},
			BlockedMediaFolders:              &[]openapi_types.UUID{},
			BlockedTags:                      &[]string{},
			EnableAllChannels:                ptr(true),
			EnableAllDevices:                 ptr(true),
			EnableAllFolders:                 ptr(true),
			EnableAudioPlaybackTranscoding:   ptr(true),
			EnableCollectionManagement:       ptr(true),
			EnableContentDeletion:            ptr(true),
			EnableContentDeletionFromFolders: &[]string{},
			EnableContentDownloading:         ptr(true),
			EnableLiveTvAccess:               ptr(true),
			EnableLiveTvManagement:           ptr(false),
			EnableLyricManagement:            ptr(false),
			EnableMediaConversion:            ptr(true),
			EnableMediaPlayback:              ptr(true),
			EnablePlaybackRemuxing:           ptr(true),
			EnablePublicSharing:              ptr(true),
			EnableRemoteAccess:               ptr(true),
			EnableRemoteControlOfOtherUsers:  ptr(true),
			EnableSharedDeviceControl:        ptr(true),
			EnableSubtitleManagement:         ptr(true),
			EnableSyncTranscoding:            ptr(true),
			EnableUserPreferenceAccess:       ptr(true),
			EnableVideoPlaybackTranscoding:   ptr(true),
			EnabledChannels:                  &[]openapi_types.UUID{},
			EnabledDevices:                   &[]string{},
			EnabledFolders:                   &[]openapi_types.UUID{},
			ForceRemoteSourceTranscoding:     ptr(true),
			InvalidLoginAttemptCount:         ptr(int32(0)),
			IsAdministrator:                  ptr(true),
			IsDisabled:                       ptr(false),
			IsHidden:                         ptr(true),
			LoginAttemptsBeforeLockout:       ptr(int32(-1)),
			MaxActiveSessions:                ptr(int32(0)),
			PasswordResetProviderId:          "Jellyfin.Server.Implementations.Users.DefaultPasswordResetProvider",
			RemoteClientBitrateLimit:         ptr(int32(0)),
			SyncPlayAccess:                   &syncPlayAccess,
		},
	}
}

func uid(s string) *openapi_types.UUID {
	u := uuid.MustParse(s)
	return &u
}

func tim(s string) *time.Time {
	t, _ := time.Parse(time.RFC3339, s)
	return &t
}
