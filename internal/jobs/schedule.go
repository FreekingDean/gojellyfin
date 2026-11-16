package jobs

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"go.temporal.io/api/common/v1"
	"go.temporal.io/api/enums/v1"
	"go.temporal.io/api/serviceerror"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/converter"
	sdktemporal "go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

const (
	scheduleWorkflow   = "ScheduleJob"
	scheduledRunPrefix = "scheduled/"
	scheduleCatchup    = 10 * time.Minute
	scheduleStartLimit = time.Hour
)

var ErrInvalidTrigger = errors.New("jobs: invalid trigger")

type TriggerKind string

const (
	TriggerDaily    TriggerKind = "Daily"
	TriggerWeekly   TriggerKind = "Weekly"
	TriggerInterval TriggerKind = "Interval"
	TriggerStartup  TriggerKind = "Startup"
)

type Trigger struct {
	Kind       TriggerKind
	TimeOfDay  time.Duration
	Interval   time.Duration
	Day        time.Weekday
	MaxRuntime time.Duration
}

func (t Trigger) validate() error {
	switch t.Kind {
	case TriggerInterval:
		if t.Interval <= 0 {
			return fmt.Errorf("%w: an interval trigger needs a positive interval", ErrInvalidTrigger)
		}
	case TriggerDaily, TriggerWeekly:
		if t.TimeOfDay < 0 || t.TimeOfDay >= 24*time.Hour {
			return fmt.Errorf("%w: a time of day must lie within one day, got %s", ErrInvalidTrigger, t.TimeOfDay)
		}
		if t.Day < time.Sunday || t.Day > time.Saturday {
			return fmt.Errorf("%w: no such day of the week %d", ErrInvalidTrigger, t.Day)
		}
	case TriggerStartup:
	default:
		return fmt.Errorf("%w: no such kind %q", ErrInvalidTrigger, t.Kind)
	}
	if t.MaxRuntime < 0 {
		return fmt.Errorf("%w: a time limit cannot be negative", ErrInvalidTrigger)
	}

	return nil
}

func (t Trigger) calendar() client.ScheduleCalendarSpec {
	at := client.ScheduleCalendarSpec{
		Second: []client.ScheduleRange{{Start: int(t.TimeOfDay % time.Minute / time.Second)}},
		Minute: []client.ScheduleRange{{Start: int(t.TimeOfDay % time.Hour / time.Minute)}},
		Hour:   []client.ScheduleRange{{Start: int(t.TimeOfDay / time.Hour)}},
	}
	if t.Kind == TriggerWeekly {
		at.DayOfWeek = []client.ScheduleRange{{Start: int(t.Day)}}
	}

	return at
}

func spec(triggers []Trigger) client.ScheduleSpec {
	var timed client.ScheduleSpec
	for _, trigger := range triggers {
		switch trigger.Kind {
		case TriggerInterval:
			timed.Intervals = append(timed.Intervals, client.ScheduleIntervalSpec{Every: trigger.Interval})
		case TriggerDaily, TriggerWeekly:
			timed.Calendars = append(timed.Calendars, trigger.calendar())
		case TriggerStartup:
		}
	}

	return timed
}

func runLimit(triggers []Trigger) time.Duration {
	limit := runTimeoutMax
	for _, trigger := range triggers {
		if trigger.Kind != TriggerStartup && trigger.MaxRuntime > 0 && trigger.MaxRuntime < limit {
			limit = trigger.MaxRuntime
		}
	}

	return limit
}

func startsOnBoot(triggers []Trigger) bool {
	return slices.ContainsFunc(triggers, func(trigger Trigger) bool { return trigger.Kind == TriggerStartup })
}

func (s *Service) Triggers(ctx context.Context, name string) ([]Trigger, error) {
	job, err := s.registry.Find(name)
	if err != nil {
		return nil, err
	}
	if !job.Startable {
		return nil, fmt.Errorf("%w: %s is only ever enqueued by another job", ErrNotFound, name)
	}

	connection, err := s.client.connection()
	if errors.Is(err, ErrNotConfigured) {
		return []Trigger{}, nil
	}
	if err != nil {
		return nil, err
	}

	described, err := connection.ScheduleClient().GetHandle(ctx, name).Describe(ctx)
	var missing *serviceerror.NotFound
	if errors.As(err, &missing) {
		return []Trigger{}, nil
	}
	if err != nil {
		return nil, err
	}

	return scheduledTriggers(described.Schedule.Action)
}

func scheduledTriggers(action client.ScheduleAction) ([]Trigger, error) {
	started, ok := action.(*client.ScheduleWorkflowAction)
	if !ok || len(started.Args) < 2 {
		return []Trigger{}, nil
	}

	payload, ok := started.Args[1].(*common.Payload)
	if !ok {
		return []Trigger{}, nil
	}

	triggers := []Trigger{}
	if err := converter.GetDefaultDataConverter().FromPayload(payload, &triggers); err != nil {
		return nil, fmt.Errorf("jobs: the schedule's triggers do not decode: %w", err)
	}

	return triggers, nil
}

func (s *Service) Schedule(ctx context.Context, name string, triggers []Trigger) error {
	job, err := s.registry.Find(name)
	if err != nil {
		return err
	}
	if !job.Startable {
		return fmt.Errorf("%w: %s is only ever enqueued by another job", ErrNotFound, name)
	}
	for _, trigger := range triggers {
		if err := trigger.validate(); err != nil {
			return err
		}
	}

	connection, err := s.client.connection()
	if err != nil {
		return err
	}
	schedules := connection.ScheduleClient()
	handle := schedules.GetHandle(ctx, name)

	if len(triggers) == 0 {
		err := handle.Delete(ctx)
		var missing *serviceerror.NotFound
		if errors.As(err, &missing) {
			return nil
		}

		return err
	}

	timed := spec(triggers)
	action := &client.ScheduleWorkflowAction{
		ID:                       scheduledRunPrefix + name,
		Workflow:                 scheduleWorkflow,
		Args:                     []any{name, triggers},
		TaskQueue:                TaskQueue,
		WorkflowExecutionTimeout: scheduleStartLimit,
	}

	_, err = schedules.Create(ctx, client.ScheduleOptions{
		ID:             name,
		Spec:           timed,
		Action:         action,
		Overlap:        enums.SCHEDULE_OVERLAP_POLICY_SKIP,
		CatchupWindow:  scheduleCatchup,
		PauseOnFailure: false,
	})
	if !errors.Is(err, sdktemporal.ErrScheduleAlreadyRunning) {
		return err
	}

	return handle.Update(ctx, client.ScheduleUpdateOptions{
		DoUpdate: func(input client.ScheduleUpdateInput) (*client.ScheduleUpdate, error) {
			replaced := input.Description.Schedule
			replaced.Spec = &timed
			replaced.Action = action

			return &client.ScheduleUpdate{Schedule: &replaced}, nil
		},
	})
}

func (s *Service) startOnBoot(ctx context.Context) error {
	var failed []error
	for _, job := range s.registry.Startable() {
		triggers, err := s.Triggers(ctx, job.Name)
		if err != nil {
			failed = append(failed, fmt.Errorf("%s: %w", job.Name, err))

			continue
		}
		if !startsOnBoot(triggers) {
			continue
		}
		if err := s.client.start(ctx, job.queue(), runTimeoutMax, job.Name); err != nil {
			failed = append(failed, fmt.Errorf("%s: %w", job.Name, err))
		}
	}

	return errors.Join(failed...)
}

func (w *Worker) schedule(ctx workflow.Context, name string, triggers []Trigger) error {
	job, err := w.service.registry.Find(name)
	if err != nil {
		return err
	}

	ctx = workflow.WithChildOptions(ctx, workflow.ChildWorkflowOptions{
		WorkflowID:               name,
		TaskQueue:                job.queue(),
		WorkflowExecutionTimeout: runLimit(triggers),
		WorkflowIDReusePolicy:    enums.WORKFLOW_ID_REUSE_POLICY_ALLOW_DUPLICATE,
		ParentClosePolicy:        enums.PARENT_CLOSE_POLICY_ABANDON,
	})

	err = workflow.ExecuteChildWorkflow(ctx, runWorkflow, name, Params{}).GetChildWorkflowExecution().Get(ctx, nil)
	if sdktemporal.IsWorkflowExecutionAlreadyStartedError(err) {
		workflow.GetLogger(ctx).Info("skipped a scheduled run, one is already going", "job", name)

		return nil
	}

	return err
}
