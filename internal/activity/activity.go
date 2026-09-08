package activity

import (
	"context"
	"fmt"
	"log"
	"time"

	"entgo.io/ent/dialect/sql"

	"github.com/FreekingDean/gojellyfin/internal/store"
	entrymodel "github.com/FreekingDean/gojellyfin/internal/store/activitylogentry"
)

type (
	Entry    = store.ActivityLogEntry
	Severity = entrymodel.Severity
	Edges    = store.ActivityLogEntryEdges
)

const SeverityInformation = entrymodel.SeverityInformation

const (
	KindAuthenticationSucceeded = "AuthenticationSucceeded"
	KindSessionEnded            = "SessionEnded"
	KindLibraryScanCompleted    = "LibraryScanCompleted"
)

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

func (s *Service) Record(ctx context.Context, entry Entry) {
	create := s.store.ActivityLogEntry.Create().
		SetName(entry.Name).
		SetKind(entry.Kind).
		SetOverview(entry.Overview).
		SetShortOverview(entry.ShortOverview).
		SetSeverity(entry.Severity)

	if entry.Edges.User != nil {
		create.SetUserID(entry.Edges.User.ID)
	}

	if entry.Edges.Item != nil {
		create.SetItemID(entry.Edges.Item.ID)
	}

	err := create.Exec(ctx)
	if err != nil {
		log.Printf("record activity %q: %v", entry.Kind, err)
	}
}

func (s *Service) Entries(ctx context.Context, query Query) ([]*Entry, int, error) {
	entries := s.store.ActivityLogEntry.Query()
	if query.MinDate != nil {
		entries = entries.Where(entrymodel.CreatedAtGTE(*query.MinDate))
	}
	if query.MaxDate != nil {
		entries = entries.Where(entrymodel.CreatedAtLTE(*query.MaxDate))
	}
	if query.HasUserID != nil {
		if *query.HasUserID {
			entries = entries.Where(entrymodel.HasUser())
		} else {
			entries = entries.Where(entrymodel.Not(entrymodal.HasUser()))
		}
	}

	total, err := entries.Clone().Count(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count activity entries: %w", err)
	}

	entries = entries.Order(entrymodel.ByCreatedAt(sql.OrderDesc()), entrymodal.ByID())
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
