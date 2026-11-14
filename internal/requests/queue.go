package requests

import (
	"context"
	"fmt"
	"time"

	"entgo.io/ent/dialect/sql"
	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/store"
	requestmodel "github.com/FreekingDean/gojellyfin/internal/store/request"
	submissionmodel "github.com/FreekingDean/gojellyfin/internal/store/requestsubmission"
	usermodel "github.com/FreekingDean/gojellyfin/internal/store/user"
)

type (
	Status = requestmodel.Status
	Kind   = requestmodel.Kind
)

const (
	StatusPending  = requestmodel.StatusPending
	StatusApproved = requestmodel.StatusApproved
	StatusDeclined = requestmodel.StatusDeclined
)

type Asked struct {
	Key         string
	Name        string
	Kind        Kind
	TmdbID      int
	Requests    []*Request
	Submissions []*Submission
}

func (s *Service) Queue(ctx context.Context) ([]Asked, error) {
	records, err := s.store.Request.Query().
		WithUser().
		Order(requestmodel.ByCreatedAt(sql.OrderDesc())).
		AllModels(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query the requests: %w", err)
	}

	queue := []Asked{}
	position := map[string]int{}
	keys := []string{}
	for _, record := range records {
		at, seen := position[record.Key]
		if !seen {
			at = len(queue)
			position[record.Key] = at
			keys = append(keys, record.Key)
			queue = append(queue, Asked{Key: record.Key, Name: record.Name, Kind: record.Kind, TmdbID: record.TmdbID})
		}
		queue[at].Requests = append(queue[at].Requests, record)
	}
	if len(keys) == 0 {
		return queue, nil
	}

	submissions, err := s.store.RequestSubmission.Query().
		Where(submissionmodel.KeyIn(keys...)).
		WithSource().
		Order(submissionmodel.ByCreatedAt()).
		AllModels(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query what the sources were asked for: %w", err)
	}
	for _, submission := range submissions {
		at := position[submission.Key]
		queue[at].Submissions = append(queue[at].Submissions, submission)
	}

	return queue, nil
}

type Allowance struct {
	CanRequest    bool
	NeedsApproval bool
	Quota         *int
}

type Asker struct {
	ID   uuid.UUID
	Name string
	Used int
	Allowance
}

func (s *Service) Askers(ctx context.Context) ([]Asker, error) {
	users, err := s.store.User.Query().Order(usermodel.ByName()).AllModels(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query the users: %w", err)
	}

	var counted []struct {
		UserID uuid.UUID `json:"user_id"`
		Count  int       `json:"count"`
	}
	err = s.store.Request.Query().
		Where(requestmodel.CreatedAtGT(time.Now().Add(-QuotaWindow))).
		GroupBy(requestmodel.FieldUserID).
		Aggregate(store.Count()).
		Scan(ctx, &counted)
	if err != nil {
		return nil, fmt.Errorf("failed to count what was asked for lately: %w", err)
	}
	used := map[uuid.UUID]int{}
	for _, count := range counted {
		used[count.UserID] = count.Count
	}

	askers := make([]Asker, len(users))
	for i, user := range users {
		askers[i] = Asker{
			ID:   user.ID,
			Name: user.Name,
			Used: used[user.ID],
			Allowance: Allowance{
				CanRequest:    user.CanRequest,
				NeedsApproval: user.RequestsNeedApproval,
				Quota:         user.RequestQuota,
			},
		}
	}

	return askers, nil
}

func (s *Service) Allow(ctx context.Context, userID uuid.UUID, allowance Allowance) error {
	update := s.store.User.UpdateOneID(userID).
		SetCanRequest(allowance.CanRequest).
		SetRequestsNeedApproval(allowance.NeedsApproval).
		ClearRequestQuota()
	if allowance.Quota != nil {
		update.SetRequestQuota(*allowance.Quota)
	}

	err := update.Exec(ctx)
	if store.IsNotFound(err) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("failed to save what the user may ask for: %w", err)
	}

	return nil
}
