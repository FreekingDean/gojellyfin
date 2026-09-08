package image

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/blob"
	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
)

const cacheFor = "public, max-age=86400"

type Server struct {
	items *items.Service
	blob  *blob.Store
}

func New(items *items.Service, objects *blob.Store) *Server {
	return &Server{items: items, blob: objects}
}

type response interface {
	api.GetItemImageResponseObject
	api.HeadItemImageResponseObject
	api.GetItemImageByIndexResponseObject
	api.HeadItemImageByIndexResponseObject
}

func (s *Server) GetItemImage(ctx context.Context, request api.GetItemImageRequestObject) (api.GetItemImageResponseObject, error) {
	found, ok := s.served(ctx, request.ItemId, request.ImageType, apiutil.Deref(request.Params.ImageIndex))
	if !ok {
		return api.GetItemImage404JSONResponse{}, nil
	}

	return found, nil
}

func (s *Server) HeadItemImage(ctx context.Context, request api.HeadItemImageRequestObject) (api.HeadItemImageResponseObject, error) {
	found, ok := s.served(ctx, request.ItemId, request.ImageType, apiutil.Deref(request.Params.ImageIndex))
	if !ok {
		return api.HeadItemImage404JSONResponse{}, nil
	}

	return found, nil
}

func (s *Server) GetItemImageByIndex(ctx context.Context, request api.GetItemImageByIndexRequestObject) (api.GetItemImageByIndexResponseObject, error) {
	found, ok := s.served(ctx, request.ItemId, request.ImageType, request.ImageIndex)
	if !ok {
		return api.GetItemImageByIndex404JSONResponse{}, nil
	}

	return found, nil
}

func (s *Server) HeadItemImageByIndex(ctx context.Context, request api.HeadItemImageByIndexRequestObject) (api.HeadItemImageByIndexResponseObject, error) {
	found, ok := s.served(ctx, request.ItemId, request.ImageType, request.ImageIndex)
	if !ok {
		return api.HeadItemImageByIndex404JSONResponse{}, nil
	}

	return found, nil
}

func (s *Server) served(ctx context.Context, itemID uuid.UUID, imageType api.ImageType, index int32) (response, bool) {
	kind := items.ImageKind(imageType)
	if items.ValidImageKind(kind) != nil {
		return nil, false
	}

	record, err := s.items.Image(ctx, itemID, kind, index)
	if err != nil {
		return nil, false
	}

	if record.Key != "" {
		object, err := s.blob.Get(ctx, record.Key)
		if err == nil {
			return stored(object), true
		}

		log.Printf("image %s %s: %v", itemID, kind, err)
	}

	return redirect(record.URL), true
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

type stored blob.Object

func (o stored) headers(w http.ResponseWriter) {
	w.Header().Set("Content-Type", o.ContentType)
	w.Header().Set("Cache-Control", cacheFor)
	if o.Size >= 0 {
		w.Header().Set("Content-Length", strconv.FormatInt(o.Size, 10))
	}
	w.WriteHeader(http.StatusOK)
}

func (o stored) write(w http.ResponseWriter) error {
	defer func() { _ = o.Body.Close() }()

	o.headers(w)

	if _, err := io.Copy(w, o.Body); err != nil {
		return fmt.Errorf("failed to write the image: %w", err)
	}

	return nil
}

func (o stored) describe(w http.ResponseWriter) error {
	defer func() { _ = o.Body.Close() }()

	o.headers(w)

	return nil
}

func (o stored) VisitGetItemImageResponse(w http.ResponseWriter) error         { return o.write(w) }
func (o stored) VisitHeadItemImageResponse(w http.ResponseWriter) error        { return o.describe(w) }
func (o stored) VisitGetItemImageByIndexResponse(w http.ResponseWriter) error  { return o.write(w) }
func (o stored) VisitHeadItemImageByIndexResponse(w http.ResponseWriter) error { return o.describe(w) }
