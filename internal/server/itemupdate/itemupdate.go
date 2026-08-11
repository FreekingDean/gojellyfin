package itemupdate

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/FreekingDean/gojellyfin/internal/config"
	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
	"github.com/FreekingDean/gojellyfin/internal/server/configuration"
	"github.com/FreekingDean/gojellyfin/internal/server/localization"
)

type Server struct {
	items  *items.Service
	config *config.Service
}

func New(items *items.Service, config *config.Service) *Server {
	return &Server{items: items, config: config}
}

func (s *Server) UpdateItem(ctx context.Context, request api.UpdateItemRequestObject) (api.UpdateItemResponseObject, error) {
	req := apiutil.Body(request.JSONBody, request.ApplicationWildcardPlusJSONBody)
	if req == nil {
		return api.UpdateItem403Response{}, nil
	}

	if _, err := s.items.ItemByID(ctx, request.ItemId); err != nil {
		return api.UpdateItem404JSONResponse{}, nil
	}

	if _, err := s.items.UpdateMetadata(ctx, request.ItemId, Metadata(req)); err != nil {
		return nil, err
	}

	return api.UpdateItem204Response{}, nil
}

func (s *Server) UpdateItemContentType(ctx context.Context, request api.UpdateItemContentTypeRequestObject) (api.UpdateItemContentTypeResponseObject, error) {
	item, err := s.items.ItemByID(ctx, request.ItemId)
	if err != nil {
		return api.UpdateItemContentType404JSONResponse{}, nil
	}

	server, err := configuration.ServerConfiguration(ctx, s.config)
	if err != nil {
		return nil, err
	}

	contentTypes := make([]api.NameValuePair, 0, len(apiutil.Deref(server.ContentTypes))+1)
	for _, pair := range apiutil.Deref(server.ContentTypes) {
		if !strings.EqualFold(apiutil.Deref(pair.Name), item.Path) {
			contentTypes = append(contentTypes, pair)
		}
	}
	if contentType := apiutil.Deref(request.Params.ContentType); contentType != "" {
		contentTypes = append(contentTypes, api.NameValuePair{
			Name:  apiutil.Ptr(item.Path),
			Value: apiutil.Ptr(contentType),
		})
	}
	server.ContentTypes = &contentTypes

	value, err := json.Marshal(server)
	if err != nil {
		return nil, err
	}
	if err := s.config.SetConfiguration(ctx, configuration.SystemConfigurationKey, value); err != nil {
		return nil, err
	}

	return api.UpdateItemContentType204Response{}, nil
}

func (s *Server) GetMetadataEditorInfo(ctx context.Context, request api.GetMetadataEditorInfoRequestObject) (api.GetMetadataEditorInfoResponseObject, error) {
	item, err := s.items.ItemByID(ctx, request.ItemId)
	if err != nil {
		return api.GetMetadataEditorInfo404JSONResponse{}, nil
	}

	server, err := configuration.ServerConfiguration(ctx, s.config)
	if err != nil {
		return nil, err
	}

	info := api.MetadataEditorInfo{
		ContentTypeOptions:    apiutil.Ptr(contentTypeOptions()),
		Countries:             apiutil.Ptr(localization.Countries()),
		Cultures:              apiutil.Ptr(localization.Cultures()),
		ParentalRatingOptions: apiutil.Ptr(localization.ParentalRatings()),
		ExternalIdInfos:       &[]api.ExternalIdInfo{},
	}
	if contentType := configuredContentType(server, item.Path); contentType != "" {
		info.ContentType = apiutil.Ptr(api.CollectionType(contentType))
	}

	return api.GetMetadataEditorInfo200JSONResponse(info), nil
}
