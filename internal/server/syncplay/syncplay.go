package syncplay

import (
	"context"
	"errors"
	"log"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/auth"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
	"github.com/FreekingDean/gojellyfin/internal/syncplay"
)

// Declared here rather than taken from notify so the tag package depends on
// what it sends, not on how it reaches the other pods.
type Publisher interface {
	Publish(ctx context.Context, sessionIDs []uuid.UUID, messageType string, data any) error
}

type Server struct {
	syncplay  *syncplay.Service
	publisher Publisher
}

func New(syncplay *syncplay.Service, publisher Publisher) *Server {
	return &Server{syncplay: syncplay, publisher: publisher}
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

	s.publish(ctx, []uuid.UUID{session.ID}, group.ID, groupJoined, groupInfo(group))

	return api.SyncPlayCreateGroup200JSONResponse(groupInfo(group)), nil
}

func (s *Server) SyncPlayJoinGroup(ctx context.Context, request api.SyncPlayJoinGroupRequestObject) (api.SyncPlayJoinGroupResponseObject, error) {
	session := auth.SessionFrom(ctx)
	if session == nil {
		return api.SyncPlayJoinGroup401Response{}, nil
	}

	body := apiutil.Body(request.JSONBody, request.ApplicationWildcardPlusJSONBody)
	if body == nil || body.GroupId == nil {
		return api.SyncPlayJoinGroup204Response{}, nil
	}

	groupID := *body.GroupId

	err := s.syncplay.Join(ctx, groupID, session.ID)
	if errors.Is(err, syncplay.ErrNoGroup) {
		s.publish(ctx, []uuid.UUID{session.ID}, groupID, groupDoesNotExist, groupID.String())

		return api.SyncPlayJoinGroup204Response{}, nil
	}
	if err != nil {
		return nil, err
	}

	group, err := s.syncplay.GroupByID(ctx, groupID)
	if err != nil {
		return nil, err
	}

	participants := syncplay.Participants(group)
	s.publish(ctx, []uuid.UUID{session.ID}, groupID, groupJoined, groupInfo(group))
	s.publish(ctx, sessionIDs(participants, session.ID), groupID, userJoined, userNameOf(participants, session.ID))

	return api.SyncPlayJoinGroup204Response{}, nil
}

func (s *Server) SyncPlayLeaveGroup(ctx context.Context, request api.SyncPlayLeaveGroupRequestObject) (api.SyncPlayLeaveGroupResponseObject, error) {
	session := auth.SessionFrom(ctx)
	if session == nil {
		return api.SyncPlayLeaveGroup401Response{}, nil
	}

	group, err := s.syncplay.GroupBySessionID(ctx, session.ID)
	if errors.Is(err, syncplay.ErrNoGroup) {
		return api.SyncPlayLeaveGroup204Response{}, nil
	}
	if err != nil {
		return nil, err
	}

	participants := syncplay.Participants(group)

	if err := s.syncplay.Leave(ctx, session.ID); err != nil {
		return nil, err
	}

	s.publish(ctx, []uuid.UUID{session.ID}, group.ID, groupLeft, group.ID.String())
	s.publish(ctx, sessionIDs(participants, session.ID), group.ID, userLeft, userNameOf(participants, session.ID))

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

// An update nobody can be told about does not undo the membership change that
// already committed, so a failed publish is logged and the request still
// answers what it did.
func (s *Server) publish(ctx context.Context, sessionIDs []uuid.UUID, groupID uuid.UUID, updateType string, data any) {
	update := groupUpdate{GroupId: groupID, Type: updateType, Data: data}

	if err := s.publisher.Publish(ctx, sessionIDs, groupUpdateMessage, update); err != nil {
		log.Printf("failed to publish %s for group %s: %v", updateType, groupID, err)
	}
}
