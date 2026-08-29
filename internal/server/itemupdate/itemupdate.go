package itemupdate

import (
	"context"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/libraries"
	"github.com/FreekingDean/gojellyfin/internal/localization"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
	"github.com/FreekingDean/gojellyfin/internal/server/dto"
)

type Server struct {
	items        *items.Service
	libraries    *libraries.Service
	localization *localization.Service
}

func New(items *items.Service, libraries *libraries.Service, localization *localization.Service) *Server {
	return &Server{items: items, libraries: libraries, localization: localization}
}

func (s *Server) UpdateItem(ctx context.Context, request api.UpdateItemRequestObject) (api.UpdateItemResponseObject, error) {
	req := apiutil.Body(request.JSONBody, request.ApplicationWildcardPlusJSONBody)
	if req == nil {
		return api.UpdateItem403Response{}, nil
	}

	item, err := s.items.ItemByID(ctx, request.ItemId)
	if err != nil {
		return api.UpdateItem404JSONResponse{}, nil
	}

	if _, err := s.items.EditMetadata(ctx, item, metadata(req)); err != nil {
		return nil, err
	}

	return api.UpdateItem204Response{}, nil
}

func (s *Server) UpdateItemContentType(ctx context.Context, request api.UpdateItemContentTypeRequestObject) (api.UpdateItemContentTypeResponseObject, error) {
	if _, err := s.items.ItemByID(ctx, request.ItemId); err != nil {
		return api.UpdateItemContentType404JSONResponse{}, nil
	}

	if !supportedContentType(request.Params.ContentType) {
		return api.UpdateItemContentType403Response{}, nil
	}

	return api.UpdateItemContentType204Response{}, nil
}

func (s *Server) GetMetadataEditorInfo(ctx context.Context, request api.GetMetadataEditorInfoRequestObject) (api.GetMetadataEditorInfoResponseObject, error) {
	item, err := s.items.ItemByID(ctx, request.ItemId)
	if err != nil {
		return api.GetMetadataEditorInfo404JSONResponse{}, nil
	}

	info := api.MetadataEditorInfo{
		ContentTypeOptions:    &[]api.NameValuePair{},
		Countries:             apiutil.Ptr(dto.CountryInfos(s.localization.Countries())),
		Cultures:              apiutil.Ptr(dto.CultureDtos(s.localization.Cultures())),
		ParentalRatingOptions: apiutil.Ptr(dto.ParentalRatings(s.localization.ParentalRatings())),
		ExternalIdInfos:       &[]api.ExternalIdInfo{},
	}
	if item.IsFolder {
		info.ContentTypeOptions = apiutil.Ptr(contentTypeOptions())
	}

	if item.LibraryID != uuid.Nil {
		library, err := s.libraries.Library(ctx, item.LibraryID)
		if err != nil {
			return nil, err
		}
		info.ContentType = contentType(library.CollectionType)
	}

	return api.GetMetadataEditorInfo200JSONResponse(info), nil
}
