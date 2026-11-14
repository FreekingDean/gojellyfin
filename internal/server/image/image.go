package image

import (
	"bytes"
	"context"
	"net/http"
	"strconv"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/collage"
	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
)

const cacheFor = "public, max-age=86400"

type Server struct {
	items    *items.Service
	collages *collage.Service
}

func New(items *items.Service, collages *collage.Service) *Server {
	return &Server{items: items, collages: collages}
}

func (s *Server) GetItemImage(ctx context.Context, request api.GetItemImageRequestObject) (api.GetItemImageResponseObject, error) {
	answer, ok := s.answer(ctx, request.ItemId, request.ImageType, apiutil.Deref(request.Params.ImageIndex))
	if !ok {
		return api.GetItemImage404JSONResponse{}, nil
	}

	return answer, nil
}

func (s *Server) HeadItemImage(ctx context.Context, request api.HeadItemImageRequestObject) (api.HeadItemImageResponseObject, error) {
	answer, ok := s.answer(ctx, request.ItemId, request.ImageType, apiutil.Deref(request.Params.ImageIndex))
	if !ok {
		return api.HeadItemImage404JSONResponse{}, nil
	}

	return answer, nil
}

func (s *Server) GetItemImageByIndex(ctx context.Context, request api.GetItemImageByIndexRequestObject) (api.GetItemImageByIndexResponseObject, error) {
	answer, ok := s.answer(ctx, request.ItemId, request.ImageType, request.ImageIndex)
	if !ok {
		return api.GetItemImageByIndex404JSONResponse{}, nil
	}

	return answer, nil
}

func (s *Server) HeadItemImageByIndex(ctx context.Context, request api.HeadItemImageByIndexRequestObject) (api.HeadItemImageByIndexResponseObject, error) {
	answer, ok := s.answer(ctx, request.ItemId, request.ImageType, request.ImageIndex)
	if !ok {
		return api.HeadItemImageByIndex404JSONResponse{}, nil
	}

	return answer, nil
}

type answer interface {
	api.GetItemImageResponseObject
	api.HeadItemImageResponseObject
	api.GetItemImageByIndexResponseObject
	api.HeadItemImageByIndexResponseObject
}

func (s *Server) answer(ctx context.Context, itemID uuid.UUID, imageType api.ImageType, index int32) (answer, bool) {
	kind := items.ImageKind(imageType)
	if items.ValidImageKind(kind) != nil {
		return nil, false
	}

	record, err := s.items.Image(ctx, itemID, kind, index)
	if err == nil {
		return redirect(record.URL), true
	}
	if kind != items.ImageKindPrimary || index != 0 {
		return nil, false
	}

	built, ok := s.collages.Image(ctx, itemID)
	if !ok {
		return nil, false
	}

	return composed(built), true
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

type composed collage.Collage

func (c composed) write(w http.ResponseWriter) error {
	w.Header().Set("Content-Type", collage.ContentType)
	w.Header().Set("Content-Length", strconv.Itoa(len(c.Body)))
	w.Header().Set("Cache-Control", c.caching())
	w.WriteHeader(http.StatusOK)

	_, err := bytes.NewReader(c.Body).WriteTo(w)

	return err
}

func (c composed) caching() string {
	if c.Complete {
		return cacheFor
	}

	return "no-store"
}

func (c composed) VisitGetItemImageResponse(w http.ResponseWriter) error         { return c.write(w) }
func (c composed) VisitHeadItemImageResponse(w http.ResponseWriter) error        { return c.write(w) }
func (c composed) VisitGetItemImageByIndexResponse(w http.ResponseWriter) error  { return c.write(w) }
func (c composed) VisitHeadItemImageByIndexResponse(w http.ResponseWriter) error { return c.write(w) }
