package userdata

import (
	"context"
	"errors"
	"time"

	"github.com/FreekingDean/gojellyfin/internal/auth"
	"github.com/FreekingDean/gojellyfin/internal/discover"
	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/requests"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
	"github.com/FreekingDean/gojellyfin/internal/server/dto"
	"github.com/google/uuid"
)

type Unowned interface {
	Title(ctx context.Context, id uuid.UUID) (discover.Title, bool, error)
}

type Requests interface {
	Request(ctx context.Context, userID uuid.UUID, title discover.Title) error
	Withdraw(ctx context.Context, userID uuid.UUID, key string) error
}

type Server struct {
	items    *items.Service
	unowned  Unowned
	requests Requests
}

func New(items *items.Service, unowned *discover.Service, requests *requests.Service) *Server {
	return &Server{items: items, unowned: unowned, requests: requests}
}

func (s *Server) GetItemUserData(ctx context.Context, request api.GetItemUserDataRequestObject) (api.GetItemUserDataResponseObject, error) {
	datum, err := dto.CurrentUserDatum(ctx, s.items, request.ItemId)
	if err != nil {
		return api.GetItemUserData404JSONResponse{}, nil
	}

	return api.GetItemUserData200JSONResponse(dto.UserItemDataDto(datum)), nil
}

func (s *Server) MarkFavoriteItem(ctx context.Context, request api.MarkFavoriteItemRequestObject) (api.MarkFavoriteItemResponseObject, error) {
	datum, err := s.saveFavorite(ctx, request.ItemId, true)
	if refused(err) {
		return api.MarkFavoriteItem403Response{}, nil
	}
	if err != nil {
		return nil, err
	}

	return api.MarkFavoriteItem200JSONResponse(dto.UserItemDataDto(datum)), nil
}

func (s *Server) UnmarkFavoriteItem(ctx context.Context, request api.UnmarkFavoriteItemRequestObject) (api.UnmarkFavoriteItemResponseObject, error) {
	datum, err := s.saveFavorite(ctx, request.ItemId, false)
	if err != nil {
		return nil, err
	}

	return api.UnmarkFavoriteItem200JSONResponse(dto.UserItemDataDto(datum)), nil
}

func (s *Server) MarkPlayedItem(ctx context.Context, request api.MarkPlayedItemRequestObject) (api.MarkPlayedItemResponseObject, error) {
	datum, err := s.savePlayed(ctx, request.ItemId, true)
	if err != nil {
		return nil, err
	}

	return api.MarkPlayedItem200JSONResponse(dto.UserItemDataDto(datum)), nil
}

func (s *Server) MarkUnplayedItem(ctx context.Context, request api.MarkUnplayedItemRequestObject) (api.MarkUnplayedItemResponseObject, error) {
	datum, err := s.savePlayed(ctx, request.ItemId, false)
	if err != nil {
		return nil, err
	}

	return api.MarkUnplayedItem200JSONResponse(dto.UserItemDataDto(datum)), nil
}

func (s *Server) UpdateItemUserData(ctx context.Context, request api.UpdateItemUserDataRequestObject) (api.UpdateItemUserDataResponseObject, error) {
	req := apiutil.Body(request.JSONBody, request.ApplicationWildcardPlusJSONBody)
	if req == nil {
		return api.UpdateItemUserData404JSONResponse{}, nil
	}

	datum, err := dto.CurrentUserDatum(ctx, s.items, request.ItemId)
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

	return api.UpdateItemUserData200JSONResponse(dto.UserItemDataDto(datum)), nil
}

func refused(err error) bool {
	return errors.Is(err, requests.ErrNowhere) || errors.Is(err, requests.ErrNotAllowed) || errors.Is(err, requests.ErrQuota)
}

func (s *Server) saveFavorite(ctx context.Context, itemID uuid.UUID, favorite bool) (*items.Datum, error) {
	title, unowned, err := s.unowned.Title(ctx, itemID)
	if err != nil {
		return nil, err
	}
	if unowned {
		held, err := s.items.ItemByKey(ctx, title.Key)
		if err != nil || held.DeletedAt != nil {
			return s.saveRequest(ctx, title, favorite)
		}
		itemID = held.ID
	}

	datum, err := dto.CurrentUserDatum(ctx, s.items, itemID)
	if err != nil {
		return nil, err
	}

	datum.IsFavorite = favorite

	return datum, s.items.SaveUserItemDatum(ctx, datum)
}

func (s *Server) saveRequest(ctx context.Context, title discover.Title, wanted bool) (*items.Datum, error) {
	userID := auth.UserID(ctx)
	if userID == uuid.Nil {
		return nil, auth.ErrUnauthorized
	}

	datum := &items.Datum{UserID: userID, ItemID: title.ID, IsFavorite: wanted}
	if !wanted {
		return datum, s.requests.Withdraw(ctx, userID, title.Key)
	}

	return datum, s.requests.Request(ctx, userID, title)
}

func (s *Server) savePlayed(ctx context.Context, itemID uuid.UUID, played bool) (*items.Datum, error) {
	datum, err := dto.CurrentUserDatum(ctx, s.items, itemID)
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
