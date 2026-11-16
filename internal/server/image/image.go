package image

import (
	"bytes"
	"context"
	"net/http"
	"strconv"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/auth"
	"github.com/FreekingDean/gojellyfin/internal/cards"
	"github.com/FreekingDean/gojellyfin/internal/channels"
	"github.com/FreekingDean/gojellyfin/internal/discover"
	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
	"github.com/FreekingDean/gojellyfin/internal/server/dto"
	"github.com/FreekingDean/gojellyfin/internal/server/link"
)

const cacheFor = "public, max-age=86400"

type Splash interface {
	Enabled() bool
	Splashscreen(deviceID string) ([]byte, error)
}

type Server struct {
	items    *items.Service
	cards    *cards.Service
	discover *discover.Service
	channels *channels.Service
	splash   Splash
	policies dto.Access
}

func New(items *items.Service, cards *cards.Service, discover *discover.Service, links *link.Handler, policies dto.Access, channels *channels.Service) *Server {
	return &Server{items: items, cards: cards, discover: discover, splash: links, policies: policies, channels: channels}
}

func (s *Server) GetSplashscreen(ctx context.Context, request api.GetSplashscreenRequestObject) (api.GetSplashscreenResponseObject, error) {
	if !s.splash.Enabled() {
		return nil, api.ErrNotImplemented
	}

	picture, err := s.splash.Splashscreen(auth.AuthorizationFrom(ctx).DeviceID)
	if err != nil {
		return nil, err
	}

	return api.GetSplashscreen200ImageResponse{
		Body:          bytes.NewReader(picture),
		ContentType:   "image/png",
		ContentLength: int64(len(picture)),
	}, nil
}

func (s *Server) GetItemImage(ctx context.Context, request api.GetItemImageRequestObject) (api.GetItemImageResponseObject, error) {
	answer, ok := s.answer(ctx, request.ItemId, request.ImageType, apiutil.Deref(request.Params.ImageIndex), apiutil.Deref(request.Params.Tag))
	if !ok {
		return api.GetItemImage404JSONResponse{}, nil
	}

	return answer, nil
}

func (s *Server) HeadItemImage(ctx context.Context, request api.HeadItemImageRequestObject) (api.HeadItemImageResponseObject, error) {
	answer, ok := s.answer(ctx, request.ItemId, request.ImageType, apiutil.Deref(request.Params.ImageIndex), apiutil.Deref(request.Params.Tag))
	if !ok {
		return api.HeadItemImage404JSONResponse{}, nil
	}

	return answer, nil
}

func (s *Server) GetItemImageByIndex(ctx context.Context, request api.GetItemImageByIndexRequestObject) (api.GetItemImageByIndexResponseObject, error) {
	answer, ok := s.answer(ctx, request.ItemId, request.ImageType, request.ImageIndex, apiutil.Deref(request.Params.Tag))
	if !ok {
		return api.GetItemImageByIndex404JSONResponse{}, nil
	}

	return answer, nil
}

func (s *Server) HeadItemImageByIndex(ctx context.Context, request api.HeadItemImageByIndexRequestObject) (api.HeadItemImageByIndexResponseObject, error) {
	answer, ok := s.answer(ctx, request.ItemId, request.ImageType, request.ImageIndex, apiutil.Deref(request.Params.Tag))
	if !ok {
		return api.HeadItemImageByIndex404JSONResponse{}, nil
	}

	return answer, nil
}

func (s *Server) GetItemImage2(ctx context.Context, request api.GetItemImage2RequestObject) (api.GetItemImage2ResponseObject, error) {
	answer, ok := s.answer(ctx, request.ItemId, request.ImageType, request.ImageIndex, request.Tag)
	if !ok {
		return api.GetItemImage2404JSONResponse{}, nil
	}

	return answer, nil
}

func (s *Server) HeadItemImage2(ctx context.Context, request api.HeadItemImage2RequestObject) (api.HeadItemImage2ResponseObject, error) {
	answer, ok := s.answer(ctx, request.ItemId, request.ImageType, request.ImageIndex, request.Tag)
	if !ok {
		return api.HeadItemImage2404JSONResponse{}, nil
	}

	return answer, nil
}

func (s *Server) GetItemImageInfos(ctx context.Context, request api.GetItemImageInfosRequestObject) (api.GetItemImageInfosResponseObject, error) {
	viewer, err := dto.ViewerFor(ctx, s.policies)
	if err != nil {
		return nil, err
	}

	if _, err := s.items.ItemByID(ctx, viewer, request.ItemId); err != nil {
		return api.GetItemImageInfos404JSONResponse{}, nil
	}

	records, err := s.items.Images(ctx, request.ItemId)
	if err != nil {
		return nil, err
	}

	infos := make(api.GetItemImageInfos200JSONResponse, 0, len(records))
	for _, record := range records {
		infos = append(infos, api.ImageInfo{
			ImageType:  apiutil.Ptr(api.ImageType(record.Kind)),
			ImageIndex: apiutil.Ptr(record.Index),
			ImageTag:   apiutil.Ptr(record.Tag),
		})
	}

	return infos, nil
}

func (s *Server) GetUserImage(context.Context, api.GetUserImageRequestObject) (api.GetUserImageResponseObject, error) {
	return api.GetUserImage404JSONResponse{}, nil
}

func (s *Server) HeadUserImage(context.Context, api.HeadUserImageRequestObject) (api.HeadUserImageResponseObject, error) {
	return api.HeadUserImage404JSONResponse{}, nil
}

type answer interface {
	api.GetItemImage2ResponseObject
	api.HeadItemImage2ResponseObject
	api.GetItemImageResponseObject
	api.HeadItemImageResponseObject
	api.GetItemImageByIndexResponseObject
	api.HeadItemImageByIndexResponseObject
}

func (s *Server) answer(ctx context.Context, itemID uuid.UUID, imageType api.ImageType, index int32, tag string) (answer, bool) {
	kind := items.ImageKind(imageType)
	if items.ValidImageKind(kind) != nil {
		return nil, false
	}

	if program, err := s.channels.Program(ctx, items.Everyone, itemID); err == nil {
		itemID = program.ItemID
	}

	record, err := s.items.Image(ctx, itemID, kind, index)
	if err == nil {
		return redirect(record.URL), true
	}
	if index != 0 {
		return nil, false
	}
	if kind == items.ImageKindPrimary {
		if url, err := s.items.PersonImage(ctx, itemID); err == nil {
			return redirect(url), true
		}
	}
	if title, found, err := s.discover.Summary(ctx, itemID); err == nil && found {
		url, found := title.Image(kind)

		return redirect(url), found
	}
	if kind != items.ImageKindPrimary {
		return nil, false
	}

	built, ok := s.cards.Image(ctx, itemID, tag)
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
func (r redirect) VisitGetItemImage2Response(w http.ResponseWriter) error        { return r.write(w) }
func (r redirect) VisitHeadItemImage2Response(w http.ResponseWriter) error       { return r.write(w) }

type composed cards.Picture

func (c composed) write(w http.ResponseWriter) error {
	w.Header().Set("Content-Type", cards.ContentType)
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
func (c composed) VisitGetItemImage2Response(w http.ResponseWriter) error        { return c.write(w) }
func (c composed) VisitHeadItemImage2Response(w http.ResponseWriter) error       { return c.write(w) }
