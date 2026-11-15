package reports

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/auth"
	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/reports"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
	"github.com/FreekingDean/gojellyfin/internal/server/dto"
	itemmodel "github.com/FreekingDean/gojellyfin/internal/store/item"
)

type Queue interface {
	File(ctx context.Context, filing reports.Filing) error
	Queue(ctx context.Context) ([]*reports.Report, error)
	Resolve(ctx context.Context, reportID uuid.UUID) error
	Dismiss(ctx context.Context, reportID uuid.UUID) error
	Replace(ctx context.Context, reportID uuid.UUID) error
}

type Library interface {
	ItemByID(ctx context.Context, viewer items.Viewer, id uuid.UUID) (*items.Item, error)
}

type Server struct {
	reports  Queue
	items    Library
	policies dto.Access
}

func New(reports *reports.Service, items *items.Service, policies dto.Access) *Server {
	return &Server{reports: reports, items: items, policies: policies}
}

func (s *Server) GoJellyfinListReports(
	ctx context.Context,
	_ api.GoJellyfinListReportsRequestObject,
) (api.GoJellyfinListReportsResponseObject, error) {
	filed, err := s.reports.Queue(ctx)
	if err != nil {
		return nil, err
	}

	listed := make(api.GoJellyfinListReports200JSONResponse, len(filed))
	for i, report := range filed {
		listed[i] = reportDto(report)
	}

	return listed, nil
}

func (s *Server) GoJellyfinFileReport(
	ctx context.Context,
	request api.GoJellyfinFileReportRequestObject,
) (api.GoJellyfinFileReportResponseObject, error) {
	userID := auth.UserID(ctx)
	if userID == uuid.Nil {
		return nil, auth.ErrUnauthorized
	}
	if request.Body == nil {
		return api.GoJellyfinFileReport400JSONResponse{Message: "no request body"}, nil
	}

	kind := reports.Kind(request.Body.Kind)
	if err := reports.ValidKind(kind); err != nil {
		return api.GoJellyfinFileReport400JSONResponse{Message: fmt.Sprintf("unknown kind of report %q", request.Body.Kind)}, nil
	}

	viewer, err := dto.ViewerFor(ctx, s.policies)
	if err != nil {
		return nil, err
	}
	if _, err := s.items.ItemByID(ctx, viewer, request.Body.ItemId); err != nil {
		return api.GoJellyfinFileReport404Response{}, nil
	}

	err = s.reports.File(ctx, reports.Filing{
		ItemID:  request.Body.ItemId,
		UserID:  &userID,
		Kind:    kind,
		Origin:  reports.OriginUser,
		Note:    strings.TrimSpace(apiutil.Deref(request.Body.Note)),
		Attempt: reports.AttemptBy(auth.SessionFrom(ctx)),
	})
	if err != nil {
		return nil, err
	}

	return api.GoJellyfinFileReport204Response{}, nil
}

func (s *Server) GoJellyfinResolveReport(
	ctx context.Context,
	request api.GoJellyfinResolveReportRequestObject,
) (api.GoJellyfinResolveReportResponseObject, error) {
	err := s.reports.Resolve(ctx, request.ReportId)
	if errors.Is(err, reports.ErrNotFound) {
		return api.GoJellyfinResolveReport404Response{}, nil
	}
	if err != nil {
		return nil, err
	}

	return api.GoJellyfinResolveReport204Response{}, nil
}

func (s *Server) GoJellyfinDismissReport(
	ctx context.Context,
	request api.GoJellyfinDismissReportRequestObject,
) (api.GoJellyfinDismissReportResponseObject, error) {
	err := s.reports.Dismiss(ctx, request.ReportId)
	if errors.Is(err, reports.ErrNotFound) {
		return api.GoJellyfinDismissReport404Response{}, nil
	}
	if err != nil {
		return nil, err
	}

	return api.GoJellyfinDismissReport204Response{}, nil
}

func (s *Server) GoJellyfinReplaceReported(
	ctx context.Context,
	request api.GoJellyfinReplaceReportedRequestObject,
) (api.GoJellyfinReplaceReportedResponseObject, error) {
	err := s.reports.Replace(ctx, request.ReportId)
	switch {
	case errors.Is(err, reports.ErrNotFound):
		return api.GoJellyfinReplaceReported404Response{}, nil
	case err != nil:
		return api.GoJellyfinReplaceReported400JSONResponse{Message: err.Error()}, nil
	}

	return api.GoJellyfinReplaceReported204Response{}, nil
}

func reportDto(filed *reports.Report) api.Report {
	listed := api.Report{
		Id:          filed.ID,
		ItemId:      filed.ItemID,
		ItemName:    titled(filed.Item),
		Kind:        string(filed.Kind),
		Origin:      string(filed.Origin),
		Note:        filed.Note,
		Status:      string(filed.Status),
		CreatedAt:   filed.CreatedAt,
		Replaceable: reports.Replaceable(filed),
	}
	if filed.User != nil {
		listed.UserName = apiutil.Ptr(filed.User.Name)
	}
	if filed.File != nil {
		listed.Path = apiutil.Ptr(filed.File.Path)
	}
	if filed.Attempt != nil {
		listed.Attempt = attemptDto(filed.Attempt)
	}

	return listed
}

func attemptDto(attempt *reports.Attempt) *api.ReportAttempt {
	listed := &api.ReportAttempt{
		UserId:        attempt.UserID,
		UserName:      apiutil.ZeroOrNilPtr(attempt.UserName),
		DeviceId:      apiutil.ZeroOrNilPtr(attempt.DeviceID),
		DeviceName:    apiutil.ZeroOrNilPtr(attempt.DeviceName),
		Client:        apiutil.ZeroOrNilPtr(attempt.Client),
		ClientVersion: apiutil.ZeroOrNilPtr(attempt.ClientVersion),
		RemoteAddress: apiutil.ZeroOrNilPtr(attempt.RemoteAddress),
		Build:         apiutil.ZeroOrNilPtr(attempt.Build),
	}

	served := attempt.Playback
	if served == nil {
		return listed
	}

	listed.Direct = apiutil.Ptr(served.Direct)
	listed.VideoDirect = apiutil.Ptr(served.VideoDirect)
	listed.AudioDirect = apiutil.Ptr(served.AudioDirect)
	listed.Container = apiutil.ZeroOrNilPtr(served.Container)
	listed.VideoCodec = apiutil.ZeroOrNilPtr(served.VideoCodec)
	listed.AudioCodec = apiutil.ZeroOrNilPtr(served.AudioCodec)
	listed.Reasons = apiutil.Ptr(append([]string{}, served.Reasons...))
	listed.StartTicks = apiutil.Ptr(attempt.StartTicks)

	return listed
}

func titled(item *items.Item) string {
	if item == nil {
		return ""
	}
	if item.Kind != itemmodel.KindEpisode || item.Parent == nil || item.Parent.Parent == nil {
		return item.Name
	}

	return fmt.Sprintf("%s S%dE%d, %s",
		item.Parent.Parent.Name,
		apiutil.Deref(item.ParentIndexNumber),
		apiutil.Deref(item.IndexNumber),
		item.Name,
	)
}
