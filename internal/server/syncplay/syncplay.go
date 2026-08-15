package syncplay

import (
	"context"
	"errors"

	"github.com/FreekingDean/gojellyfin/internal/auth"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
	"github.com/FreekingDean/gojellyfin/internal/syncplay"
)

type Server struct {
	syncplay *syncplay.Service
}

func New(syncplay *syncplay.Service) *Server {
	return &Server{syncplay: syncplay}
}

func (s *Server) SyncPlayCreateGroup(ctx context.Context, request api.SyncPlayCreateGroupRequestObject) (api.SyncPlayCreateGroupResponseObject, error) {
	session := auth.SessionFrom(ctx)
	if session == nil {
		return api.SyncPlayCreateGroup401Response{}, nil
	}

	body := apiutil.Body(request.JSONBody, request.ApplicationWildcardPlusJSONBody)
	if body == nil {
		body = &api.NewGroupRequestDto{}
	}

	group, err := s.syncplay.Create(ctx, apiutil.Deref(body.GroupName), session.ID)
	if err != nil {
		return nil, err
	}

	return api.SyncPlayCreateGroup200JSONResponse(groupInfo(group)), nil
}

// Upstream reports a bad group id over the websocket and answers 204 either
// way; without that channel the refusal has to be the status code.
func (s *Server) SyncPlayJoinGroup(ctx context.Context, request api.SyncPlayJoinGroupRequestObject) (api.SyncPlayJoinGroupResponseObject, error) {
	session := auth.SessionFrom(ctx)
	if session == nil {
		return api.SyncPlayJoinGroup401Response{}, nil
	}

	body := apiutil.Body(request.JSONBody, request.ApplicationWildcardPlusJSONBody)
	if body == nil || body.GroupId == nil {
		return api.SyncPlayJoinGroup403Response{}, nil
	}

	err := s.syncplay.Join(ctx, *body.GroupId, session.ID)
	if errors.Is(err, syncplay.ErrNoGroup) {
		return api.SyncPlayJoinGroup403Response{}, nil
	}
	if err != nil {
		return nil, err
	}

	return api.SyncPlayJoinGroup204Response{}, nil
}

func (s *Server) SyncPlayLeaveGroup(ctx context.Context, request api.SyncPlayLeaveGroupRequestObject) (api.SyncPlayLeaveGroupResponseObject, error) {
	session := auth.SessionFrom(ctx)
	if session == nil {
		return api.SyncPlayLeaveGroup401Response{}, nil
	}

	if err := s.syncplay.Leave(ctx, session.ID); err != nil {
		return nil, err
	}

	return api.SyncPlayLeaveGroup204Response{}, nil
}

func (s *Server) SyncPlayGetGroups(ctx context.Context, request api.SyncPlayGetGroupsRequestObject) (api.SyncPlayGetGroupsResponseObject, error) {
	groups, err := s.syncplay.List(ctx)
	if err != nil {
		return nil, err
	}

	return api.SyncPlayGetGroups200JSONResponse(groupInfos(groups)), nil
}

func (s *Server) SyncPlayGetGroup(ctx context.Context, request api.SyncPlayGetGroupRequestObject) (api.SyncPlayGetGroupResponseObject, error) {
	group, err := s.syncplay.GroupByID(ctx, request.Id)
	if errors.Is(err, syncplay.ErrNoGroup) {
		return api.SyncPlayGetGroup404JSONResponse{}, nil
	}
	if err != nil {
		return nil, err
	}

	return api.SyncPlayGetGroup200JSONResponse(groupInfo(group)), nil
}
