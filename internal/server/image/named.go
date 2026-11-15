package image

import (
	"context"
	"errors"
	"net/http"

	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	itemmodel "github.com/FreekingDean/gojellyfin/internal/store/item"
)

func (s *Server) GetArtistImage(ctx context.Context, request api.GetArtistImageRequestObject) (api.GetArtistImageResponseObject, error) {
	found, ok, err := s.artist(ctx, request.Name, request.ImageType, request.ImageIndex)
	if err != nil || !ok {
		return api.GetArtistImage404JSONResponse{}, err
	}

	return found, nil
}

func (s *Server) HeadArtistImage(ctx context.Context, request api.HeadArtistImageRequestObject) (api.HeadArtistImageResponseObject, error) {
	found, ok, err := s.artist(ctx, request.Name, request.ImageType, request.ImageIndex)
	if err != nil || !ok {
		return api.HeadArtistImage404JSONResponse{}, err
	}

	return found, nil
}

func (s *Server) GetPersonImage(ctx context.Context, request api.GetPersonImageRequestObject) (api.GetPersonImageResponseObject, error) {
	found, ok, err := s.person(ctx, request.Name, request.ImageType, 0)
	if err != nil || !ok {
		return api.GetPersonImage404JSONResponse{}, err
	}

	return found, nil
}

func (s *Server) HeadPersonImage(ctx context.Context, request api.HeadPersonImageRequestObject) (api.HeadPersonImageResponseObject, error) {
	found, ok, err := s.person(ctx, request.Name, request.ImageType, 0)
	if err != nil || !ok {
		return api.HeadPersonImage404JSONResponse{}, err
	}

	return found, nil
}

func (s *Server) GetPersonImageByIndex(ctx context.Context, request api.GetPersonImageByIndexRequestObject) (api.GetPersonImageByIndexResponseObject, error) {
	found, ok, err := s.person(ctx, request.Name, request.ImageType, request.ImageIndex)
	if err != nil || !ok {
		return api.GetPersonImageByIndex404JSONResponse{}, err
	}

	return found, nil
}

func (s *Server) HeadPersonImageByIndex(ctx context.Context, request api.HeadPersonImageByIndexRequestObject) (api.HeadPersonImageByIndexResponseObject, error) {
	found, ok, err := s.person(ctx, request.Name, request.ImageType, request.ImageIndex)
	if err != nil || !ok {
		return api.HeadPersonImageByIndex404JSONResponse{}, err
	}

	return found, nil
}

func (s *Server) GetGenreImage(context.Context, api.GetGenreImageRequestObject) (api.GetGenreImageResponseObject, error) {
	return api.GetGenreImage404JSONResponse{}, nil
}

func (s *Server) HeadGenreImage(context.Context, api.HeadGenreImageRequestObject) (api.HeadGenreImageResponseObject, error) {
	return api.HeadGenreImage404JSONResponse{}, nil
}

func (s *Server) GetGenreImageByIndex(context.Context, api.GetGenreImageByIndexRequestObject) (api.GetGenreImageByIndexResponseObject, error) {
	return api.GetGenreImageByIndex404JSONResponse{}, nil
}

func (s *Server) HeadGenreImageByIndex(context.Context, api.HeadGenreImageByIndexRequestObject) (api.HeadGenreImageByIndexResponseObject, error) {
	return api.HeadGenreImageByIndex404JSONResponse{}, nil
}

func (s *Server) GetMusicGenreImage(context.Context, api.GetMusicGenreImageRequestObject) (api.GetMusicGenreImageResponseObject, error) {
	return api.GetMusicGenreImage404JSONResponse{}, nil
}

func (s *Server) HeadMusicGenreImage(context.Context, api.HeadMusicGenreImageRequestObject) (api.HeadMusicGenreImageResponseObject, error) {
	return api.HeadMusicGenreImage404JSONResponse{}, nil
}

func (s *Server) GetMusicGenreImageByIndex(context.Context, api.GetMusicGenreImageByIndexRequestObject) (api.GetMusicGenreImageByIndexResponseObject, error) {
	return api.GetMusicGenreImageByIndex404JSONResponse{}, nil
}

func (s *Server) HeadMusicGenreImageByIndex(context.Context, api.HeadMusicGenreImageByIndexRequestObject) (api.HeadMusicGenreImageByIndexResponseObject, error) {
	return api.HeadMusicGenreImageByIndex404JSONResponse{}, nil
}

func (s *Server) GetStudioImage(context.Context, api.GetStudioImageRequestObject) (api.GetStudioImageResponseObject, error) {
	return api.GetStudioImage404JSONResponse{}, nil
}

func (s *Server) HeadStudioImage(context.Context, api.HeadStudioImageRequestObject) (api.HeadStudioImageResponseObject, error) {
	return api.HeadStudioImage404JSONResponse{}, nil
}

func (s *Server) GetStudioImageByIndex(context.Context, api.GetStudioImageByIndexRequestObject) (api.GetStudioImageByIndexResponseObject, error) {
	return api.GetStudioImageByIndex404JSONResponse{}, nil
}

func (s *Server) HeadStudioImageByIndex(context.Context, api.HeadStudioImageByIndexRequestObject) (api.HeadStudioImageByIndexResponseObject, error) {
	return api.HeadStudioImageByIndex404JSONResponse{}, nil
}

func (s *Server) artist(ctx context.Context, name string, imageType api.ImageType, index int32) (redirect, bool, error) {
	kind := items.ImageKind(imageType)
	if items.ValidImageKind(kind) != nil {
		return "", false, nil
	}

	artist, err := s.items.NamedItem(ctx, items.Everyone, itemmodel.KindMusicArtist, name)
	if err != nil {
		return "", false, missing(err)
	}

	record, err := s.items.Image(ctx, artist.ID, kind, index)
	if err != nil {
		return "", false, nil
	}

	return redirect(record.URL), true, nil
}

func (s *Server) person(ctx context.Context, name string, imageType api.ImageType, index int32) (redirect, bool, error) {
	if items.ImageKind(imageType) != items.ImageKindPrimary || index != 0 {
		return "", false, nil
	}

	url, err := s.items.PersonImageNamed(ctx, name)
	if err != nil {
		return "", false, missing(err)
	}

	return redirect(url), true, nil
}

func missing(err error) error {
	if errors.Is(err, items.ErrNotFound) {
		return nil
	}

	return err
}

func (r redirect) VisitGetArtistImageResponse(w http.ResponseWriter) error         { return r.write(w) }
func (r redirect) VisitHeadArtistImageResponse(w http.ResponseWriter) error        { return r.write(w) }
func (r redirect) VisitGetPersonImageResponse(w http.ResponseWriter) error         { return r.write(w) }
func (r redirect) VisitHeadPersonImageResponse(w http.ResponseWriter) error        { return r.write(w) }
func (r redirect) VisitGetPersonImageByIndexResponse(w http.ResponseWriter) error  { return r.write(w) }
func (r redirect) VisitHeadPersonImageByIndexResponse(w http.ResponseWriter) error { return r.write(w) }
