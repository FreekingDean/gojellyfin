package tasks

import (
	"context"
	"errors"
	"maps"
	"slices"
	"strings"
	"sync"
	"time"
)

var ErrNotFound = errors.New("task not found")

type State string

const (
	StateIdle       State = "Idle"
	StateCancelling State = "Cancelling"
	StateRunning    State = "Running"
)

type Status string

const (
	StatusCancelled Status = "Cancelled"
	StatusCompleted Status = "Completed"
	StatusFailed    Status = "Failed"
)

type Definition struct {
	ID          string
	Name        string
	Description string
	Category    string
	Run         func(ctx context.Context) error
}

type Trigger struct {
	Type            string
	DayOfWeek       *string
	IntervalTicks   *int64
	TimeOfDayTicks  *int64
	MaxRuntimeTicks *int64
}

type Result struct {
	StartedAt time.Time
	EndedAt   time.Time
	Status    Status
	Error     string
}

type Info struct {
	ID          string
	Name        string
	Description string
	Category    string
	State       State
	LastResult  *Result
	Triggers    []Trigger
}

type task struct {
	definition Definition

	mu       sync.Mutex
	state    State
	last     *Result
	triggers []Trigger
	cancel   context.CancelFunc
}

type Registry struct {
	mu    sync.Mutex
	tasks map[string]*task
}

func New() *Registry {
	return &Registry{tasks: make(map[string]*task)}
}

func (r *Registry) Register(definition Definition) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.tasks[definition.ID] = &task{definition: definition, state: StateIdle}
}

func (r *Registry) Tasks() []Info {
	r.mu.Lock()
	registered := slices.Collect(maps.Values(r.tasks))
	r.mu.Unlock()

	infos := make([]Info, 0, len(registered))
	for _, registeredTask := range registered {
		infos = append(infos, registeredTask.info())
	}
	slices.SortFunc(infos, func(a, b Info) int { return strings.Compare(a.Name, b.Name) })

	return infos
}

func (r *Registry) Task(id string) (Info, error) {
	found, err := r.lookup(id)
	if err != nil {
		return Info{}, err
	}

	return found.info(), nil
}

func (r *Registry) Start(id string) error {
	found, err := r.lookup(id)
	if err != nil {
		return err
	}
	found.start()

	return nil
}

func (r *Registry) Stop(id string) error {
	found, err := r.lookup(id)
	if err != nil {
		return err
	}
	found.stop()

	return nil
}

func (r *Registry) SetTriggers(id string, triggers []Trigger) error {
	found, err := r.lookup(id)
	if err != nil {
		return err
	}

	found.mu.Lock()
	defer found.mu.Unlock()
	found.triggers = slices.Clone(triggers)

	return nil
}

func (r *Registry) lookup(id string) (*task, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	found, ok := r.tasks[id]
	if !ok {
		return nil, ErrNotFound
	}

	return found, nil
}

func (t *task) start() {
	t.mu.Lock()
	if t.state != StateIdle {
		t.mu.Unlock()
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	t.state = StateRunning
	t.cancel = cancel
	startedAt := time.Now()
	t.mu.Unlock()

	go func() {
		defer cancel()
		t.finish(startedAt, t.definition.Run(ctx))
	}()
}

func (t *task) stop() {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.state != StateRunning {
		return
	}
	t.state = StateCancelling
	t.cancel()
}

func (t *task) finish(startedAt time.Time, err error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	result := Result{StartedAt: startedAt, EndedAt: time.Now(), Status: StatusCompleted}
	switch {
	case t.state == StateCancelling:
		result.Status = StatusCancelled
	case err != nil:
		result.Status = StatusFailed
		result.Error = err.Error()
	}

	t.state = StateIdle
	t.cancel = nil
	t.last = &result
}

func (t *task) info() Info {
	t.mu.Lock()
	defer t.mu.Unlock()

	info := Info{
		ID:          t.definition.ID,
		Name:        t.definition.Name,
		Description: t.definition.Description,
		Category:    t.definition.Category,
		State:       t.state,
		Triggers:    slices.Clone(t.triggers),
	}
	if t.last != nil {
		last := *t.last
		info.LastResult = &last
	}

	return info
}
