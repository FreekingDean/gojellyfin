package activity

import (
	"context"
	"fmt"
	"log"
	"time"

	"entgo.io/ent/dialect/sql"
	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/store"
	entrymodal "github.com/FreekingDean/gojellyfin/internal/store/activitylogentry"
)

type (
	Entry    = store.ActivityLogEntry
	Severity = entrymodal.Severity
)

const SeverityInformation = entrymodal.SeverityInformation

const (
	KindAuthenticationSucceeded = "AuthenticationSucceeded"
	KindSessionEnded            = "SessionEnded"
	KindLibraryScanCompleted    = "LibraryScanCompleted"
)

type Event struct {
	Name          string
	Kind          string
	Overview      string
	ShortOverview string
	Severity      Severity
	UserID        *uuid.UUID
	ItemID        *uuid.UUID
}

type Query struct {
	StartIndex int
	Limit      int
	MinDate    *time.Time
	MaxDate    *time.Time
	HasUserID  *bool
}

type Service struct {
	store *store.Client
}

func New(client *store.Client) *Service {
	return &Service{store: client}
}

func (s *Service) Record(ctx context.Context, event Event) {
	err := s.store.ActivityLogEntry.Create().
		SetName(event.Name).
		SetKind(event.Kind).
		SetOverview(event.Overview).
		SetShortOverview(event.ShortOverview).
		SetSeverity(event.Severity).
		SetNillableUserID(event.UserID).
		SetNillableItemID(event.ItemID).
		Exec(ctx)
	if err != nil {
		log.Printf("record activity %q: %v", event.Kind, err)
	}
}

func (s *Service) Entries(ctx context.Context, query Query) ([]*Entry, int, error) {
	entries := s.store.ActivityLogEntry.Query()
	if query.MinDate != nil {
		entries = entries.Where(entrymodal.CreatedAtGTE(*query.MinDate))
	}
	if query.MaxDate != nil {
		entries = entries.Where(entrymodal.CreatedAtLTE(*query.MaxDate))
	}
	if query.HasUserID != nil {
		if *query.HasUserID {
			entries = entries.Where(entrymodal.HasUser())
		} else {
			entries = entries.Where(entrymodal.Not(entrymodal.HasUser()))
		}
	}

	total, err := entries.Clone().Count(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count activity entries: %w", err)
	}

	entries = entries.Order(entrymodal.ByCreatedAt(sql.OrderDesc()), entrymodal.ByID())
	if query.StartIndex > 0 {
		entries = entries.Offset(query.StartIndex)
	}
	if query.Limit > 0 {
		entries = entries.Limit(query.Limit)
	}

	records, err := entries.WithUser().WithItem().All(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query activity entries: %w", err)
	}

	return records, total, nil
}
