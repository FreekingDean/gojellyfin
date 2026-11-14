package library

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/auth"
	"github.com/FreekingDean/gojellyfin/internal/filesystem"
	"github.com/FreekingDean/gojellyfin/internal/reports"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
	"github.com/FreekingDean/gojellyfin/internal/server/dto"
)

func (s *Server) DeleteItem(ctx context.Context, request api.DeleteItemRequestObject) (api.DeleteItemResponseObject, error) {
	reported, err := s.reportBroken(ctx, request.ItemId)
	if err != nil {
		return nil, err
	}
	if reported {
		return api.DeleteItem204Response{}, nil
	}

	allowed, err := s.users.MayDeleteContent(ctx, auth.UserID(ctx))
	if err != nil {
		return nil, err
	}
	if !allowed {
		return api.DeleteItem403Response{}, nil
	}

	if err := s.deleteItems(ctx, []uuid.UUID{request.ItemId}); errors.Is(err, filesystem.ErrNotSupported) {
		return api.DeleteItem403Response{}, nil
	} else if err != nil {
		return nil, err
	}

	return api.DeleteItem204Response{}, nil
}

func (s *Server) reportBroken(ctx context.Context, itemID uuid.UUID) (bool, error) {
	userID := auth.UserID(ctx)
	if userID == uuid.Nil {
		return false, nil
	}

	viewer, err := dto.ViewerFor(ctx, s.users)
	if err != nil {
		return false, err
	}

	held, err := s.items.ItemsByIDs(ctx, viewer, []uuid.UUID{itemID})
	if err != nil {
		return false, err
	}

	item, ok := held[itemID]
	if !ok || !reports.Takes(item.Kind) {
		return false, nil
	}

	err = s.reports.File(ctx, reports.Filing{
		ItemID: itemID,
		UserID: &userID,
		Kind:   reports.KindUnplayable,
		Origin: reports.OriginDelete,
	})

	return err == nil, err
}

func (s *Server) DeleteItems(ctx context.Context, request api.DeleteItemsRequestObject) (api.DeleteItemsResponseObject, error) {
	allowed, err := s.users.MayDeleteContent(ctx, auth.UserID(ctx))
	if err != nil {
		return nil, err
	}
	if !allowed {
		return api.DeleteItems403Response{}, nil
	}

	if err := s.deleteItems(ctx, apiutil.Deref(request.Params.Ids)); errors.Is(err, filesystem.ErrNotSupported) {
		return api.DeleteItems403Response{}, nil
	} else if err != nil {
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
		paths, err := s.items.SourcePaths(ctx, record.ID)
		if err != nil {
			return err
		}
		for _, path := range paths {
			if err := s.filesystem.RemoveAll(ctx, path); err != nil {
				return err
			}
		}
		if err := s.items.DeleteItem(ctx, record.ID); err != nil {
			return err
		}
	}

	return nil
}
