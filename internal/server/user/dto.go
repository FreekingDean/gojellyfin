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
		Configuration:             apiutil.Ptr(configurationDto(user.Configuration)),
		Policy:                    apiutil.Ptr(policyDto(user.Policy)),
		LastLoginDate:             user.LastLoginAt,
		LastActivityDate:          user.LastActivityAt,
	}

	return dto
}

func configurationDto(configuration *users.Configuration) api.UserConfiguration {
	if configuration == nil {
		return api.UserConfiguration{}
	}

	return api.UserConfiguration{
		CastReceiverId:             apiutil.Ptr(config.CastReceiverID(configuration.CastReceiverID)),
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

const encodesVideo = false

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
		EnableVideoPlaybackTranscoding:   apiutil.Ptr(encodesVideo),
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
		MaxParentalRating:                level(policy.MaxParentalRating),
		MaxParentalSubRating:             level(policy.MaxParentalSubRating),
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
	return s.users.SaveConfiguration(ctx, id, func(configuration *users.Configuration) {
		mergeConfiguration(configuration, req)
	})
}

func mergeConfiguration(configuration *users.Configuration, req *api.UserConfiguration) {
	configuration.CastReceiverID = apiutil.OrElse(req.CastReceiverId, configuration.CastReceiverID)
	configuration.AudioLanguagePreference = apiutil.OrElse(req.AudioLanguagePreference, configuration.AudioLanguagePreference)
	configuration.SubtitleLanguagePreference = apiutil.OrElse(req.SubtitleLanguagePreference, configuration.SubtitleLanguagePreference)
	configuration.PlayDefaultAudioTrack = apiutil.OrElse(req.PlayDefaultAudioTrack, configuration.PlayDefaultAudioTrack)
	configuration.DisplayMissingEpisodes = apiutil.OrElse(req.DisplayMissingEpisodes, configuration.DisplayMissingEpisodes)
	configuration.DisplayCollectionsView = apiutil.OrElse(req.DisplayCollectionsView, configuration.DisplayCollectionsView)
	configuration.EnableLocalPassword = apiutil.OrElse(req.EnableLocalPassword, configuration.EnableLocalPassword)
	configuration.HidePlayedInLatest = apiutil.OrElse(req.HidePlayedInLatest, configuration.HidePlayedInLatest)
	configuration.RememberAudioSelections = apiutil.OrElse(req.RememberAudioSelections, configuration.RememberAudioSelections)
	configuration.RememberSubtitleSelections = apiutil.OrElse(req.RememberSubtitleSelections, configuration.RememberSubtitleSelections)
	configuration.EnableNextEpisodeAutoPlay = apiutil.OrElse(req.EnableNextEpisodeAutoPlay, configuration.EnableNextEpisodeAutoPlay)
	configuration.GroupedFolders = apiutil.OrElse(req.GroupedFolders, configuration.GroupedFolders)
	configuration.OrderedViews = apiutil.OrElse(req.OrderedViews, configuration.OrderedViews)
	configuration.LatestItemsExcludes = apiutil.OrElse(req.LatestItemsExcludes, configuration.LatestItemsExcludes)
	configuration.MyMediaExcludes = apiutil.OrElse(req.MyMediaExcludes, configuration.MyMediaExcludes)
	if req.SubtitleMode != nil {
		configuration.SubtitleMode = users.SubtitleMode(*req.SubtitleMode)
	}
}

func (s *Server) savePolicy(ctx context.Context, id uuid.UUID, req *api.UserPolicy) error {
	return s.users.SavePolicy(ctx, id, func(policy *users.Policy) {
		mergePolicy(policy, req)
	})
}

func mergePolicy(policy *users.Policy, req *api.UserPolicy) {
	policy.IsAdministrator = apiutil.OrElse(req.IsAdministrator, policy.IsAdministrator)
	policy.IsHidden = apiutil.OrElse(req.IsHidden, policy.IsHidden)
	policy.IsDisabled = apiutil.OrElse(req.IsDisabled, policy.IsDisabled)
	policy.EnableCollectionManagement = apiutil.OrElse(req.EnableCollectionManagement, policy.EnableCollectionManagement)
	policy.EnableSubtitleManagement = apiutil.OrElse(req.EnableSubtitleManagement, policy.EnableSubtitleManagement)
	policy.EnableLyricManagement = apiutil.OrElse(req.EnableLyricManagement, policy.EnableLyricManagement)
	policy.EnableUserPreferenceAccess = apiutil.OrElse(req.EnableUserPreferenceAccess, policy.EnableUserPreferenceAccess)
	policy.EnableRemoteControlOfOtherUsers = apiutil.OrElse(req.EnableRemoteControlOfOtherUsers, policy.EnableRemoteControlOfOtherUsers)
	policy.EnableSharedDeviceControl = apiutil.OrElse(req.EnableSharedDeviceControl, policy.EnableSharedDeviceControl)
	policy.EnableRemoteAccess = apiutil.OrElse(req.EnableRemoteAccess, policy.EnableRemoteAccess)
	policy.EnableLiveTvManagement = apiutil.OrElse(req.EnableLiveTvManagement, policy.EnableLiveTvManagement)
	policy.EnableLiveTvAccess = apiutil.OrElse(req.EnableLiveTvAccess, policy.EnableLiveTvAccess)
	policy.EnableMediaPlayback = apiutil.OrElse(req.EnableMediaPlayback, policy.EnableMediaPlayback)
	policy.EnableAudioPlaybackTranscoding = apiutil.OrElse(req.EnableAudioPlaybackTranscoding, policy.EnableAudioPlaybackTranscoding)
	policy.EnableVideoPlaybackTranscoding = apiutil.OrElse(req.EnableVideoPlaybackTranscoding, policy.EnableVideoPlaybackTranscoding)
	policy.EnablePlaybackRemuxing = apiutil.OrElse(req.EnablePlaybackRemuxing, policy.EnablePlaybackRemuxing)
	policy.ForceRemoteSourceTranscoding = apiutil.OrElse(req.ForceRemoteSourceTranscoding, policy.ForceRemoteSourceTranscoding)
	policy.EnableContentDeletion = apiutil.OrElse(req.EnableContentDeletion, policy.EnableContentDeletion)
	policy.EnableContentDownloading = apiutil.OrElse(req.EnableContentDownloading, policy.EnableContentDownloading)
	policy.EnableSyncTranscoding = apiutil.OrElse(req.EnableSyncTranscoding, policy.EnableSyncTranscoding)
	policy.EnableMediaConversion = apiutil.OrElse(req.EnableMediaConversion, policy.EnableMediaConversion)
	policy.EnablePublicSharing = apiutil.OrElse(req.EnablePublicSharing, policy.EnablePublicSharing)
	policy.EnableAllDevices = apiutil.OrElse(req.EnableAllDevices, policy.EnableAllDevices)
	policy.EnableAllChannels = apiutil.OrElse(req.EnableAllChannels, policy.EnableAllChannels)
	policy.EnableAllFolders = apiutil.OrElse(req.EnableAllFolders, policy.EnableAllFolders)
	policy.InvalidLoginAttemptCount = apiutil.OrElse(req.InvalidLoginAttemptCount, policy.InvalidLoginAttemptCount)
	policy.LoginAttemptsBeforeLockout = apiutil.OrElse(req.LoginAttemptsBeforeLockout, policy.LoginAttemptsBeforeLockout)
	policy.MaxActiveSessions = apiutil.OrElse(req.MaxActiveSessions, policy.MaxActiveSessions)
	policy.RemoteClientBitrateLimit = apiutil.OrElse(req.RemoteClientBitrateLimit, policy.RemoteClientBitrateLimit)
	policy.AllowedTags = apiutil.OrElse(req.AllowedTags, policy.AllowedTags)
	policy.BlockedTags = apiutil.OrElse(req.BlockedTags, policy.BlockedTags)
	policy.EnableContentDeletionFromFolders = apiutil.OrElse(req.EnableContentDeletionFromFolders, policy.EnableContentDeletionFromFolders)
	policy.EnabledDevices = apiutil.OrElse(req.EnabledDevices, policy.EnabledDevices)
	policy.EnabledChannels = apiutil.OrElse(req.EnabledChannels, policy.EnabledChannels)
	policy.EnabledFolders = apiutil.OrElse(req.EnabledFolders, policy.EnabledFolders)
	policy.BlockedMediaFolders = apiutil.OrElse(req.BlockedMediaFolders, policy.BlockedMediaFolders)
	policy.BlockedChannels = apiutil.OrElse(req.BlockedChannels, policy.BlockedChannels)
	policy.MaxParentalRating = rating(req.MaxParentalRating)
	policy.MaxParentalSubRating = rating(req.MaxParentalSubRating)
	if req.AuthenticationProviderId != "" {
		policy.AuthenticationProviderID = req.AuthenticationProviderId
	}
	if req.PasswordResetProviderId != "" {
		policy.PasswordResetProviderID = req.PasswordResetProviderId
	}
	if req.SyncPlayAccess != nil {
		policy.SyncPlayAccess = users.SyncPlayAccess(*req.SyncPlayAccess)
	}
	if req.BlockUnratedItems != nil {
		policy.BlockUnratedItems = unratedItems(*req.BlockUnratedItems)
	}
	if req.AccessSchedules != nil {
		policy.AccessSchedules = accessSchedules(*req.AccessSchedules)
	}
}

func unratedItems(items []api.UnratedItem) []string {
	unrated := make([]string, 0, len(items))
	for _, item := range items {
		unrated = append(unrated, string(item))
	}

	return unrated
}

func accessSchedules(requested []api.AccessSchedule) []users.AccessSchedule {
	schedules := make([]users.AccessSchedule, 0, len(requested))
	for _, schedule := range requested {
		schedules = append(schedules, users.AccessSchedule{
			DayOfWeek: string(apiutil.Deref(schedule.DayOfWeek)),
			StartHour: apiutil.Deref(schedule.StartHour),
			EndHour:   apiutil.Deref(schedule.EndHour),
		})
	}

	return schedules
}

func rating(value *int32) *int {
	if value == nil {
		return nil
	}

	return apiutil.Ptr(int(*value))
}

func level(value *int) *int32 {
	if value == nil {
		return nil
	}

	return apiutil.Ptr(int32(*value))
}
