package activity

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"entgo.io/ent/dialect/sql"
	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/store"
	entrymodel "github.com/FreekingDean/gojellyfin/internal/store/activitylogentry"
	itemmodel "github.com/FreekingDean/gojellyfin/internal/store/item"
	usermodel "github.com/FreekingDean/gojellyfin/internal/store/user"
)

type (
	Entry    = store.ActivityLogEntryModel
	Severity = entrymodel.Severity
)

const SeverityInformation = entrymodel.SeverityInformation

const (
	KindAuthenticationSucceeded = "AuthenticationSucceeded"
	KindSessionEnded            = "SessionEnded"
	KindLibraryScanCompleted    = "LibraryScanCompleted"
)

type Query struct {
	StartIndex    int
	Limit         int
	MinDate       *time.Time
	MaxDate       *time.Time
	HasUserID     *bool
	Name          string
	Overview      string
	ShortOverview string
	Kind          string
	Username      string
	ItemID        *uuid.UUID
	Severity      *Severity
	Sort          []Sort
}

type SortField string

const (
	SortByCreated       SortField = "created"
	SortBySeverity      SortField = "severity"
	SortByName          SortField = "name"
	SortByOverview      SortField = "overview"
	SortByShortOverview SortField = "short_overview"
	SortByKind          SortField = "kind"
	SortByUsername      SortField = "username"
)

type Sort struct {
	Field      SortField
	Descending bool
}

var severities = []string{"Trace", "Debug", "Information", "Warning", "Error", "Critical", "None"}

type Service struct {
	store *store.Client
}

func New(client *store.Client) *Service {
	return &Service{store: client}
}

func (s *Service) Record(ctx context.Context, entry Entry) {
	err := s.store.ActivityLogEntry.Create().
		SetName(entry.Name).
		SetKind(entry.Kind).
		SetOverview(entry.Overview).
		SetShortOverview(entry.ShortOverview).
		SetSeverity(entry.Severity).
		SetNillableUserID(entry.UserID).
		SetNillableItemID(entry.ItemID).
		Exec(ctx)

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
			entries = entries.Where(entrymodel.Not(entrymodel.HasUser()))
		}
	}
	if query.Name != "" {
		entries = entries.Where(entrymodel.NameContainsFold(query.Name))
	}
	if query.Overview != "" {
		entries = entries.Where(entrymodel.OverviewContainsFold(query.Overview))
	}
	if query.ShortOverview != "" {
		entries = entries.Where(entrymodel.ShortOverviewContainsFold(query.ShortOverview))
	}
	if query.Kind != "" {
		entries = entries.Where(entrymodel.KindContainsFold(query.Kind))
	}
	if query.Username != "" {
		entries = entries.Where(entrymodel.HasUserWith(usermodel.UsernameContainsFold(query.Username)))
	}
	if query.ItemID != nil {
		entries = entries.Where(entrymodel.HasItemWith(itemmodel.ID(*query.ItemID)))
	}
	if query.Severity != nil {
		entries = entries.Where(entrymodel.SeverityEQ(*query.Severity))
	}

	total, err := entries.Clone().Count(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count activity entries: %w", err)
	}

	entries = entries.Order(ordering(query.Sort)...)
	if query.StartIndex > 0 {
		entries = entries.Offset(query.StartIndex)
	}
	if query.Limit > 0 {
		entries = entries.Limit(query.Limit)
	}

	records, err := entries.WithUser().WithItem().AllModels(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query activity entries: %w", err)
	}

	return records, total, nil
}

func ordering(sorts []Sort) []entrymodel.OrderOption {
	if len(sorts) == 0 {
		sorts = []Sort{{Field: SortByCreated, Descending: true}}
	}

	order := make([]entrymodel.OrderOption, 0, len(sorts)+1)
	for _, sort := range sorts {
		direction := sql.OrderAsc()
		if sort.Descending {
			direction = sql.OrderDesc()
		}

		switch sort.Field {
		case SortByCreated:
			order = append(order, entrymodel.ByCreatedAt(direction))
		case SortBySeverity:
			order = append(order, bySeverity(sort.Descending))
		case SortByName:
			order = append(order, entrymodel.ByName(direction))
		case SortByOverview:
			order = append(order, entrymodel.ByOverview(direction))
		case SortByShortOverview:
			order = append(order, entrymodel.ByShortOverview(direction))
		case SortByKind:
			order = append(order, entrymodel.ByKind(direction))
		case SortByUsername:
			order = append(order, entrymodel.ByUserField(usermodel.FieldUsername, direction))
		}
	}

	return append(order, entrymodel.ByID())
}

func bySeverity(descending bool) entrymodel.OrderOption {
	levels := "'" + strings.Join(severities, "','") + "'"

	return func(selector *sql.Selector) {
		selector.OrderExprFunc(func(builder *sql.Builder) {
			builder.WriteString("array_position(ARRAY[" + levels + "]::text[], " + selector.C(entrymodel.FieldSeverity) + "::text)")
			if descending {
				builder.WriteString(" DESC")
			}
		})
	}
}
