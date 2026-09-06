package image

import (
	"context"
	"net/http"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
)

const cacheFor = "public, max-age=86400"

type Server struct {
	items *items.Service
}

func New(items *items.Service) *Server {
	return &Server{items: items}
}

func (s *Server) GetItemImage(ctx context.Context, request api.GetItemImageRequestObject) (api.GetItemImageResponseObject, error) {
	url, ok := s.url(ctx, request.ItemId, request.ImageType, apiutil.Deref(request.Params.ImageIndex))
	if !ok {
		return api.GetItemImage404JSONResponse{}, nil
	}

	return redirect(url), nil
}

func (s *Server) HeadItemImage(ctx context.Context, request api.HeadItemImageRequestObject) (api.HeadItemImageResponseObject, error) {
	url, ok := s.url(ctx, request.ItemId, request.ImageType, apiutil.Deref(request.Params.ImageIndex))
	if !ok {
		return api.HeadItemImage404JSONResponse{}, nil
	}

	return redirect(url), nil
}

func (s *Server) GetItemImageByIndex(ctx context.Context, request api.GetItemImageByIndexRequestObject) (api.GetItemImageByIndexResponseObject, error) {
	url, ok := s.url(ctx, request.ItemId, request.ImageType, request.ImageIndex)
	if !ok {
		return api.GetItemImageByIndex404JSONResponse{}, nil
	}

	return redirect(url), nil
}

func (s *Server) HeadItemImageByIndex(ctx context.Context, request api.HeadItemImageByIndexRequestObject) (api.HeadItemImageByIndexResponseObject, error) {
	url, ok := s.url(ctx, request.ItemId, request.ImageType, request.ImageIndex)
	if !ok {
		return api.HeadItemImageByIndex404JSONResponse{}, nil
	}

	return redirect(url), nil
}

func (s *Server) url(ctx context.Context, itemID uuid.UUID, imageType api.ImageType, index int32) (string, bool) {
	kind := items.ImageKind(imageType)
	if items.ValidImageKind(kind) != nil {
		return "", false
	}

	record, err := s.items.Image(ctx, itemID, kind, index)
	if err != nil {
		return "", false
	}

	return record.URL, true
}

type redirect string

func (r redirect) write(w http.ResponseWriter) error {
	w.Header().Set("Location", string(r))
	w.Header().Set("Cache-Control", cacheFor)
	w.WriteHeader(http.StatusFound)

	return nil
}

func (r redirect) VisitGetItemImageResponse(w http.ResponseWriter) error         { return r.write(w) }
func (r redirect) VisitHeadItemImageResponse(w http.ResponseWriter) error        { return r.write(w) }
func (r redirect) VisitGetItemImageByIndexResponse(w http.ResponseWriter) error  { return r.write(w) }
func (r redirect) VisitHeadItemImageByIndexResponse(w http.ResponseWriter) error { return r.write(w) }
