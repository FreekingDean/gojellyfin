package activitylog

import (
	"context"

	"github.com/FreekingDean/gojellyfin/internal/activity"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
)

//go:generate go run go.uber.org/mock/mockgen -source=$GOFILE -destination=mocks/mock_$GOFILE -package=mocks
type ActivitiesService interface {
	Entries(ctx context.Context, query activity.Query) ([]*activity.Entry, int, error)
}

type Server struct {
	activity ActivitiesService
}

func New(activity *activity.Service) *Server {
	return &Server{activity: activity}
}

func (s *Server) GetLogEntries(ctx context.Context, request api.GetLogEntriesRequestObject) (api.GetLogEntriesResponseObject, error) {
	startIndex := apiutil.Deref(request.Params.StartIndex)

	entries, total, err := s.activity.Entries(ctx, activity.Query{
		StartIndex: int(startIndex),
		Limit:      int(apiutil.Deref(request.Params.Limit)),
		MinDate:    request.Params.MinDate,
		MaxDate:    request.Params.MaxDate,
		HasUserID:  request.Params.HasUserId,
	})
	if err != nil {
		return nil, err
	}

	converted := make([]api.ActivityLogEntry, len(entries))
	for i, entry := range entries {
		converted[i] = entryDto(entry)
	}

	return api.GetLogEntries200JSONResponse{
		Items:            &converted,
		StartIndex:       apiutil.Ptr(startIndex),
		TotalRecordCount: apiutil.Ptr(int32(total)),
	}, nil
}
