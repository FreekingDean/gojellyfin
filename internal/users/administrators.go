package users

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/store"
	usermodel "github.com/FreekingDean/gojellyfin/internal/store/user"
	policymodel "github.com/FreekingDean/gojellyfin/internal/store/userpolicy"
)

var ErrLastAdministrator = errors.New("there must be at least one user with administrative access")

func (s *Service) SavePolicy(ctx context.Context, id uuid.UUID, edit func(*Policy)) error {
	return s.keepingAnAdministrator(ctx, func(tx *store.Tx) error {
		policy, err := tx.UserPolicy.Query().
			Where(policymodel.HasUserWith(usermodel.ID(id))).
			ForUpdate().
			OnlyModel(ctx)
		if err != nil {
			return fmt.Errorf("failed to read user policy: %w", err)
		}

		edit(policy)
		if err := tx.UserPolicy.UpdateOneID(policy.ID).SetModel(policy).Exec(ctx); err != nil {
			return fmt.Errorf("failed to update user policy: %w", err)
		}

		return nil
	})
}

func (s *Service) DeleteUser(ctx context.Context, id uuid.UUID) error {
	return s.keepingAnAdministrator(ctx, func(tx *store.Tx) error {
		if err := tx.User.DeleteOneID(id).Exec(ctx); err != nil {
			return fmt.Errorf("failed to delete user: %w", err)
		}

		return nil
	})
}

func (s *Service) keepingAnAdministrator(ctx context.Context, write func(*store.Tx) error) error {
	return s.store.WithTx(ctx, func(tx *store.Tx) error {
		before, err := tx.UserPolicy.Query().
			Where(policymodel.IsAdministrator(true)).
			ForUpdate().
			IDs(ctx)
		if err != nil {
			return fmt.Errorf("failed to lock administrators: %w", err)
		}

		if err := write(tx); err != nil {
			return err
		}

		after, err := tx.UserPolicy.Query().Where(policymodel.IsAdministrator(true)).Count(ctx)
		if err != nil {
			return fmt.Errorf("failed to count administrators: %w", err)
		}
		if len(before) > 0 && after == 0 {
			return ErrLastAdministrator
		}

		return nil
	})
}
