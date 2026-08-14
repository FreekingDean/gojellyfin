package user

import (
	"context"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/config"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
	"github.com/FreekingDean/gojellyfin/internal/users"
)

func userDto(user *users.User) api.UserDto {
	if user == nil {
		return api.UserDto{}
	}

	dto := api.UserDto{
		Id:                        &user.ID,
		Name:                      apiutil.Ptr(user.Name),
		ServerId:                  apiutil.Ptr(config.ServerID),
		EnableAutoLogin:           apiutil.Ptr(false),
		HasConfiguredEasyPassword: apiutil.Ptr(false),
		HasConfiguredPassword:     apiutil.Ptr(user.PasswordHash != ""),
		HasPassword:               apiutil.Ptr(user.PasswordHash != ""),
		Configuration:             apiutil.Ptr(configurationDto(user.Edges.Configuration)),
		Policy:                    apiutil.Ptr(policyDto(user.Edges.Policy)),
	}

	if !user.LastLoginAt.IsZero() {
		dto.LastLoginDate = &user.LastLoginAt
	}
	if !user.LastActivityAt.IsZero() {
		dto.LastActivityDate = &user.LastActivityAt
	}

	return dto
}

func configurationDto(configuration *users.Configuration) api.UserConfiguration {
	if configuration == nil {
		return api.UserConfiguration{}
	}

	return api.UserConfiguration{
		CastReceiverId:             apiutil.Ptr(configuration.CastReceiverID),
		AudioLanguagePreference:    apiutil.Ptr(configuration.AudioLanguagePreference),
		SubtitleLanguagePreference: apiutil.Ptr(configuration.SubtitleLanguagePreference),
		PlayDefaultAudioTrack:      apiutil.Ptr(configuration.PlayDefaultAudioTrack),
		SubtitleMode:               apiutil.Ptr(api.SubtitlePlaybackMode(configuration.SubtitleMode)),
		GroupedFolders:             &configuration.GroupedFolders,
		OrderedViews:               &configuration.OrderedViews,
		LatestItemsExcludes:        &configuration.LatestItemsExcludes,
		MyMediaExcludes:            &configuration.MyMediaExcludes,
		DisplayMissingEpisodes:     apiutil.Ptr(configuration.DisplayMissingEpisodes),
		DisplayCollectionsView:     apiutil.Ptr(configuration.DisplayCollectionsView),
		EnableLocalPassword:        apiutil.Ptr(configuration.EnableLocalPassword),
		HidePlayedInLatest:         apiutil.Ptr(configuration.HidePlayedInLatest),
		RememberAudioSelections:    apiutil.Ptr(configuration.RememberAudioSelections),
		RememberSubtitleSelections: apiutil.Ptr(configuration.RememberSubtitleSelections),
		EnableNextEpisodeAutoPlay:  apiutil.Ptr(configuration.EnableNextEpisodeAutoPlay),
	}
}

func policyDto(policy *users.Policy) api.UserPolicy {
	if policy == nil {
		return api.UserPolicy{}
	}

	schedules := make([]api.AccessSchedule, 0, len(policy.AccessSchedules))
	for _, schedule := range policy.AccessSchedules {
		schedules = append(schedules, api.AccessSchedule{
			DayOfWeek: apiutil.Ptr(api.DynamicDayOfWeek(schedule.DayOfWeek)),
			StartHour: apiutil.Ptr(schedule.StartHour),
			EndHour:   apiutil.Ptr(schedule.EndHour),
		})
	}

	unrated := make([]api.UnratedItem, 0, len(policy.BlockUnratedItems))
	for _, item := range policy.BlockUnratedItems {
		unrated = append(unrated, api.UnratedItem(item))
	}

	return api.UserPolicy{
		IsAdministrator:                  apiutil.Ptr(policy.IsAdministrator),
		IsHidden:                         apiutil.Ptr(policy.IsHidden),
		IsDisabled:                       apiutil.Ptr(policy.IsDisabled),
		EnableCollectionManagement:       apiutil.Ptr(policy.EnableCollectionManagement),
		EnableSubtitleManagement:         apiutil.Ptr(policy.EnableSubtitleManagement),
		EnableLyricManagement:            apiutil.Ptr(policy.EnableLyricManagement),
		EnableUserPreferenceAccess:       apiutil.Ptr(policy.EnableUserPreferenceAccess),
		EnableRemoteControlOfOtherUsers:  apiutil.Ptr(policy.EnableRemoteControlOfOtherUsers),
		EnableSharedDeviceControl:        apiutil.Ptr(policy.EnableSharedDeviceControl),
		EnableRemoteAccess:               apiutil.Ptr(policy.EnableRemoteAccess),
		EnableLiveTvManagement:           apiutil.Ptr(policy.EnableLiveTvManagement),
		EnableLiveTvAccess:               apiutil.Ptr(policy.EnableLiveTvAccess),
		EnableMediaPlayback:              apiutil.Ptr(policy.EnableMediaPlayback),
		EnableAudioPlaybackTranscoding:   apiutil.Ptr(policy.EnableAudioPlaybackTranscoding),
		EnableVideoPlaybackTranscoding:   apiutil.Ptr(policy.EnableVideoPlaybackTranscoding),
		EnablePlaybackRemuxing:           apiutil.Ptr(policy.EnablePlaybackRemuxing),
		ForceRemoteSourceTranscoding:     apiutil.Ptr(policy.ForceRemoteSourceTranscoding),
		EnableContentDeletion:            apiutil.Ptr(policy.EnableContentDeletion),
		EnableContentDownloading:         apiutil.Ptr(policy.EnableContentDownloading),
		EnableSyncTranscoding:            apiutil.Ptr(policy.EnableSyncTranscoding),
		EnableMediaConversion:            apiutil.Ptr(policy.EnableMediaConversion),
		EnablePublicSharing:              apiutil.Ptr(policy.EnablePublicSharing),
		EnableAllDevices:                 apiutil.Ptr(policy.EnableAllDevices),
		EnableAllChannels:                apiutil.Ptr(policy.EnableAllChannels),
		EnableAllFolders:                 apiutil.Ptr(policy.EnableAllFolders),
		InvalidLoginAttemptCount:         apiutil.Ptr(policy.InvalidLoginAttemptCount),
		LoginAttemptsBeforeLockout:       apiutil.Ptr(policy.LoginAttemptsBeforeLockout),
		MaxActiveSessions:                apiutil.Ptr(policy.MaxActiveSessions),
		RemoteClientBitrateLimit:         apiutil.Ptr(policy.RemoteClientBitrateLimit),
		AllowedTags:                      &policy.AllowedTags,
		BlockedTags:                      &policy.BlockedTags,
		AccessSchedules:                  &schedules,
		BlockUnratedItems:                &unrated,
		EnableContentDeletionFromFolders: &policy.EnableContentDeletionFromFolders,
		EnabledDevices:                   &policy.EnabledDevices,
		EnabledChannels:                  &policy.EnabledChannels,
		EnabledFolders:                   &policy.EnabledFolders,
		BlockedMediaFolders:              &policy.BlockedMediaFolders,
		BlockedChannels:                  &policy.BlockedChannels,
		AuthenticationProviderId:         policy.AuthenticationProviderID,
		PasswordResetProviderId:          policy.PasswordResetProviderID,
		SyncPlayAccess:                   apiutil.Ptr(api.SyncPlayUserAccessType(policy.SyncPlayAccess)),
	}
}

func (s *Server) saveConfiguration(ctx context.Context, id uuid.UUID, req *api.UserConfiguration) error {
	update := s.users.UpdateConfiguration(id).
		SetNillableCastReceiverID(req.CastReceiverId).
		SetNillableAudioLanguagePreference(req.AudioLanguagePreference).
		SetNillableSubtitleLanguagePreference(req.SubtitleLanguagePreference).
		SetNillablePlayDefaultAudioTrack(req.PlayDefaultAudioTrack).
		SetNillableDisplayMissingEpisodes(req.DisplayMissingEpisodes).
		SetNillableDisplayCollectionsView(req.DisplayCollectionsView).
		SetNillableEnableLocalPassword(req.EnableLocalPassword).
		SetNillableHidePlayedInLatest(req.HidePlayedInLatest).
		SetNillableRememberAudioSelections(req.RememberAudioSelections).
		SetNillableRememberSubtitleSelections(req.RememberSubtitleSelections).
		SetNillableEnableNextEpisodeAutoPlay(req.EnableNextEpisodeAutoPlay)

	if req.SubtitleMode != nil {
		update.SetSubtitleMode(users.SubtitleMode(*req.SubtitleMode))
	}
	if req.GroupedFolders != nil {
		update.SetGroupedFolders(*req.GroupedFolders)
	}
	if req.OrderedViews != nil {
		update.SetOrderedViews(*req.OrderedViews)
	}
	if req.LatestItemsExcludes != nil {
		update.SetLatestItemsExcludes(*req.LatestItemsExcludes)
	}
	if req.MyMediaExcludes != nil {
		update.SetMyMediaExcludes(*req.MyMediaExcludes)
	}

	return update.Exec(ctx)
}

func (s *Server) savePolicy(ctx context.Context, id uuid.UUID, req *api.UserPolicy) error {
	update := s.users.UpdatePolicy(id).
		SetNillableIsAdministrator(req.IsAdministrator).
		SetNillableIsHidden(req.IsHidden).
		SetNillableIsDisabled(req.IsDisabled).
		SetNillableEnableCollectionManagement(req.EnableCollectionManagement).
		SetNillableEnableSubtitleManagement(req.EnableSubtitleManagement).
		SetNillableEnableLyricManagement(req.EnableLyricManagement).
		SetNillableEnableUserPreferenceAccess(req.EnableUserPreferenceAccess).
		SetNillableEnableRemoteControlOfOtherUsers(req.EnableRemoteControlOfOtherUsers).
		SetNillableEnableSharedDeviceControl(req.EnableSharedDeviceControl).
		SetNillableEnableRemoteAccess(req.EnableRemoteAccess).
		SetNillableEnableLiveTvManagement(req.EnableLiveTvManagement).
		SetNillableEnableLiveTvAccess(req.EnableLiveTvAccess).
		SetNillableEnableMediaPlayback(req.EnableMediaPlayback).
		SetNillableEnableAudioPlaybackTranscoding(req.EnableAudioPlaybackTranscoding).
		SetNillableEnableVideoPlaybackTranscoding(req.EnableVideoPlaybackTranscoding).
		SetNillableEnablePlaybackRemuxing(req.EnablePlaybackRemuxing).
		SetNillableForceRemoteSourceTranscoding(req.ForceRemoteSourceTranscoding).
		SetNillableEnableContentDeletion(req.EnableContentDeletion).
		SetNillableEnableContentDownloading(req.EnableContentDownloading).
		SetNillableEnableSyncTranscoding(req.EnableSyncTranscoding).
		SetNillableEnableMediaConversion(req.EnableMediaConversion).
		SetNillableEnablePublicSharing(req.EnablePublicSharing).
		SetNillableEnableAllDevices(req.EnableAllDevices).
		SetNillableEnableAllChannels(req.EnableAllChannels).
		SetNillableEnableAllFolders(req.EnableAllFolders).
		SetNillableInvalidLoginAttemptCount(req.InvalidLoginAttemptCount).
		SetNillableLoginAttemptsBeforeLockout(req.LoginAttemptsBeforeLockout).
		SetNillableMaxActiveSessions(req.MaxActiveSessions).
		SetNillableRemoteClientBitrateLimit(req.RemoteClientBitrateLimit)

	if req.AuthenticationProviderId != "" {
		update.SetAuthenticationProviderID(req.AuthenticationProviderId)
	}
	if req.PasswordResetProviderId != "" {
		update.SetPasswordResetProviderID(req.PasswordResetProviderId)
	}
	if req.SyncPlayAccess != nil {
		update.SetSyncPlayAccess(users.SyncPlayAccess(*req.SyncPlayAccess))
	}
	if req.AllowedTags != nil {
		update.SetAllowedTags(*req.AllowedTags)
	}
	if req.BlockedTags != nil {
		update.SetBlockedTags(*req.BlockedTags)
	}
	if req.EnableContentDeletionFromFolders != nil {
		update.SetEnableContentDeletionFromFolders(*req.EnableContentDeletionFromFolders)
	}
	if req.EnabledDevices != nil {
		update.SetEnabledDevices(*req.EnabledDevices)
	}
	if req.EnabledChannels != nil {
		update.SetEnabledChannels(*req.EnabledChannels)
	}
	if req.EnabledFolders != nil {
		update.SetEnabledFolders(*req.EnabledFolders)
	}
	if req.BlockedMediaFolders != nil {
		update.SetBlockedMediaFolders(*req.BlockedMediaFolders)
	}
	if req.BlockedChannels != nil {
		update.SetBlockedChannels(*req.BlockedChannels)
	}
	if req.BlockUnratedItems != nil {
		unrated := make([]string, 0, len(*req.BlockUnratedItems))
		for _, item := range *req.BlockUnratedItems {
			unrated = append(unrated, string(item))
		}
		update.SetBlockUnratedItems(unrated)
	}
	if req.AccessSchedules != nil {
		schedules := make([]users.AccessSchedule, 0, len(*req.AccessSchedules))
		for _, schedule := range *req.AccessSchedules {
			schedules = append(schedules, users.AccessSchedule{
				DayOfWeek: string(apiutil.Deref(schedule.DayOfWeek)),
				StartHour: apiutil.Deref(schedule.StartHour),
				EndHour:   apiutil.Deref(schedule.EndHour),
			})
		}
		update.SetAccessSchedules(schedules)
	}

	return update.Exec(ctx)
}

// What a sign-in screen needs and no more: no policy, no configuration, and
// nothing about when the account was last used.
func publicUserDto(user *users.User) api.UserDto {
	if user == nil {
		return api.UserDto{}
	}

	return api.UserDto{
		Id:                    &user.ID,
		Name:                  apiutil.Ptr(user.Name),
		ServerId:              apiutil.Ptr(config.ServerID),
		HasPassword:           apiutil.Ptr(user.PasswordHash != ""),
		HasConfiguredPassword: apiutil.Ptr(user.PasswordHash != ""),
	}
}
