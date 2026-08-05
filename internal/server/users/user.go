package users

import (
	"context"
	"encoding/json"
	"time"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/FreekingDean/gojellyfin/internal/auth"
	"github.com/FreekingDean/gojellyfin/internal/http/middleware"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/store"
)

const (
	serverId     = "e10a32fca79342d7b8b9d96e255ce1bc"
	rootFolderId = "e9d5075a555c1cbc394eec4cef295274"
)

type Server struct {
	store store.Store
}

func New(s store.Store) *Server {
	return &Server{
		store: s,
	}
}

func (s *Server) AuthenticateUserByName(ctx context.Context, request api.AuthenticateUserByNameRequestObject) (api.AuthenticateUserByNameResponseObject, error) {
	req := body(request.JSONBody, request.ApplicationWildcardPlusJSONBody)
	if req == nil || req.Username == nil {
		return nil, middleware.ErrUnauthorized
	}

	user, err := s.store.GetUserByUsername(ctx, *req.Username)
	if err != nil {
		return nil, middleware.ErrUnauthorized
	}

	matches, err := auth.Verify(deref(req.Pw), user.PasswordHash)
	if err != nil || !matches {
		return nil, middleware.ErrUnauthorized
	}

	token, err := auth.NewToken()
	if err != nil {
		return nil, err
	}

	now := time.Now()
	authorization := middleware.AuthorizationFrom(ctx)
	session := &store.Session{
		UserID:           user.ID,
		AccessToken:      token,
		DeviceID:         authorization.DeviceID,
		DeviceName:       authorization.Device,
		Client:           authorization.Client,
		AppVersion:       authorization.Version,
		LastActivityDate: now,
	}
	if err := s.store.CreateSession(ctx, session); err != nil {
		return nil, err
	}

	if err := s.store.TouchUserLogin(ctx, user.ID); err != nil {
		return nil, err
	}
	user.LastLoginDate = &now
	user.LastActivityDate = &now

	dto, err := userDto(user)
	if err != nil {
		return nil, err
	}

	return api.AuthenticateUserByName200JSONResponse{
		AccessToken: ptr(token),
		ServerId:    ptr(serverId),
		User:        &dto,
		SessionInfo: sessionDto(session, user),
	}, nil
}

func (s *Server) GetCurrentUser(ctx context.Context, request api.GetCurrentUserRequestObject) (api.GetCurrentUserResponseObject, error) {
	dto, err := userDto(middleware.UserFrom(ctx))
	if err != nil {
		return nil, err
	}

	return api.GetCurrentUser200JSONResponse(dto), nil
}

func (s *Server) GetUserById(ctx context.Context, request api.GetUserByIdRequestObject) (api.GetUserByIdResponseObject, error) {
	user, err := s.store.GetUser(ctx, request.UserId)
	if err != nil {
		return api.GetUserById404JSONResponse{}, nil
	}

	dto, err := userDto(user)
	if err != nil {
		return nil, err
	}

	return api.GetUserById200JSONResponse(dto), nil
}

func (s *Server) GetUsers(ctx context.Context, request api.GetUsersRequestObject) (api.GetUsersResponseObject, error) {
	dtos, err := s.listUserDtos(ctx)
	if err != nil {
		return nil, err
	}

	return api.GetUsers200JSONResponse(dtos), nil
}

func (s *Server) GetPublicUsers(ctx context.Context, request api.GetPublicUsersRequestObject) (api.GetPublicUsersResponseObject, error) {
	dtos, err := s.listUserDtos(ctx)
	if err != nil {
		return nil, err
	}

	return api.GetPublicUsers200JSONResponse(dtos), nil
}

func (s *Server) CreateUserByName(ctx context.Context, request api.CreateUserByNameRequestObject) (api.CreateUserByNameResponseObject, error) {
	req := body(request.JSONBody, request.ApplicationWildcardPlusJSONBody)
	if req == nil {
		return api.CreateUserByName403Response{}, nil
	}

	hash, err := auth.Hash(deref(req.Password))
	if err != nil {
		return nil, err
	}

	user := &store.User{
		Name:         req.Name,
		Username:     req.Name,
		PasswordHash: hash,
	}
	if err := s.store.CreateUser(ctx, user); err != nil {
		return nil, err
	}

	dto, err := userDto(user)
	if err != nil {
		return nil, err
	}

	return api.CreateUserByName200JSONResponse(dto), nil
}

func (s *Server) UpdateUser(ctx context.Context, request api.UpdateUserRequestObject) (api.UpdateUserResponseObject, error) {
	req := body(request.JSONBody, request.ApplicationWildcardPlusJSONBody)
	if req == nil || request.Params.UserId == nil {
		return api.UpdateUser400JSONResponse{}, nil
	}

	user, err := s.store.GetUser(ctx, *request.Params.UserId)
	if err != nil {
		return api.UpdateUser400JSONResponse{}, nil
	}

	if req.Name != nil {
		user.Name = *req.Name
	}
	if req.Configuration != nil {
		if user.Configuration, err = json.Marshal(req.Configuration); err != nil {
			return nil, err
		}
	}
	if req.Policy != nil {
		if user.Policy, err = json.Marshal(req.Policy); err != nil {
			return nil, err
		}
		user.IsAdministrator = deref(req.Policy.IsAdministrator)
	}

	if err := s.store.UpdateUser(ctx, user); err != nil {
		return nil, err
	}

	return api.UpdateUser204Response{}, nil
}

func (s *Server) UpdateUserConfiguration(ctx context.Context, request api.UpdateUserConfigurationRequestObject) (api.UpdateUserConfigurationResponseObject, error) {
	req := body(request.JSONBody, request.ApplicationWildcardPlusJSONBody)
	if req == nil {
		return api.UpdateUserConfiguration403JSONResponse{}, nil
	}

	user, err := s.userFor(ctx, request.Params.UserId)
	if err != nil {
		return api.UpdateUserConfiguration403JSONResponse{}, nil
	}

	if user.Configuration, err = json.Marshal(req); err != nil {
		return nil, err
	}
	if err := s.store.UpdateUser(ctx, user); err != nil {
		return nil, err
	}

	return api.UpdateUserConfiguration204Response{}, nil
}

func (s *Server) UpdateUserPolicy(ctx context.Context, request api.UpdateUserPolicyRequestObject) (api.UpdateUserPolicyResponseObject, error) {
	req := body(request.JSONBody, request.ApplicationWildcardPlusJSONBody)
	if req == nil {
		return api.UpdateUserPolicy400JSONResponse{}, nil
	}

	user, err := s.store.GetUser(ctx, request.UserId)
	if err != nil {
		return api.UpdateUserPolicy400JSONResponse{}, nil
	}

	if user.Policy, err = json.Marshal(req); err != nil {
		return nil, err
	}
	user.IsAdministrator = deref(req.IsAdministrator)

	if err := s.store.UpdateUser(ctx, user); err != nil {
		return nil, err
	}

	return api.UpdateUserPolicy204Response{}, nil
}

func (s *Server) UpdateUserPassword(ctx context.Context, request api.UpdateUserPasswordRequestObject) (api.UpdateUserPasswordResponseObject, error) {
	req := body(request.JSONBody, request.ApplicationWildcardPlusJSONBody)
	if req == nil {
		return api.UpdateUserPassword403JSONResponse{}, nil
	}

	user, err := s.userFor(ctx, request.Params.UserId)
	if err != nil {
		return api.UpdateUserPassword404JSONResponse{}, nil
	}

	if !deref(req.ResetPassword) {
		matches, err := auth.Verify(deref(req.CurrentPw), user.PasswordHash)
		if err != nil || !matches {
			return api.UpdateUserPassword403JSONResponse{}, nil
		}
	}

	if user.PasswordHash, err = auth.Hash(deref(req.NewPw)); err != nil {
		return nil, err
	}
	if err := s.store.UpdateUser(ctx, user); err != nil {
		return nil, err
	}

	return api.UpdateUserPassword204Response{}, nil
}

func (s *Server) DeleteUser(ctx context.Context, request api.DeleteUserRequestObject) (api.DeleteUserResponseObject, error) {
	if _, err := s.store.GetUser(ctx, request.UserId); err != nil {
		return api.DeleteUser404JSONResponse{}, nil
	}

	if err := s.store.DeleteUser(ctx, request.UserId); err != nil {
		return nil, err
	}

	return api.DeleteUser204Response{}, nil
}

func (s *Server) listUserDtos(ctx context.Context) ([]api.UserDto, error) {
	users, err := s.store.ListUsers(ctx)
	if err != nil {
		return nil, err
	}

	dtos := make([]api.UserDto, 0, len(users))
	for _, user := range users {
		dto, err := userDto(&user)
		if err != nil {
			return nil, err
		}
		dtos = append(dtos, dto)
	}

	return dtos, nil
}

func (s *Server) userFor(ctx context.Context, id *openapi_types.UUID) (*store.User, error) {
	if id == nil {
		return middleware.UserFrom(ctx), nil
	}

	return s.store.GetUser(ctx, *id)
}

func userDto(u *store.User) (api.UserDto, error) {
	if u == nil {
		return api.UserDto{}, nil
	}

	configuration := defaultConfiguration()
	if len(u.Configuration) > 0 {
		if err := json.Unmarshal(u.Configuration, &configuration); err != nil {
			return api.UserDto{}, err
		}
	}

	policy := defaultPolicy(u.IsAdministrator)
	if len(u.Policy) > 0 {
		if err := json.Unmarshal(u.Policy, &policy); err != nil {
			return api.UserDto{}, err
		}
	}

	return api.UserDto{
		Id:                        &u.ID,
		Name:                      ptr(u.Name),
		ServerId:                  ptr(serverId),
		EnableAutoLogin:           ptr(false),
		HasConfiguredEasyPassword: ptr(false),
		HasConfiguredPassword:     ptr(u.PasswordHash != ""),
		HasPassword:               ptr(u.PasswordHash != ""),
		LastLoginDate:             u.LastLoginDate,
		LastActivityDate:          u.LastActivityDate,
		Configuration:             &configuration,
		Policy:                    &policy,
	}, nil
}

func defaultConfiguration() api.UserConfiguration {
	subtitleMode := api.SubtitlePlaybackModeOnlyForced

	return api.UserConfiguration{
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
	}
}

func defaultPolicy(isAdministrator bool) api.UserPolicy {
	syncPlayAccess := api.SyncPlayUserAccessTypeCreateAndJoinGroups

	return api.UserPolicy{
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
		EnableCollectionManagement:       ptr(isAdministrator),
		EnableContentDeletion:            ptr(isAdministrator),
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
		EnableRemoteControlOfOtherUsers:  ptr(isAdministrator),
		EnableSharedDeviceControl:        ptr(true),
		EnableSubtitleManagement:         ptr(true),
		EnableSyncTranscoding:            ptr(true),
		EnableUserPreferenceAccess:       ptr(true),
		EnableVideoPlaybackTranscoding:   ptr(true),
		EnabledChannels:                  &[]openapi_types.UUID{},
		EnabledDevices:                   &[]string{},
		EnabledFolders:                   &[]openapi_types.UUID{},
		ForceRemoteSourceTranscoding:     ptr(false),
		InvalidLoginAttemptCount:         ptr(int32(0)),
		IsAdministrator:                  ptr(isAdministrator),
		IsDisabled:                       ptr(false),
		IsHidden:                         ptr(false),
		LoginAttemptsBeforeLockout:       ptr(int32(-1)),
		MaxActiveSessions:                ptr(int32(0)),
		PasswordResetProviderId:          "Jellyfin.Server.Implementations.Users.DefaultPasswordResetProvider",
		RemoteClientBitrateLimit:         ptr(int32(0)),
		SyncPlayAccess:                   &syncPlayAccess,
	}
}

func ptr[T any](v T) *T {
	return &v
}

func deref[T any](v *T) T {
	if v == nil {
		var zero T
		return zero
	}

	return *v
}
func body[T any](jsonBody, wildcardBody *T) *T {
	if jsonBody != nil {
		return jsonBody
	}

	return wildcardBody
}

func sessionDto(session *store.Session, user *store.User) *api.SessionInfoDto {
	dto := &api.SessionInfoDto{
		Id:                    ptr(session.ID.String()),
		ServerId:              ptr(serverId),
		Client:                ptr(session.Client),
		DeviceId:              ptr(session.DeviceID),
		DeviceName:            ptr(session.DeviceName),
		ApplicationVersion:    ptr(session.AppVersion),
		LastActivityDate:      ptr(session.LastActivityDate),
		IsActive:              ptr(true),
		SupportsRemoteControl: ptr(false),
		PlayableMediaTypes:    &[]api.MediaType{},
		SupportedCommands:     &[]api.GeneralCommandType{},
		AdditionalUsers:       &[]api.SessionUserInfo{},
	}

	if user != nil {
		dto.UserId = &user.ID
		dto.UserName = ptr(user.Name)
	}

	return dto
}
