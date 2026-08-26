package users

import (
	"context"

	"github.com/google/uuid"

	policymodal "github.com/FreekingDean/gojellyfin/internal/store/userpolicy"
)

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
