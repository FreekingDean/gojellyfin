package apikey

import (
	"context"

	"github.com/FreekingDean/gojellyfin/internal/apikeys"
	"github.com/FreekingDean/gojellyfin/internal/auth"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
)

type Server struct {
	apikeys *apikeys.Service
}

func New(apikeys *apikeys.Service) *Server {
	return &Server{apikeys: apikeys}
}

func (s *Server) GetKeys(ctx context.Context, request api.GetKeysRequestObject) (api.GetKeysResponseObject, error) {
	keys, err := s.apikeys.Keys(ctx)
	if err != nil {
		return nil, err
	}

	infos := make([]api.AuthenticationInfo, 0, len(keys))
	for _, key := range keys {
		infos = append(infos, authenticationInfo(key))
	}

	return api.GetKeys200JSONResponse{
		Items:            &infos,
		StartIndex:       apiutil.Ptr(int32(0)),
		TotalRecordCount: apiutil.Ptr(int32(len(infos))),
	}, nil
}

func (s *Server) CreateKey(ctx context.Context, request api.CreateKeyRequestObject) (api.CreateKeyResponseObject, error) {
	token, err := auth.NewToken()
	if err != nil {
		return nil, err
	}

	if _, err := s.apikeys.Create(ctx, request.Params.App, token); err != nil {
		return nil, err
	}

	return api.CreateKey204Response{}, nil
}

func (s *Server) RevokeKey(ctx context.Context, request api.RevokeKeyRequestObject) (api.RevokeKeyResponseObject, error) {
	if err := s.apikeys.Revoke(ctx, request.Key); err != nil {
		return nil, err
	}

	return api.RevokeKey204Response{}, nil
}
