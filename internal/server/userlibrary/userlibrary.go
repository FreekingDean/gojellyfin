package userlibrary

import (
	"context"
	"time"

	"github.com/FreekingDean/gojellyfin/internal/auth"
	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
	serveritems "github.com/FreekingDean/gojellyfin/internal/server/items"
	"github.com/google/uuid"
)

type Server struct {
	items *items.Service
}

func New(items *items.Service) *Server {
	return &Server{items: items}
}

func (s *Server) GetItemUserData(ctx context.Context, request api.GetItemUserDataRequestObject) (api.GetItemUserDataResponseObject, error) {
	datum, err := s.userItemDatum(ctx, request.ItemId)
	if err != nil {
		return api.GetItemUserData404JSONResponse{}, nil
	}

	return api.GetItemUserData200JSONResponse(serveritems.UserItemDataDto(datum)), nil
}

func (s *Server) MarkFavoriteItem(ctx context.Context, request api.MarkFavoriteItemRequestObject) (api.MarkFavoriteItemResponseObject, error) {
	datum, err := s.saveFavorite(ctx, request.ItemId, true)
	if err != nil {
		return nil, err
	}

	return api.MarkFavoriteItem200JSONResponse(serveritems.UserItemDataDto(datum)), nil
}

func (s *Server) UnmarkFavoriteItem(ctx context.Context, request api.UnmarkFavoriteItemRequestObject) (api.UnmarkFavoriteItemResponseObject, error) {
	datum, err := s.saveFavorite(ctx, request.ItemId, false)
	if err != nil {
		return nil, err
	}

	return api.UnmarkFavoriteItem200JSONResponse(serveritems.UserItemDataDto(datum)), nil
}

func (s *Server) MarkPlayedItem(ctx context.Context, request api.MarkPlayedItemRequestObject) (api.MarkPlayedItemResponseObject, error) {
	datum, err := s.savePlayed(ctx, request.ItemId, true)
	if err != nil {
		return nil, err
	}

	return api.MarkPlayedItem200JSONResponse(serveritems.UserItemDataDto(datum)), nil
}

func (s *Server) MarkUnplayedItem(ctx context.Context, request api.MarkUnplayedItemRequestObject) (api.MarkUnplayedItemResponseObject, error) {
	datum, err := s.savePlayed(ctx, request.ItemId, false)
	if err != nil {
		return nil, err
	}

	return api.MarkUnplayedItem200JSONResponse(serveritems.UserItemDataDto(datum)), nil
}

func (s *Server) UpdateItemUserData(ctx context.Context, request api.UpdateItemUserDataRequestObject) (api.UpdateItemUserDataResponseObject, error) {
	req := apiutil.Body(request.JSONBody, request.ApplicationWildcardPlusJSONBody)
	if req == nil {
		return api.UpdateItemUserData404JSONResponse{}, nil
	}

	datum, err := s.userItemDatum(ctx, request.ItemId)
	if err != nil {
		return api.UpdateItemUserData404JSONResponse{}, nil
	}

	if req.Played != nil {
		datum.Played = *req.Played
	}
	if req.IsFavorite != nil {
		datum.IsFavorite = *req.IsFavorite
	}
	if req.PlayCount != nil {
		datum.PlayCount = *req.PlayCount
	}
	if req.PlaybackPositionTicks != nil {
		datum.PlaybackPositionTicks = *req.PlaybackPositionTicks
	}
	if req.LastPlayedDate != nil {
		datum.LastPlayedAt = req.LastPlayedDate
	}

	if err := s.items.SaveUserItemDatum(ctx, datum); err != nil {
		return nil, err
	}

	return api.UpdateItemUserData200JSONResponse(serveritems.UserItemDataDto(datum)), nil
}

func (s *Server) saveFavorite(ctx context.Context, itemID uuid.UUID, favorite bool) (*items.Datum, error) {
	datum, err := s.userItemDatum(ctx, itemID)
	if err != nil {
		return nil, err
	}

	datum.IsFavorite = favorite

	return datum, s.items.SaveUserItemDatum(ctx, datum)
}

func (s *Server) savePlayed(ctx context.Context, itemID uuid.UUID, played bool) (*items.Datum, error) {
	datum, err := s.userItemDatum(ctx, itemID)
	if err != nil {
		return nil, err
	}

	datum.Played = played
	datum.PlaybackPositionTicks = 0
	if played {
		datum.PlayCount++
		datum.LastPlayedAt = apiutil.Ptr(time.Now())
	}

	return datum, s.items.SaveUserItemDatum(ctx, datum)
}

func (s *Server) userItemDatum(ctx context.Context, itemID uuid.UUID) (*items.Datum, error) {
	userID := auth.UserID(ctx)
	if userID == uuid.Nil {
		return nil, auth.ErrUnauthorized
	}

	return s.items.UserItemDatum(ctx, userID, itemID)
}
