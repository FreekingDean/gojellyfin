package syncplay

import (
	"context"
	"errors"
	"fmt"
	"time"

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

type Participant struct {
	SessionID uuid.UUID
	UserName  string
}

type Departure struct {
	GroupID   uuid.UUID
	Remaining []Participant
}

type Joined struct {
	Group    *Group
	Left     *Departure
	Rejoined bool
}

func Participants(group *Group) []Participant {
	participants := make([]Participant, 0, len(group.Edges.Members))
	for _, member := range group.Edges.Members {
		session := member.Edges.Session
		if session == nil || session.Edges.User == nil {
			continue
		}
		participants = append(participants, Participant{SessionID: session.ID, UserName: session.Edges.User.Name})
	}

	return participants
}

type Service struct {
	store *store.Client
}

func New(client *store.Client) *Service {
	return &Service{store: client}
}

func (s *Service) Create(ctx context.Context, name string, sessionID uuid.UUID) (Joined, error) {
	var groupID, previousID uuid.UUID

	err := s.store.WithTx(ctx, func(tx *store.Tx) error {
		member, err := memberOf(ctx, tx, sessionID)
		if err != nil {
			return err
		}
		if member != nil {
			previousID = member.GroupID
		}

		group, err := tx.SyncPlayGroup.Create().SetName(name).Save(ctx)
		if err != nil {
			return fmt.Errorf("failed to create syncplay group: %w", err)
		}

		groupID = group.ID

		return join(ctx, tx, group.ID, previousID, sessionID)
	})
	if err != nil {
		return Joined{}, err
	}

	return s.joined(ctx, groupID, previousID, false)
}

func (s *Service) Join(ctx context.Context, groupID, sessionID uuid.UUID) (Joined, error) {
	var previousID uuid.UUID
	var rejoined bool

	err := s.store.WithTx(ctx, func(tx *store.Tx) error {
		exists, err := tx.SyncPlayGroup.Query().
			Where(groupmodal.ID(groupID), groupmodal.HasMembers()).
			Exist(ctx)
		if err != nil {
			return fmt.Errorf("failed to query syncplay group %s: %w", groupID, err)
		}
		if !exists {
			return ErrNoGroup
		}

		member, err := memberOf(ctx, tx, sessionID)
		if err != nil {
			return err
		}
		if member != nil {
			if member.GroupID == groupID {
				rejoined = true
			} else {
				previousID = member.GroupID
			}
		}

		return join(ctx, tx, groupID, previousID, sessionID)
	})
	if err != nil {
		return Joined{}, err
	}

	return s.joined(ctx, groupID, previousID, rejoined)
}

func (s *Service) Leave(ctx context.Context, sessionID uuid.UUID) (*Departure, error) {
	var groupID uuid.UUID

	err := s.store.WithTx(ctx, func(tx *store.Tx) error {
		member, err := memberOf(ctx, tx, sessionID)
		if err != nil || member == nil {
			return err
		}

		groupID = member.GroupID

		if err := tx.SyncPlayGroupMember.DeleteOne(member).Exec(ctx); err != nil {
			return fmt.Errorf("failed to leave syncplay group: %w", err)
		}

		return touch(ctx, tx, groupID)
	})
	if err != nil || groupID == uuid.Nil {
		return nil, err
	}

	return s.departure(ctx, groupID)
}

func (s *Service) List(ctx context.Context) ([]*Group, error) {
	groups, err := withParticipants(s.store.SyncPlayGroup.Query()).
		Where(groupmodal.HasMembers()).
		Order(groupmodal.ByCreatedAt(), groupmodal.ByID()).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list syncplay groups: %w", err)
	}

	return groups, nil
}

func (s *Service) GroupByID(ctx context.Context, groupID uuid.UUID) (*Group, error) {
	group, err := withParticipants(s.store.SyncPlayGroup.Query()).
		Where(groupmodal.ID(groupID), groupmodal.HasMembers()).
		Only(ctx)
	if store.IsNotFound(err) {
		return nil, ErrNoGroup
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query syncplay group %s: %w", groupID, err)
	}

	return group, nil
}

func (s *Service) GroupBySessionID(ctx context.Context, sessionID uuid.UUID) (*Group, error) {
	member, err := s.store.SyncPlayGroupMember.Query().
		Where(membermodal.SessionID(sessionID)).
		Only(ctx)
	if store.IsNotFound(err) {
		return nil, ErrNoGroup
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query syncplay membership: %w", err)
	}

	return s.GroupByID(ctx, member.GroupID)
}

func (s *Service) joined(ctx context.Context, groupID, previousID uuid.UUID, rejoined bool) (Joined, error) {
	group, err := s.GroupByID(ctx, groupID)
	if err != nil {
		return Joined{}, err
	}

	result := Joined{Group: group, Rejoined: rejoined}
	if previousID == uuid.Nil {
		return result, nil
	}

	left, err := s.departure(ctx, previousID)
	if err != nil {
		return Joined{}, err
	}

	result.Left = left

	return result, nil
}

func (s *Service) departure(ctx context.Context, groupID uuid.UUID) (*Departure, error) {
	group, err := s.GroupByID(ctx, groupID)
	if errors.Is(err, ErrNoGroup) {
		return &Departure{GroupID: groupID}, nil
	}
	if err != nil {
		return nil, err
	}

	return &Departure{GroupID: groupID, Remaining: Participants(group)}, nil
}

func join(ctx context.Context, tx *store.Tx, groupID, previousID, sessionID uuid.UUID) error {
	if err := tx.SyncPlayGroupMember.Create().
		SetGroupID(groupID).
		SetSessionID(sessionID).
		OnConflictColumns(membermodal.FieldSessionID).
		UpdateGroupID().
		UpdateUpdatedAt().
		Exec(ctx); err != nil {
		return fmt.Errorf("failed to join syncplay group: %w", err)
	}

	if previousID != uuid.Nil {
		if err := touch(ctx, tx, previousID); err != nil {
			return err
		}
	}

	return touch(ctx, tx, groupID)
}

func touch(ctx context.Context, tx *store.Tx, groupID uuid.UUID) error {
	if err := tx.SyncPlayGroup.UpdateOneID(groupID).SetUpdatedAt(time.Now()).Exec(ctx); err != nil {
		return fmt.Errorf("failed to touch syncplay group %s: %w", groupID, err)
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

func withParticipants(query *store.SyncPlayGroupQuery) *store.SyncPlayGroupQuery {
	return query.WithMembers(func(members *store.SyncPlayGroupMemberQuery) {
		members.WithSession(func(sessions *store.SessionQuery) {
			sessions.WithUser()
		})
	})
}
