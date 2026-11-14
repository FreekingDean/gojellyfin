package syncplay

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/store"
	groupmodal "github.com/FreekingDean/gojellyfin/internal/store/syncplaygroup"
	membermodal "github.com/FreekingDean/gojellyfin/internal/store/syncplaygroupmember"
)

type (
	Group  = store.SyncPlayGroupModel
	Member = store.SyncPlayGroupMemberModel
)

var (
	ErrNoGroup      = errors.New("no such syncplay group")
	ErrNotInGroup   = errors.New("session is in no syncplay group")
	ErrAccessDenied = errors.New("session cannot see the syncplay group's queue")
)

type Publisher interface {
	Publish(ctx context.Context, sessionIDs []uuid.UUID, messageType string, data any) error
}

type Participant struct {
	SessionID uuid.UUID
	UserName  string
}

type Departure struct {
	GroupID   uuid.UUID
	Messages  []Outgoing
}

type Joined struct {
	Group    *Group
	Left     *Departure
	Rejoined bool
	Messages []Outgoing
}

func Participants(group *Group) []Participant {
	participants := make([]Participant, 0, len(group.Members))
	for _, member := range group.Members {
		session := member.Session
		if session == nil || session.User == nil {
			continue
		}
		participants = append(participants, Participant{SessionID: session.ID, UserName: session.User.Name})
	}

	return participants
}

type Service struct {
	store     *store.Client
	library   Library
	policies  Policies
	publisher Publisher
	now       func() time.Time
}

func New(client *store.Client, library Library, policies Policies, publisher Publisher) *Service {
	return &Service{store: client, library: library, policies: policies, publisher: publisher, now: time.Now}
}

func (s *Service) Create(ctx context.Context, name string, sessionID uuid.UUID) (Joined, error) {
	var groupID uuid.UUID
	var left *Departure
	var messages []Outgoing

	err := s.store.WithTx(ctx, func(tx *store.Tx) error {
		var err error
		left, err = s.vacate(ctx, tx, sessionID, uuid.Nil)
		if err != nil {
			return err
		}

		group, err := tx.SyncPlayGroup.Create().SetName(name).Save(ctx)
		if err != nil {
			return fmt.Errorf("failed to create syncplay group: %w", err)
		}
		groupID = group.ID

		messages, err = s.seat(ctx, tx, groupID, sessionID)

		return err
	})
	if err != nil {
		return Joined{}, err
	}

	return s.joined(ctx, groupID, left, false, messages)
}

func (s *Service) Join(ctx context.Context, groupID, sessionID, userID uuid.UUID) (Joined, error) {
	var left *Departure
	var messages []Outgoing
	var rejoined bool

	err := s.store.WithTx(ctx, func(tx *store.Tx) error {
		room, err := lockRoom(ctx, tx, groupID)
		if err != nil {
			return err
		}
		if len(room.Seats) == 0 {
			return ErrNoGroup
		}

		group, err := tx.SyncPlayGroup.Query().Where(groupmodal.ID(groupID)).WithQueue().OnlyModel(ctx)
		if err != nil {
			return fmt.Errorf("failed to read the queue of syncplay group %s: %w", groupID, err)
		}
		allowed, err := s.CanSeeQueue(ctx, userID, group)
		if err != nil {
			return err
		}
		if !allowed {
			return ErrAccessDenied
		}

		rejoined = room.seat(sessionID) != nil

		left, err = s.vacate(ctx, tx, sessionID, groupID)
		if err != nil {
			return err
		}

		messages, err = s.seat(ctx, tx, groupID, sessionID)

		return err
	})
	if err != nil {
		return Joined{}, err
	}

	return s.joined(ctx, groupID, left, rejoined, messages)
}

func (s *Service) Leave(ctx context.Context, sessionID uuid.UUID) (*Departure, error) {
	var left *Departure

	err := s.store.WithTx(ctx, func(tx *store.Tx) error {
		member, err := memberOf(ctx, tx, sessionID)
		if err != nil {
			return err
		}
		if member == nil {
			return ErrNotInGroup
		}

		left, err = s.vacate(ctx, tx, sessionID, uuid.Nil)
		if err != nil {
			return err
		}

		if err := tx.SyncPlayGroupMember.DeleteOneID(member.ID).Exec(ctx); err != nil {
			return fmt.Errorf("failed to leave syncplay group: %w", err)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return left, nil
}

func (s *Service) SessionsEnding(ctx context.Context, sessionIDs []uuid.UUID) {
	for _, sessionID := range sessionIDs {
		left, err := s.Leave(ctx, sessionID)
		if errors.Is(err, ErrNotInGroup) {
			continue
		}
		if err != nil {
			log.Printf("syncplay: failed to leave the group of ending session %s: %v", sessionID, err)

			continue
		}

		s.Publish(ctx, left.Messages)
	}
}

func (s *Service) Apply(ctx context.Context, sessionID uuid.UUID, request Request) ([]Outgoing, error) {
	var messages []Outgoing

	err := s.store.WithTx(ctx, func(tx *store.Tx) error {
		member, err := memberOf(ctx, tx, sessionID)
		if err != nil {
			return err
		}
		if member == nil {
			return ErrNotInGroup
		}

		room, err := lockRoom(ctx, tx, member.GroupID)
		if err != nil {
			return err
		}

		seat := room.seat(sessionID)
		if seat == nil {
			return ErrNotInGroup
		}

		room.catalogue, err = s.known(ctx, room, request.ItemIDs)
		if err != nil {
			return err
		}

		room.now = s.now()
		room.handle(request, room.State, seat)
		messages = room.out

		return saveRoom(ctx, tx, room)
	})

	return messages, err
}

func (s *Service) List(ctx context.Context, userID uuid.UUID) ([]*Group, error) {
	groups, err := withRoom(s.store.SyncPlayGroup.Query()).
		Where(groupmodal.HasMembers()).
		Order(groupmodal.ByCreatedAt(), groupmodal.ByID()).
		AllModels(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list syncplay groups: %w", err)
	}

	visible := make([]*Group, 0, len(groups))
	for _, group := range groups {
		allowed, err := s.CanSeeQueue(ctx, userID, group)
		if err != nil {
			return nil, err
		}
		if allowed {
			visible = append(visible, group)
		}
	}

	return visible, nil
}

func (s *Service) GroupFor(ctx context.Context, userID, groupID uuid.UUID) (*Group, error) {
	group, err := s.GroupByID(ctx, groupID)
	if err != nil {
		return nil, err
	}

	allowed, err := s.CanSeeQueue(ctx, userID, group)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, ErrNoGroup
	}

	return group, nil
}

func (s *Service) GroupByID(ctx context.Context, groupID uuid.UUID) (*Group, error) {
	group, err := withRoom(s.store.SyncPlayGroup.Query()).
		Where(groupmodal.ID(groupID), groupmodal.HasMembers()).
		OnlyModel(ctx)
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

func (s *Service) GroupsBySessionIDs(ctx context.Context, sessionIDs []uuid.UUID) ([]*Group, error) {
	if len(sessionIDs) == 0 {
		return nil, nil
	}

	groups, err := withRoom(s.store.SyncPlayGroup.Query()).
		Where(groupmodal.HasMembersWith(membermodal.SessionIDIn(sessionIDs...))).
		AllModels(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query the syncplay groups of %d sessions: %w", len(sessionIDs), err)
	}

	return groups, nil
}

func (s *Service) joined(ctx context.Context, groupID uuid.UUID, left *Departure, rejoined bool, messages []Outgoing) (Joined, error) {
	group, err := s.GroupByID(ctx, groupID)
	if err != nil {
		return Joined{}, err
	}

	return Joined{Group: group, Left: left, Rejoined: rejoined, Messages: messages}, nil
}

func (s *Service) vacate(ctx context.Context, tx *store.Tx, sessionID, staying uuid.UUID) (*Departure, error) {
	member, err := memberOf(ctx, tx, sessionID)
	if err != nil || member == nil || member.GroupID == staying {
		return nil, err
	}

	room, err := lockRoom(ctx, tx, member.GroupID)
	if err != nil {
		return nil, err
	}

	room.catalogue = known{}
	room.now = s.now()
	room.leave(sessionID)

	if err := saveRoom(ctx, tx, room); err != nil {
		return nil, err
	}

	return &Departure{GroupID: room.ID, Messages: room.out}, nil
}

func (s *Service) seat(ctx context.Context, tx *store.Tx, groupID, sessionID uuid.UUID) ([]Outgoing, error) {
	member, err := memberOf(ctx, tx, sessionID)
	if err != nil {
		return nil, err
	}

	if member == nil || member.GroupID != groupID {
		if err := tx.SyncPlayGroupMember.Create().
			SetGroupID(groupID).
			SetSessionID(sessionID).
			OnConflictColumns(membermodal.FieldSessionID).
			UpdateGroupID().
			UpdatePing().
			UpdateIsBuffering().
			UpdateIgnoreWait().
			UpdateUpdatedAt().
			Exec(ctx); err != nil {
			return nil, fmt.Errorf("failed to join syncplay group: %w", err)
		}
	}

	room, err := lockRoom(ctx, tx, groupID)
	if err != nil {
		return nil, err
	}

	room.catalogue, err = s.known(ctx, room, nil)
	if err != nil {
		return nil, err
	}

	room.now = s.now()
	room.join(room.seat(sessionID))

	if err := saveRoom(ctx, tx, room); err != nil {
		return nil, err
	}

	return room.out, nil
}

func memberOf(ctx context.Context, tx *store.Tx, sessionID uuid.UUID) (*Member, error) {
	member, err := tx.SyncPlayGroupMember.Query().
		Where(membermodal.SessionID(sessionID)).
		OnlyModel(ctx)
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
		members.Order(membermodal.ByCreatedAt(), membermodal.ByID())
		members.WithSession(func(sessions *store.SessionQuery) {
			sessions.WithUser()
		})
	})
}
