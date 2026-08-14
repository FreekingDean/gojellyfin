package users

import (
	"context"

	"github.com/google/uuid"

	policymodal "github.com/FreekingDean/gojellyfin/internal/store/userpolicy"
)

// Satisfies reports whether the user may perform an operation the spec guards
// with these scopes. Every scope has to hold; a user with no policy row, or a
// disabled one, satisfies nothing.
func (s *Service) Satisfies(ctx context.Context, id uuid.UUID, scopes []string) (bool, error) {
	policy, err := s.policy(ctx, id)
	if err != nil || policy == nil {
		return false, err
	}
	if policy.IsDisabled {
		return false, nil
	}
	if policy.IsAdministrator {
		return true, nil
	}

	for _, scope := range scopes {
		if !granted(policy, scope) {
			return false, nil
		}
	}

	return true, nil
}

// Only an administrator reaches this with an unlisted scope, and they have
// already been answered, so anything not named here is refused. That is what
// makes a scope added by a later spec fail closed rather than open.
func granted(policy *Policy, scope string) bool {
	switch scope {
	case "DefaultAuthorization", "FirstTimeSetupOrDefault", "FirstTimeSetupOrIgnoreParentalControl":
		return true
	case "LiveTvAccess":
		return policy.EnableLiveTvAccess
	case "LiveTvManagement":
		return policy.EnableLiveTvManagement
	case "CollectionManagement":
		return policy.EnableCollectionManagement
	case "SubtitleManagement":
		return policy.EnableSubtitleManagement
	case "LyricManagement":
		return policy.EnableLyricManagement
	case "Download":
		return policy.EnableContentDownloading
	case "SyncPlayCreateGroup":
		return policy.SyncPlayAccess == policymodal.SyncPlayAccessCreateAndJoinGroups
	case "SyncPlayHasAccess", "SyncPlayIsInGroup", "SyncPlayJoinGroup":
		return policy.SyncPlayAccess != policymodal.SyncPlayAccessNone
	}

	return false
}
