package syncplay

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/store"
	groupmodal "github.com/FreekingDean/gojellyfin/internal/store/syncplaygroup"
	membermodal "github.com/FreekingDean/gojellyfin/internal/store/syncplaygroupmember"
)

type (
	Group  = store.SyncPlayGroup
	Member = store.SyncPlayGroupMember
	State  = groupmodal.State
)

var ErrNoGroup = errors.New("no such syncplay group")

// Go cannot attach a method to the aliases the domain hands out, so the edge
// walk to the member names lives here rather than in the tag package.
func Participants(group *Group) []string {
	names := make([]string, 0, len(group.Edges.Members))
	for _, member := range group.Edges.Members {
		session := member.Edges.Session
		if session == nil || session.Edges.User == nil {
			continue
		}
		names = append(names, session.Edges.User.Name)
	}

	return names
}

type Service struct {
	store *store.Client
}

func New(client *store.Client) *Service {
	return &Service{store: client}
}

func (s *Service) Create(ctx context.Context, name string, sessionID uuid.UUID) (*Group, error) {
	var groupID uuid.UUID

	err := s.store.WithTx(ctx, func(tx *store.Tx) error {
		group, err := tx.SyncPlayGroup.Create().SetName(name).Save(ctx)
		if err != nil {
			return fmt.Errorf("failed to create syncplay group: %w", err)
		}

		groupID = group.ID

		return join(ctx, tx, group.ID, sessionID)
	})
	if err != nil {
		return nil, err
	}

	return s.GroupByID(ctx, groupID)
}

func (s *Service) Join(ctx context.Context, groupID, sessionID uuid.UUID) error {
	return s.store.WithTx(ctx, func(tx *store.Tx) error {
		exists, err := tx.SyncPlayGroup.Query().Where(groupmodal.ID(groupID)).Exist(ctx)
		if err != nil {
			return fmt.Errorf("failed to query syncplay group %s: %w", groupID, err)
		}
		if !exists {
			return ErrNoGroup
		}

		return join(ctx, tx, groupID, sessionID)
	})
}

func (s *Service) Leave(ctx context.Context, sessionID uuid.UUID) error {
	return s.store.WithTx(ctx, func(tx *store.Tx) error {
		member, err := memberOf(ctx, tx, sessionID)
		if err != nil || member == nil {
			return err
		}

		return leave(ctx, tx, member)
	})
}

func (s *Service) List(ctx context.Context) ([]*Group, error) {
	groups, err := withParticipants(s.store.SyncPlayGroup.Query()).
		Order(groupmodal.ByCreatedAt(), groupmodal.ByID()).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list syncplay groups: %w", err)
	}

	return groups, nil
}

func (s *Service) GroupByID(ctx context.Context, groupID uuid.UUID) (*Group, error) {
	group, err := withParticipants(s.store.SyncPlayGroup.Query()).
		Where(groupmodal.ID(groupID)).
		Only(ctx)
	if store.IsNotFound(err) {
		return nil, ErrNoGroup
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query syncplay group %s: %w", groupID, err)
	}

	return group, nil
}

func join(ctx context.Context, tx *store.Tx, groupID, sessionID uuid.UUID) error {
	member, err := memberOf(ctx, tx, sessionID)
	if err != nil {
		return err
	}
	if member != nil {
		if member.GroupID == groupID {
			return nil
		}
		if err := leave(ctx, tx, member); err != nil {
			return err
		}
	}

	if err := tx.SyncPlayGroupMember.Create().
		SetGroupID(groupID).
		SetSessionID(sessionID).
		Exec(ctx); err != nil {
		return fmt.Errorf("failed to join syncplay group: %w", err)
	}

	return nil
}

func memberOf(ctx context.Context, tx *store.Tx, sessionID uuid.UUID) (*Member, error) {
	member, err := tx.SyncPlayGroupMember.Query().
		Where(membermodal.SessionID(sessionID)).
		Only(ctx)
	if store.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query syncplay membership: %w", err)
	}

	return member, nil
}

func leave(ctx context.Context, tx *store.Tx, member *Member) error {
	if err := tx.SyncPlayGroupMember.DeleteOne(member).Exec(ctx); err != nil {
		return fmt.Errorf("failed to leave syncplay group: %w", err)
	}

	return disbandIfEmpty(ctx, tx, member.GroupID)
}

func disbandIfEmpty(ctx context.Context, tx *store.Tx, groupID uuid.UUID) error {
	remaining, err := tx.SyncPlayGroupMember.Query().
		Where(membermodal.GroupID(groupID)).
		Count(ctx)
	if err != nil {
		return fmt.Errorf("failed to count syncplay members: %w", err)
	}
	if remaining > 0 {
		return nil
	}

	if err := tx.SyncPlayGroup.DeleteOneID(groupID).Exec(ctx); err != nil {
		return fmt.Errorf("failed to disband syncplay group: %w", err)
	}

	return nil
}

func withParticipants(query *store.SyncPlayGroupQuery) *store.SyncPlayGroupQuery {
	return query.WithMembers(func(members *store.SyncPlayGroupMemberQuery) {
		members.WithSession(func(sessions *store.SessionQuery) {
			sessions.WithUser()
		})
	})
}
