package syncplay

import (
	"context"
	"errors"
	"log"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/auth"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
	"github.com/FreekingDean/gojellyfin/internal/sessions"
	"github.com/FreekingDean/gojellyfin/internal/syncplay"
)

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

	joined, err := s.syncplay.Create(ctx, apiutil.Deref(body.GroupName), session.ID)
	if err != nil {
		return nil, err
	}

	s.announce(ctx, session, joined)

	return api.SyncPlayCreateGroup200JSONResponse(groupInfo(joined.Group)), nil
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

	joined, err := s.syncplay.Join(ctx, groupID, session.ID, auth.UserID(ctx))
	if errors.Is(err, syncplay.ErrNoGroup) {
		s.publish(ctx, []uuid.UUID{session.ID}, groupID, groupDoesNotExist, groupID.String())

		return api.SyncPlayJoinGroup204Response{}, nil
	}
	if errors.Is(err, syncplay.ErrAccessDenied) {
		s.publish(ctx, []uuid.UUID{session.ID}, groupID, libraryAccessDenied, "")

		return api.SyncPlayJoinGroup204Response{}, nil
	}
	if err != nil {
		return nil, err
	}

	s.announce(ctx, session, joined)

	return api.SyncPlayJoinGroup204Response{}, nil
}

func (s *Server) SyncPlayLeaveGroup(ctx context.Context, request api.SyncPlayLeaveGroupRequestObject) (api.SyncPlayLeaveGroupResponseObject, error) {
	session := auth.SessionFrom(ctx)
	if session == nil {
		return api.SyncPlayLeaveGroup401Response{}, nil
	}

	left, err := s.syncplay.Leave(ctx, session.ID)
	if errors.Is(err, syncplay.ErrNotInGroup) {
		s.publish(ctx, []uuid.UUID{session.ID}, uuid.Nil, notInGroup, "")

		return api.SyncPlayLeaveGroup204Response{}, nil
	}
	if err != nil {
		return nil, err
	}

	s.depart(ctx, left)

	return api.SyncPlayLeaveGroup204Response{}, nil
}

func (s *Server) SyncPlayGetGroups(ctx context.Context, request api.SyncPlayGetGroupsRequestObject) (api.SyncPlayGetGroupsResponseObject, error) {
	groups, err := s.syncplay.List(ctx, auth.UserID(ctx))
	if err != nil {
		return nil, err
	}

	return api.SyncPlayGetGroups200JSONResponse(groupInfos(groups)), nil
}

func (s *Server) SyncPlayGetGroup(ctx context.Context, request api.SyncPlayGetGroupRequestObject) (api.SyncPlayGetGroupResponseObject, error) {
	group, err := s.syncplay.GroupFor(ctx, auth.UserID(ctx), request.Id)
	if errors.Is(err, syncplay.ErrNoGroup) {
		return api.SyncPlayGetGroup404JSONResponse{}, nil
	}
	if err != nil {
		return nil, err
	}

	return api.SyncPlayGetGroup200JSONResponse(groupInfo(group)), nil
}

func (s *Server) Resync(ctx context.Context, sessionIDs []uuid.UUID) {
	groups, err := s.syncplay.GroupsBySessionIDs(ctx, sessionIDs)
	if err != nil {
		log.Printf("failed to re-sync syncplay groups: %v", err)

		return
	}

	asked := make(map[uuid.UUID]bool, len(sessionIDs))
	for _, id := range sessionIDs {
		asked[id] = true
	}

	for _, group := range groups {
		var members []uuid.UUID
		for _, participant := range syncplay.Participants(group) {
			if asked[participant.SessionID] {
				members = append(members, participant.SessionID)
			}
		}

		s.publish(ctx, members, group.ID, groupJoined, groupInfo(group))
	}
}

func (s *Server) announce(ctx context.Context, session *sessions.Session, joined syncplay.Joined) {
	if joined.Left != nil {
		s.depart(ctx, joined.Left)
	}

	s.publish(ctx, []uuid.UUID{session.ID}, joined.Group.ID, groupJoined, groupInfo(joined.Group))

	if !joined.Rejoined {
		others := sessionIDs(syncplay.Participants(joined.Group), session.ID)
		s.publish(ctx, others, joined.Group.ID, userJoined, userName(session))
	}

	s.syncplay.Publish(ctx, joined.Messages)
}

func (s *Server) depart(ctx context.Context, left *syncplay.Departure) {
	s.syncplay.Publish(ctx, left.Messages)
}

func (s *Server) publish(ctx context.Context, sessionIDs []uuid.UUID, groupID uuid.UUID, updateType string, data any) {
	if len(sessionIDs) == 0 {
		return
	}

	update := syncplay.Update{GroupID: groupID, Type: updateType, Data: data}

	if err := s.publisher.Publish(ctx, sessionIDs, syncplay.UpdateMessage, update); err != nil {
		log.Printf("failed to publish %s for group %s: %v", updateType, groupID, err)
	}
}
