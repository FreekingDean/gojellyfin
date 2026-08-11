package devices

import (
	"context"

	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
	"github.com/FreekingDean/gojellyfin/internal/sessions"
)

type Server struct {
	sessions *sessions.Service
}

func New(sessions *sessions.Service) *Server {
	return &Server{sessions: sessions}
}

func (s *Server) GetDevices(ctx context.Context, request api.GetDevicesRequestObject) (api.GetDevicesResponseObject, error) {
	devices, err := s.sessions.Devices(ctx)
	if err != nil {
		return nil, err
	}

	items := make([]api.DeviceInfoDto, 0, len(devices))
	for _, device := range devices {
		items = append(items, DeviceInfoDto(device))
	}

	return api.GetDevices200JSONResponse{
		Items:            &items,
		StartIndex:       apiutil.Ptr(int32(0)),
		TotalRecordCount: apiutil.Ptr(int32(len(items))),
	}, nil
}

func (s *Server) GetDeviceInfo(ctx context.Context, request api.GetDeviceInfoRequestObject) (api.GetDeviceInfoResponseObject, error) {
	device, err := s.sessions.DeviceByClientID(ctx, request.Params.Id)
	if err != nil {
		return api.GetDeviceInfo404JSONResponse{}, nil
	}

	return api.GetDeviceInfo200JSONResponse(DeviceInfoDto(device)), nil
}

func (s *Server) GetDeviceOptions(ctx context.Context, request api.GetDeviceOptionsRequestObject) (api.GetDeviceOptionsResponseObject, error) {
	device, err := s.sessions.DeviceByClientID(ctx, request.Params.Id)
	if err != nil {
		return api.GetDeviceOptions404JSONResponse{}, nil
	}

	return api.GetDeviceOptions200JSONResponse(DeviceOptionsDto(device)), nil
}

func (s *Server) UpdateDeviceOptions(ctx context.Context, request api.UpdateDeviceOptionsRequestObject) (api.UpdateDeviceOptionsResponseObject, error) {
	req := apiutil.Body(request.JSONBody, request.ApplicationWildcardPlusJSONBody)
	if req == nil {
		return api.UpdateDeviceOptions403Response{}, nil
	}

	if _, err := s.sessions.DeviceByClientID(ctx, request.Params.Id); err != nil {
		return api.UpdateDeviceOptions403Response{}, nil
	}

	if err := s.sessions.RenameDevice(ctx, request.Params.Id, apiutil.Deref(req.CustomName)); err != nil {
		return nil, err
	}

	return api.UpdateDeviceOptions204Response{}, nil
}

func (s *Server) DeleteDevice(ctx context.Context, request api.DeleteDeviceRequestObject) (api.DeleteDeviceResponseObject, error) {
	if _, err := s.sessions.DeviceByClientID(ctx, request.Params.Id); err != nil {
		return api.DeleteDevice404JSONResponse{}, nil
	}

	if err := s.sessions.RemoveDevice(ctx, request.Params.Id); err != nil {
		return nil, err
	}

	return api.DeleteDevice204Response{}, nil
}
