package library

import (
	"context"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/auth"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
)

func (s *Server) DeleteItem(ctx context.Context, request api.DeleteItemRequestObject) (api.DeleteItemResponseObject, error) {
	allowed, err := s.users.MayDeleteContent(ctx, auth.UserID(ctx))
	if err != nil {
		return nil, err
	}
	if !allowed {
		return api.DeleteItem403Response{}, nil
	}

	if err := s.deleteItems(ctx, []uuid.UUID{request.ItemId}); err != nil {
		return nil, err
	}

	return api.DeleteItem204Response{}, nil
}

func (s *Server) DeleteItems(ctx context.Context, request api.DeleteItemsRequestObject) (api.DeleteItemsResponseObject, error) {
	allowed, err := s.users.MayDeleteContent(ctx, auth.UserID(ctx))
	if err != nil {
		return nil, err
	}
	if !allowed {
		return api.DeleteItems403Response{}, nil
	}

	if err := s.deleteItems(ctx, apiutil.Deref(request.Params.Ids)); err != nil {
		return nil, err
	}

	return api.DeleteItems204Response{}, nil
}

func (s *Server) deleteItems(ctx context.Context, ids []uuid.UUID) error {
	records, err := s.itemsByID(ctx, ids)
	if err != nil {
		return err
	}

	for _, record := range records {
		if err := s.filesystem.RemoveAll(ctx, record.Path); err != nil {
			return err
		}
		if err := s.items.DeleteItem(ctx, record.ID); err != nil {
			return err
		}
	}

	return nil
}
