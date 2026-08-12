package activitylog

import (
	"context"

	"github.com/FreekingDean/gojellyfin/internal/activity"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
)

type Server struct {
	activity *activity.Service
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
		HasUserID:  request.Params.HasUserId,
	})
	if err != nil {
		return nil, err
	}

	converted := make([]api.ActivityLogEntry, 0, len(entries))
	for _, entry := range entries {
		converted = append(converted, entryDto(entry))
	}

	return api.GetLogEntries200JSONResponse{
		Items:            &converted,
		StartIndex:       apiutil.Ptr(startIndex),
		TotalRecordCount: apiutil.Ptr(int32(total)),
	}, nil
}
