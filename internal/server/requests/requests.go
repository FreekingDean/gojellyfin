package requests

import (
	"context"
	"errors"
	"slices"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/auth"
	"github.com/FreekingDean/gojellyfin/internal/discover"
	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/requests"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
)

type Queue interface {
	Queue(ctx context.Context) ([]requests.Asked, error)
	Approve(ctx context.Context, requestID uuid.UUID) error
	Decline(ctx context.Context, requestID uuid.UUID) error
	Askers(ctx context.Context) ([]requests.Asker, error)
	Allow(ctx context.Context, userID uuid.UUID, allowance requests.Allowance) error
	Mine(ctx context.Context, userID uuid.UUID) ([]*requests.Request, error)
}

type Holdings interface {
	HeldKeys(ctx context.Context, keys []string) ([]string, error)
}

type Server struct {
	requests Queue
	library  Holdings
}

func New(requests *requests.Service, library *items.Service) *Server {
	return &Server{requests: requests, library: library}
}

func (s *Server) GoJellyfinListMyRequests(
	ctx context.Context,
	_ api.GoJellyfinListMyRequestsRequestObject,
) (api.GoJellyfinListMyRequestsResponseObject, error) {
	userID := auth.UserID(ctx)
	if userID == uuid.Nil {
		return nil, auth.ErrUnauthorized
	}

	mine, err := s.requests.Mine(ctx, userID)
	if err != nil {
		return nil, err
	}

	keys := make([]string, len(mine))
	for i, record := range mine {
		keys[i] = record.Key
	}
	held, err := s.library.HeldKeys(ctx, keys)
	if err != nil {
		return nil, err
	}

	listed := make(api.GoJellyfinListMyRequests200JSONResponse, len(mine))
	for i, record := range mine {
		listed[i] = api.MyRequest{
			ItemId:    discover.ID(items.Kind(record.Kind), uint32(record.TmdbID)),
			Name:      record.Name,
			Kind:      string(record.Kind),
			State:     state(record.Status, slices.Contains(held, record.Key)),
			CreatedAt: record.CreatedAt,
		}
	}

	return listed, nil
}

func state(status requests.Status, held bool) string {
	switch {
	case status == requests.StatusDeclined:
		return "Declined"
	case held:
		return "Ready"
	case status == requests.StatusPending:
		return "Pending"
	default:
		return "Requested"
	}
}

func (s *Server) GoJellyfinListRequests(
	ctx context.Context,
	_ api.GoJellyfinListRequestsRequestObject,
) (api.GoJellyfinListRequestsResponseObject, error) {
	queue, err := s.requests.Queue(ctx)
	if err != nil {
		return nil, err
	}

	titles := make(api.GoJellyfinListRequests200JSONResponse, len(queue))
	for i, asked := range queue {
		titles[i] = requestedTitle(asked)
	}

	return titles, nil
}

func (s *Server) GoJellyfinApproveRequest(
	ctx context.Context,
	request api.GoJellyfinApproveRequestRequestObject,
) (api.GoJellyfinApproveRequestResponseObject, error) {
	err := s.requests.Approve(ctx, request.RequestId)
	switch {
	case errors.Is(err, requests.ErrNotFound):
		return api.GoJellyfinApproveRequest404Response{}, nil
	case errors.Is(err, requests.ErrNowhere):
		return api.GoJellyfinApproveRequest400JSONResponse{Message: err.Error()}, nil
	case err != nil:
		return nil, err
	}

	return api.GoJellyfinApproveRequest204Response{}, nil
}

func (s *Server) GoJellyfinDeclineRequest(
	ctx context.Context,
	request api.GoJellyfinDeclineRequestRequestObject,
) (api.GoJellyfinDeclineRequestResponseObject, error) {
	err := s.requests.Decline(ctx, request.RequestId)
	if errors.Is(err, requests.ErrNotFound) {
		return api.GoJellyfinDeclineRequest404Response{}, nil
	}
	if err != nil {
		return nil, err
	}

	return api.GoJellyfinDeclineRequest204Response{}, nil
}

func (s *Server) GoJellyfinListRequestUsers(
	ctx context.Context,
	_ api.GoJellyfinListRequestUsersRequestObject,
) (api.GoJellyfinListRequestUsersResponseObject, error) {
	askers, err := s.requests.Askers(ctx)
	if err != nil {
		return nil, err
	}

	users := make(api.GoJellyfinListRequestUsers200JSONResponse, len(askers))
	for i, asker := range askers {
		users[i] = api.RequestUser{
			Id:            asker.ID,
			Name:          asker.Name,
			Used:          int32(asker.Used),
			CanRequest:    asker.CanRequest,
			NeedsApproval: asker.NeedsApproval,
			Quota:         quotaDto(asker.Quota),
		}
	}

	return users, nil
}

func (s *Server) GoJellyfinUpdateRequestUser(
	ctx context.Context,
	request api.GoJellyfinUpdateRequestUserRequestObject,
) (api.GoJellyfinUpdateRequestUserResponseObject, error) {
	if request.Body == nil {
		return api.GoJellyfinUpdateRequestUser404Response{}, nil
	}

	allowance := requests.Allowance{
		CanRequest:    request.Body.CanRequest,
		NeedsApproval: request.Body.NeedsApproval,
	}
	if request.Body.Quota != nil && *request.Body.Quota >= 0 {
		allowance.Quota = apiutil.Ptr(int(*request.Body.Quota))
	}

	err := s.requests.Allow(ctx, request.UserId, allowance)
	if errors.Is(err, requests.ErrNotFound) {
		return api.GoJellyfinUpdateRequestUser404Response{}, nil
	}
	if err != nil {
		return nil, err
	}

	return api.GoJellyfinUpdateRequestUser204Response{}, nil
}

func quotaDto(quota *int) *int32 {
	if quota == nil {
		return nil
	}

	return apiutil.Ptr(int32(*quota))
}

func requestedTitle(asked requests.Asked) api.RequestedTitle {
	entries := make([]api.RequestEntry, len(asked.Requests))
	for i, entry := range asked.Requests {
		entries[i] = api.RequestEntry{
			Id:        entry.ID,
			UserId:    entry.UserID,
			Status:    string(entry.Status),
			CreatedAt: entry.CreatedAt,
		}
		if entry.User != nil {
			entries[i].UserName = entry.User.Name
		}
	}

	submissions := make([]api.RequestSubmission, len(asked.Submissions))
	for i, submission := range asked.Submissions {
		submissions[i] = api.RequestSubmission{
			AcceptedAt: submission.AcceptedAt,
			Error:      apiutil.ZeroOrNilPtr(submission.LastError),
		}
		if submission.Source != nil {
			submissions[i].SourceName = submission.Source.Name
		}
	}

	return api.RequestedTitle{
		ItemId:      discover.ID(items.Kind(asked.Kind), uint32(asked.TmdbID)),
		Name:        asked.Name,
		Kind:        string(asked.Kind),
		Requests:    entries,
		Submissions: submissions,
	}
}
