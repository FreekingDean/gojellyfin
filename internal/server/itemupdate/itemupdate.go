package itemupdate

import (
	"context"

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

	item, err := s.items.ItemByID(ctx, items.Everyone, request.ItemId)
	if err != nil {
		return api.UpdateItem404JSONResponse{}, nil
	}

	if _, err := s.items.EditMetadata(ctx, item, metadata(req)); err != nil {
		return nil, err
	}

	return api.UpdateItem204Response{}, nil
}

func (s *Server) UpdateItemContentType(ctx context.Context, request api.UpdateItemContentTypeRequestObject) (api.UpdateItemContentTypeResponseObject, error) {
	item, err := s.items.ItemByID(ctx, items.Everyone, request.ItemId)
	if err != nil {
		return api.UpdateItemContentType404JSONResponse{}, nil
	}

	current, err := s.collectionType(ctx, item)
	if err != nil {
		return nil, err
	}
	if requested := apiutil.Deref(request.Params.ContentType); requested != "" && libraries.CollectionType(requested) != current {
		return api.UpdateItemContentType403Response{}, nil
	}

	return api.UpdateItemContentType204Response{}, nil
}

func (s *Server) GetMetadataEditorInfo(ctx context.Context, request api.GetMetadataEditorInfoRequestObject) (api.GetMetadataEditorInfoResponseObject, error) {
	item, err := s.items.ItemByID(ctx, items.Everyone, request.ItemId)
	if err != nil {
		return api.GetMetadataEditorInfo404JSONResponse{}, nil
	}

	current, err := s.collectionType(ctx, item)
	if err != nil {
		return nil, err
	}

	return api.GetMetadataEditorInfo200JSONResponse{
		ContentType:           contentType(current),
		ContentTypeOptions:    &[]api.NameValuePair{},
		Countries:             apiutil.Ptr(dto.CountryInfos(s.localization.Countries())),
		Cultures:              apiutil.Ptr(dto.CultureDtos(s.localization.Cultures())),
		ParentalRatingOptions: apiutil.Ptr(dto.ParentalRatings(s.localization.ParentalRatings())),
		ExternalIdInfos:       &[]api.ExternalIdInfo{},
	}, nil
}

func (s *Server) collectionType(ctx context.Context, item *items.Item) (libraries.CollectionType, error) {
	ancestry, err := s.items.Ancestors(ctx, item.ID)
	if err != nil {
		return "", err
	}
	if ancestry == nil || len(ancestry.LibraryIDs) == 0 {
		return "", nil
	}

	library, err := s.libraries.Library(ctx, ancestry.LibraryIDs[0])
	if err != nil {
		return "", err
	}

	return library.CollectionType, nil
}
