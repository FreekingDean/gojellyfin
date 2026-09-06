package jobs

import (
	"context"
	"errors"
	"time"

	"go.temporal.io/api/enums/v1"
	"go.temporal.io/api/serviceerror"
	"go.temporal.io/sdk/client"

	"github.com/FreekingDean/gojellyfin/internal/env"
)

var ErrNotConfigured = errors.New("jobs: TEMPORAL_HOSTPORT is not set")

var ErrNoNamespace = errors.New("jobs: TEMPORAL_HOSTPORT is set but TEMPORAL_NAMESPACE is not")

var ErrNotFound = errors.New("jobs: no such job")

type Client struct {
	client client.Client
}

func NewClient(config env.Config) (*Client, error) {
	if config.Temporal.HostPort == "" {
		return &Client{}, nil
	}
	if config.Temporal.Namespace == "" {
		return nil, ErrNoNamespace
	}

	connected, err := client.Dial(client.Options{
		HostPort:  config.Temporal.HostPort,
		Namespace: config.Temporal.Namespace,
	})
	if err != nil {
		return nil, err
	}

	return &Client{client: connected}, nil
}

func (c *Client) Enabled() bool {
	return c.client != nil
}

func (c *Client) connection() (client.Client, error) {
	if c.client == nil {
		return nil, ErrNotConfigured
	}

	return c.client, nil
}

func (c *Client) Close() {
	if c.client != nil {
		c.client.Close()
	}
}

type State string

const (
	StateIdle       State = "Idle"
	StateRunning    State = "Running"
	StateCancelling State = "Cancelling"
)

type Result struct {
	Succeeded bool
	Cancelled bool
	StartedAt time.Time
	EndedAt   time.Time
}

type Status struct {
	Job   Job
	State State
	Last  *Result
}

type Service struct {
	client   *Client
	registry *Registry
}

func NewService(client *Client, registry *Registry) *Service {
	return &Service{client: client, registry: registry}
}

func (s *Service) All(ctx context.Context) ([]Status, error) {
	statuses := make([]Status, 0)
	for _, job := range s.registry.All() {
		status, err := s.status(ctx, job)
		if err != nil {
			return nil, err
		}
		statuses = append(statuses, status)
	}

	return statuses, nil
}

func (s *Service) Status(ctx context.Context, name string) (Status, error) {
	job, err := s.registry.Find(name)
	if err != nil {
		return Status{}, err
	}

	return s.status(ctx, job)
}

func (s *Service) Start(ctx context.Context, name string, options Options) error {
	job, err := s.registry.Find(name)
	if err != nil {
		return err
	}

	connection, err := s.client.connection()
	if err != nil {
		return err
	}

	_, err = connection.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
		ID:                       job.Name(),
		TaskQueue:                job.Queue(),
		WorkflowExecutionTimeout: runTimeoutMax,
		WorkflowIDReusePolicy:    enums.WORKFLOW_ID_REUSE_POLICY_ALLOW_DUPLICATE,
	}, job.Name(), options)
	var running *serviceerror.WorkflowExecutionAlreadyStarted
	if errors.As(err, &running) {
		return nil
	}

	return err
}

func (s *Service) Cancel(ctx context.Context, name string) error {
	job, err := s.registry.Find(name)
	if err != nil {
		return err
	}

	connection, err := s.client.connection()
	if err != nil {
		return err
	}

	return connection.CancelWorkflow(ctx, job.Name(), "")
}

func (s *Service) status(ctx context.Context, job Job) (Status, error) {
	status := Status{Job: job, State: StateIdle}

	connection, err := s.client.connection()
	if errors.Is(err, ErrNotConfigured) {
		return status, nil
	}
	if err != nil {
		return Status{}, err
	}

	described, err := connection.DescribeWorkflowExecution(ctx, job.Name(), "")
	var missing *serviceerror.NotFound
	if errors.As(err, &missing) {
		return status, nil
	}
	if err != nil {
		return Status{}, err
	}

	info := described.GetWorkflowExecutionInfo()
	switch info.GetStatus() {
	case enums.WORKFLOW_EXECUTION_STATUS_RUNNING:
		status.State = StateRunning
		return status, nil
	case enums.WORKFLOW_EXECUTION_STATUS_CANCELED, enums.WORKFLOW_EXECUTION_STATUS_TERMINATED:
		status.State = StateCancelling
	}

	status.Last = &Result{
		Succeeded: info.GetStatus() == enums.WORKFLOW_EXECUTION_STATUS_COMPLETED,
		Cancelled: info.GetStatus() == enums.WORKFLOW_EXECUTION_STATUS_CANCELED ||
			info.GetStatus() == enums.WORKFLOW_EXECUTION_STATUS_TERMINATED,
		StartedAt: info.GetStartTime().AsTime(),
		EndedAt:   info.GetCloseTime().AsTime(),
	}

	return status, nil
}
