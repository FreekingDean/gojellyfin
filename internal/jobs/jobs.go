package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"entgo.io/ent/dialect/sql"
	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/store"
	"github.com/FreekingDean/gojellyfin/internal/store/entities"
	jobmodal "github.com/FreekingDean/gojellyfin/internal/store/job"
	schedulemodal "github.com/FreekingDean/gojellyfin/internal/store/jobschedule"
	"github.com/FreekingDean/gojellyfin/internal/store/predicate"
)

type (
	Job     = store.Job
	State   = jobmodal.State
	Trigger = entities.JobTrigger
)

const (
	StateQueued    = jobmodal.StateQueued
	StateRunning   = jobmodal.StateRunning
	StateSucceeded = jobmodal.StateSucceeded
	StateFailed    = jobmodal.StateFailed
	StateCancelled = jobmodal.StateCancelled
)

const LibraryScanKind = "RefreshLibrary"

const (
	retryBackoff    = 30 * time.Second
	maxRetryBackoff = 10 * time.Minute
)

var (
	ErrNotFound  = errors.New("job not found")
	ErrLeaseLost = errors.New("job lease lost")
)

var (
	activeStates = []State{StateQueued, StateRunning}
	endedStates  = []State{StateSucceeded, StateFailed, StateCancelled}
)

var definitions = []Definition{{
	Kind:        LibraryScanKind,
	Name:        "Scan Media Library",
	Description: "Scans the media libraries for new and changed files.",
	Category:    "Library",
}}

type Definition struct {
	Kind        string
	Name        string
	Description string
	Category    string
}

type Request struct {
	Kind        string
	Payload     json.RawMessage
	DedupeKey   string
	RunAt       time.Time
	MaxAttempts int
}

type Summary struct {
	Definition Definition
	Active     *Job
	Last       *Job
	Triggers   []Trigger
}

type Reporter func(progress float64)

type Runner func(ctx context.Context, job *Job, report Reporter) error

type Service struct {
	store *store.Client
}

func New(client *store.Client) *Service {
	return &Service{store: client}
}

func (s *Service) Start(ctx context.Context, kind string) (*Job, error) {
	definition, err := s.definition(kind)
	if err != nil {
		return nil, err
	}

	return s.Enqueue(ctx, Request{Kind: definition.Kind, DedupeKey: definition.Kind})
}

func (s *Service) Enqueue(ctx context.Context, request Request) (*Job, error) {
	if request.DedupeKey != "" {
		existing, err := s.activeByDedupeKey(ctx, request.DedupeKey)
		if err != nil && !errors.Is(err, ErrNotFound) {
			return nil, err
		}
		if existing != nil {
			return existing, nil
		}
	}

	create := s.store.Job.Create().SetKind(request.Kind)
	if len(request.Payload) > 0 {
		create.SetPayload(request.Payload)
	}
	if request.DedupeKey != "" {
		create.SetDedupeKey(request.DedupeKey)
	}
	if !request.RunAt.IsZero() {
		create.SetRunAt(request.RunAt)
	}
	if request.MaxAttempts > 0 {
		create.SetMaxAttempts(request.MaxAttempts)
	}

	job, err := create.Save(ctx)
	if err != nil {
		if store.IsConstraintError(err) {
			return s.activeByDedupeKey(ctx, request.DedupeKey)
		}

		return nil, fmt.Errorf("failed to enqueue job: %w", err)
	}

	return job, nil
}

func (s *Service) Lease(ctx context.Context, worker string, kinds []string, lease time.Duration) (*Job, error) {
	var leased *Job

	err := s.store.WithTx(ctx, func(tx *store.Tx) error {
		now := time.Now()
		predicates := []predicate.Job{
			jobmodal.Or(
				jobmodal.And(jobmodal.StateEQ(StateQueued), jobmodal.RunAtLTE(now)),
				jobmodal.And(jobmodal.StateEQ(StateRunning), jobmodal.LeaseExpiresAtLT(now)),
			),
		}
		if len(kinds) > 0 {
			predicates = append(predicates, jobmodal.KindIn(kinds...))
		}

		found, err := tx.Job.Query().
			Where(predicates...).
			Order(jobmodal.ByRunAt()).
			Limit(1).
			ForUpdate(sql.WithLockAction(sql.SkipLocked)).
			All(ctx)
		if err != nil {
			return fmt.Errorf("failed to lease a job: %w", err)
		}
		if len(found) == 0 {
			return nil
		}

		updated, err := tx.Job.UpdateOne(found[0]).
			SetState(StateRunning).
			SetWorker(worker).
			SetAttempt(found[0].Attempt + 1).
			SetLeaseExpiresAt(now.Add(lease)).
			SetStartedAt(now).
			SetProgress(0).
			Save(ctx)
		if err != nil {
			return fmt.Errorf("failed to claim a job: %w", err)
		}
		leased = updated

		return nil
	})
	if err != nil {
		return nil, err
	}
	if leased == nil {
		return nil, ErrNotFound
	}

	return leased, nil
}

func (s *Service) Renew(ctx context.Context, job *Job, lease time.Duration, progress float64) (bool, error) {
	updated, err := s.store.Job.Update().
		Where(s.held(job)...).
		SetLeaseExpiresAt(time.Now().Add(lease)).
		SetProgress(progress).
		Save(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to renew the job lease: %w", err)
	}
	if updated == 0 {
		return false, ErrLeaseLost
	}

	current, err := s.JobByID(ctx, job.ID)
	if err != nil {
		return false, err
	}

	return current.CancelRequested, nil
}

func (s *Service) Succeed(ctx context.Context, job *Job) error {
	return s.finish(ctx, job, StateSucceeded, "")
}

func (s *Service) Cancelled(ctx context.Context, job *Job) error {
	return s.finish(ctx, job, StateCancelled, "")
}

func (s *Service) Fail(ctx context.Context, job *Job, cause error) error {
	if job.Attempt >= job.MaxAttempts {
		return s.finish(ctx, job, StateFailed, cause.Error())
	}

	updated, err := s.store.Job.Update().
		Where(s.held(job)...).
		SetState(StateQueued).
		SetRunAt(time.Now().Add(backoff(job.Attempt))).
		SetErrorMessage(cause.Error()).
		ClearWorker().
		ClearLeaseExpiresAt().
		Save(ctx)
	if err != nil {
		return fmt.Errorf("failed to reschedule the job: %w", err)
	}
	if updated == 0 {
		return ErrLeaseLost
	}

	return nil
}

func (s *Service) Release(ctx context.Context, job *Job) error {
	updated, err := s.store.Job.Update().
		Where(s.held(job)...).
		SetState(StateQueued).
		SetAttempt(job.Attempt - 1).
		SetRunAt(time.Now()).
		ClearWorker().
		ClearLeaseExpiresAt().
		ClearStartedAt().
		Save(ctx)
	if err != nil {
		return fmt.Errorf("failed to release the job: %w", err)
	}
	if updated == 0 {
		return ErrLeaseLost
	}

	return nil
}

func (s *Service) Cancel(ctx context.Context, id uuid.UUID) error {
	job, err := s.JobByID(ctx, id)
	if err != nil {
		return err
	}

	switch job.State {
	case StateQueued:
		return s.finish(ctx, job, StateCancelled, "")
	case StateRunning:
		if err := s.store.Job.UpdateOne(job).SetCancelRequested(true).Exec(ctx); err != nil {
			return fmt.Errorf("failed to request cancellation: %w", err)
		}
	}

	return nil
}

func (s *Service) JobByID(ctx context.Context, id uuid.UUID) (*Job, error) {
	job, err := s.store.Job.Get(ctx, id)
	if store.IsNotFound(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read the job: %w", err)
	}

	return job, nil
}

func (s *Service) Definitions() []Definition {
	return definitions
}

func (s *Service) Summaries(ctx context.Context) ([]Summary, error) {
	summaries := make([]Summary, 0, len(definitions))
	for _, definition := range definitions {
		summary, err := s.summary(ctx, definition)
		if err != nil {
			return nil, err
		}
		summaries = append(summaries, summary)
	}

	return summaries, nil
}

func (s *Service) Summary(ctx context.Context, kind string) (Summary, error) {
	definition, err := s.definition(kind)
	if err != nil {
		return Summary{}, err
	}

	return s.summary(ctx, definition)
}

func (s *Service) SetTriggers(ctx context.Context, kind string, triggers []Trigger) error {
	if _, err := s.definition(kind); err != nil {
		return err
	}
	if triggers == nil {
		triggers = []Trigger{}
	}

	err := s.store.JobSchedule.Create().
		SetKind(kind).
		SetTriggers(triggers).
		OnConflictColumns(schedulemodal.FieldKind).
		UpdateTriggers().
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to store the job triggers: %w", err)
	}

	return nil
}

func (s *Service) definition(kind string) (Definition, error) {
	for _, definition := range definitions {
		if definition.Kind == kind {
			return definition, nil
		}
	}

	return Definition{}, ErrNotFound
}

func (s *Service) summary(ctx context.Context, definition Definition) (Summary, error) {
	active, err := s.store.Job.Query().
		Where(jobmodal.Kind(definition.Kind), jobmodal.StateIn(activeStates...)).
		Order(jobmodal.ByCreatedAt(sql.OrderDesc())).
		First(ctx)
	if err != nil && !store.IsNotFound(err) {
		return Summary{}, fmt.Errorf("failed to read the active job: %w", err)
	}

	last, err := s.store.Job.Query().
		Where(jobmodal.Kind(definition.Kind), jobmodal.StateIn(endedStates...)).
		Order(jobmodal.ByFinishedAt(sql.OrderDesc())).
		First(ctx)
	if err != nil && !store.IsNotFound(err) {
		return Summary{}, fmt.Errorf("failed to read the last job: %w", err)
	}

	schedule, err := s.store.JobSchedule.Query().Where(schedulemodal.Kind(definition.Kind)).Only(ctx)
	if err != nil && !store.IsNotFound(err) {
		return Summary{}, fmt.Errorf("failed to read the job triggers: %w", err)
	}

	summary := Summary{Definition: definition, Active: active, Last: last, Triggers: []Trigger{}}
	if schedule != nil {
		summary.Triggers = schedule.Triggers
	}

	return summary, nil
}

func (s *Service) activeByDedupeKey(ctx context.Context, key string) (*Job, error) {
	job, err := s.store.Job.Query().Where(jobmodal.DedupeKey(key)).Only(ctx)
	if store.IsNotFound(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read the queued job: %w", err)
	}

	return job, nil
}

func (s *Service) finish(ctx context.Context, job *Job, state State, message string) error {
	update := s.store.Job.Update().
		Where(s.held(job)...).
		SetState(state).
		SetFinishedAt(time.Now()).
		ClearWorker().
		ClearLeaseExpiresAt().
		ClearDedupeKey()
	if message != "" {
		update.SetErrorMessage(message)
	}

	updated, err := update.Save(ctx)
	if err != nil {
		return fmt.Errorf("failed to finish the job: %w", err)
	}
	if updated == 0 {
		return ErrLeaseLost
	}

	return nil
}

func (s *Service) held(job *Job) []predicate.Job {
	return []predicate.Job{
		jobmodal.ID(job.ID),
		jobmodal.Attempt(job.Attempt),
		jobmodal.StateEQ(job.State),
	}
}

func backoff(attempt int) time.Duration {
	wait := retryBackoff << (attempt - 1)
	if wait > maxRetryBackoff || wait <= 0 {
		return maxRetryBackoff
	}

	return wait
}
