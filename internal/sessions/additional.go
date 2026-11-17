package sessions

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/store"
	itemmodel "github.com/FreekingDean/gojellyfin/internal/store/item"
	sessionmodel "github.com/FreekingDean/gojellyfin/internal/store/session"
	usermodel "github.com/FreekingDean/gojellyfin/internal/store/user"
)

var (
	ErrNoSession   = errors.New("no such session")
	ErrNoUser      = errors.New("no such user")
	ErrPrimaryUser = errors.New("the user already owns the session")
	ErrNoSuchItem  = errors.New("no such item")
	liveSession    = sessionmodel.RevokedAtIsNil()
)

const ViewingTimeout = PlayingTimeout

func (s *Service) ByID(ctx context.Context, id uuid.UUID) (*Session, error) {
	session, err := s.store.Session.Query().
		Where(sessionmodel.ID(id), liveSession).
		WithUser().
		OnlyModel(ctx)
	if store.IsNotFound(err) {
		return nil, fmt.Errorf("%w: %s", ErrNoSession, id)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query the session: %w", err)
	}

	return session, nil
}

func (s *Service) AddUser(ctx context.Context, sessionID, userID uuid.UUID) error {
	session, err := s.ByID(ctx, sessionID)
	if err != nil {
		return err
	}
	if session.User != nil && session.User.ID == userID {
		return ErrPrimaryUser
	}

	joined, err := s.store.Session.Query().
		Where(sessionmodel.ID(sessionID), sessionmodel.HasAdditionalUsersWith(usermodel.ID(userID))).
		Exist(ctx)
	if err != nil {
		return fmt.Errorf("failed to read the session's users: %w", err)
	}
	if joined {
		return nil
	}

	err = s.store.Session.UpdateOneID(sessionID).AddAdditionalUserIDs(userID).Exec(ctx)
	if store.IsConstraintError(err) {
		return fmt.Errorf("%w: %s", ErrNoUser, userID)
	}
	if err != nil {
		return fmt.Errorf("failed to add the user to the session: %w", err)
	}

	return nil
}

func (s *Service) RemoveUser(ctx context.Context, sessionID, userID uuid.UUID) error {
	if _, err := s.ByID(ctx, sessionID); err != nil {
		return err
	}

	if err := s.store.Session.UpdateOneID(sessionID).RemoveAdditionalUserIDs(userID).Exec(ctx); err != nil {
		return fmt.Errorf("failed to remove the user from the session: %w", err)
	}

	return nil
}

func (s *Service) Viewing(ctx context.Context, sessionID, itemID uuid.UUID) error {
	exists, err := s.store.Item.Query().Where(itemmodel.ID(itemID), itemmodel.DeletedAtIsNil()).Exist(ctx)
	if err != nil {
		return fmt.Errorf("failed to find the item: %w", err)
	}
	if !exists {
		return fmt.Errorf("%w: %s", ErrNoSuchItem, itemID)
	}

	now := time.Now()
	err = s.store.Session.UpdateOneID(sessionID).
		SetNowViewingItemID(itemID).
		SetViewingAt(now).
		SetLastActivityAt(now).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to record what the session is viewing: %w", err)
	}

	return nil
}

func NowViewing(session *Session, now time.Time) bool {
	return session.NowViewing != nil && session.ViewingAt != nil && now.Sub(*session.ViewingAt) < ViewingTimeout
}
