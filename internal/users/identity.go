package users

import (
	"context"
	"errors"
	"fmt"

	"github.com/FreekingDean/gojellyfin/internal/store"
	usermodel "github.com/FreekingDean/gojellyfin/internal/store/user"
)

var ErrUsernameClaimed = errors.New("a local account already holds this username and no identity may claim it")

type Identity struct {
	Issuer        string
	Subject       string
	Username      string
	Administrator *bool
}

func (s *Service) SignInIdentity(ctx context.Context, identity Identity, linkExisting bool) (*User, error) {
	user, err := s.userByIdentity(ctx, identity)
	if store.IsNotFound(err) {
		user, err = s.adopt(ctx, identity, linkExisting)
	}
	if err != nil {
		return nil, err
	}

	if identity.Administrator == nil || user.Policy == nil || user.Policy.IsAdministrator == *identity.Administrator {
		return user, nil
	}
	if err := s.store.UserPolicy.UpdateOneID(user.Policy.ID).SetIsAdministrator(*identity.Administrator).Exec(ctx); err != nil {
		return nil, fmt.Errorf("failed to sync administrator from identity: %w", err)
	}

	return s.User(ctx, user.ID)
}

func (s *Service) userByIdentity(ctx context.Context, identity Identity) (*User, error) {
	return s.query().
		Where(usermodel.OidcIssuer(identity.Issuer), usermodel.OidcSubject(identity.Subject)).
		OnlyModel(ctx)
}

func (s *Service) adopt(ctx context.Context, identity Identity, linkExisting bool) (*User, error) {
	existing, err := s.UserByUsername(ctx, identity.Username)
	switch {
	case err == nil && (!linkExisting || existing.OidcIssuer != nil):
		return nil, ErrUsernameClaimed
	case err == nil:
		return s.link(ctx, existing, identity)
	case !store.IsNotFound(err):
		return nil, err
	}

	return s.create(ctx, identity.Username, "", false, &identity)
}

func (s *Service) link(ctx context.Context, user *User, identity Identity) (*User, error) {
	linked, err := s.store.User.Update().
		Where(usermodel.ID(user.ID), usermodel.OidcIssuerIsNil(), usermodel.OidcSubjectIsNil()).
		SetOidcIssuer(identity.Issuer).
		SetOidcSubject(identity.Subject).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to link identity: %w", err)
	}
	if linked == 0 {
		return nil, ErrUsernameClaimed
	}

	return s.User(ctx, user.ID)
}
