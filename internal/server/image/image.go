package image

import (
	"context"
	"io"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/artwork"
	"github.com/FreekingDean/gojellyfin/internal/filesystem"
	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/libraries"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
)

type Server struct {
	items      *items.Service
	libraries  *libraries.Service
	filesystem *filesystem.Service
	artwork    artwork.Store
}

func New(items *items.Service, catalogue *libraries.Service, filesystem *filesystem.Service, artwork artwork.Store) *Server {
	return &Server{items: items, libraries: catalogue, filesystem: filesystem, artwork: artwork}
}

func (s *Server) GetItemImage(ctx context.Context, request api.GetItemImageRequestObject) (api.GetItemImageResponseObject, error) {
	file, ok := s.open(ctx, request.ItemId, request.ImageType, apiutil.Deref(request.Params.ImageIndex))
	if !ok {
		return api.GetItemImage404JSONResponse{}, nil
	}

	return api.GetItemImage200ImageResponse{
		Body:          file.body,
		ContentType:   file.contentType,
		ContentLength: file.length,
	}, nil
}

func (s *Server) HeadItemImage(ctx context.Context, request api.HeadItemImageRequestObject) (api.HeadItemImageResponseObject, error) {
	file, ok := s.open(ctx, request.ItemId, request.ImageType, apiutil.Deref(request.Params.ImageIndex))
	if !ok {
		return api.HeadItemImage404JSONResponse{}, nil
	}

	return api.HeadItemImage200ImageResponse{
		Body:          file.body,
		ContentType:   file.contentType,
		ContentLength: file.length,
	}, nil
}

func (s *Server) GetItemImageByIndex(ctx context.Context, request api.GetItemImageByIndexRequestObject) (api.GetItemImageByIndexResponseObject, error) {
	file, ok := s.open(ctx, request.ItemId, request.ImageType, request.ImageIndex)
	if !ok {
		return api.GetItemImageByIndex404JSONResponse{}, nil
	}

	return api.GetItemImageByIndex200ImageResponse{
		Body:          file.body,
		ContentType:   file.contentType,
		ContentLength: file.length,
	}, nil
}

func (s *Server) HeadItemImageByIndex(ctx context.Context, request api.HeadItemImageByIndexRequestObject) (api.HeadItemImageByIndexResponseObject, error) {
	file, ok := s.open(ctx, request.ItemId, request.ImageType, request.ImageIndex)
	if !ok {
		return api.HeadItemImageByIndex404JSONResponse{}, nil
	}

	return api.HeadItemImageByIndex200ImageResponse{
		Body:          file.body,
		ContentType:   file.contentType,
		ContentLength: file.length,
	}, nil
}

func (s *Server) open(ctx context.Context, itemID uuid.UUID, imageType api.ImageType, index int32) (imageFile, bool) {
	kind := items.ImageKind(imageType)
	if items.ValidImageKind(kind) != nil {
		return imageFile{}, false
	}

	record, err := s.items.Image(ctx, itemID, kind, index)
	if err != nil {
		return s.openLibrary(ctx, itemID, kind, index)
	}

	body, size, ok := s.read(ctx, record)
	if !ok {
		return imageFile{}, false
	}

	return imageFile{body: body, contentType: contentType(record.Path), length: size}, true
}

func (s *Server) openLibrary(ctx context.Context, id uuid.UUID, kind items.ImageKind, index int32) (imageFile, bool) {
	if kind != items.ImageKindPrimary || index != 0 {
		return imageFile{}, false
	}

	library, err := s.libraries.Library(ctx, id)
	if err != nil || library.ImageTag == "" {
		return imageFile{}, false
	}

	key := libraries.ImageKey(library.ID, library.ImageTag)
	body, size, found, err := s.artwork.Open(ctx, key)
	if err != nil || !found {
		return imageFile{}, false
	}

	return imageFile{body: body, contentType: contentType(key), length: size}, true
}

func (s *Server) read(ctx context.Context, record *items.Image) (io.ReadCloser, int64, bool) {
	if record.Source == items.ImageSourceRemote {
		body, size, found, err := s.artwork.Open(ctx, record.Path)
		if err != nil || !found {
			return nil, 0, false
		}

		return body, size, true
	}

	body, size, err := s.filesystem.Open(ctx, record.Path)
	if err != nil {
		return nil, 0, false
	}

	return body, size, true
}
