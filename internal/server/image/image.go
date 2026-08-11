package image

import (
	"context"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
)

type Server struct {
	items *items.Service
}

func New(items *items.Service) *Server {
	return &Server{items: items}
}

func (s *Server) GetItemImage(ctx context.Context, request api.GetItemImageRequestObject) (api.GetItemImageResponseObject, error) {
	file, ok := s.open(ctx, request.ItemId, request.ImageType, apiutil.Deref(request.Params.ImageIndex), request.Params.Tag)
	if !ok {
		return api.GetItemImage404JSONResponse{}, nil
	}

	return file, nil
}

func (s *Server) HeadItemImage(ctx context.Context, request api.HeadItemImageRequestObject) (api.HeadItemImageResponseObject, error) {
	file, ok := s.open(ctx, request.ItemId, request.ImageType, apiutil.Deref(request.Params.ImageIndex), request.Params.Tag)
	if !ok {
		return api.HeadItemImage404JSONResponse{}, nil
	}

	return file.withoutBody(), nil
}

func (s *Server) GetItemImageByIndex(ctx context.Context, request api.GetItemImageByIndexRequestObject) (api.GetItemImageByIndexResponseObject, error) {
	file, ok := s.open(ctx, request.ItemId, request.ImageType, request.ImageIndex, request.Params.Tag)
	if !ok {
		return api.GetItemImageByIndex404JSONResponse{}, nil
	}

	return file, nil
}

func (s *Server) HeadItemImageByIndex(ctx context.Context, request api.HeadItemImageByIndexRequestObject) (api.HeadItemImageByIndexResponseObject, error) {
	file, ok := s.open(ctx, request.ItemId, request.ImageType, request.ImageIndex, request.Params.Tag)
	if !ok {
		return api.HeadItemImageByIndex404JSONResponse{}, nil
	}

	return file.withoutBody(), nil
}

func (s *Server) open(ctx context.Context, itemID uuid.UUID, imageType api.ImageType, index int32, tag *string) (*imageFile, bool) {
	kind := items.ImageKind(imageType)
	if items.ValidImageKind(kind) != nil {
		return nil, false
	}

	record, err := s.items.Image(ctx, itemID, kind, index)
	if err != nil {
		return nil, false
	}

	return openImage(record, tag)
}
