package jobs

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.temporal.io/api/enums/v1"
	"go.temporal.io/api/serviceerror"
	"go.temporal.io/api/workflow/v1"
	"go.temporal.io/api/workflowservice/v1"
	"go.temporal.io/sdk/client"

	"github.com/FreekingDean/gojellyfin/internal/env"
)

const TaskQueue = "gojellyfin"

var ErrNotConfigured = errors.New("jobs: TEMPORAL_HOSTPORT is not set")

var ErrNoNamespace = errors.New("jobs: TEMPORAL_HOSTPORT is set but TEMPORAL_NAMESPACE is not")

var ErrNotFound = errors.New("jobs: no such job")

const (
	runningOnly  = `ExecutionStatus = "Running"`
	finishedOnly = `ExecutionStatus != "Running"`
)

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

func (c *Client) Enqueue(ctx context.Context, name string, params ...Param) error {
	connection, err := c.connection()
	if err != nil {
		return err
	}

	held := make(Params, len(params))
	for _, param := range params {
		held[param.Name] = param.Value
	}

	_, err = connection.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
		ID:                       held.id(name),
		TaskQueue:                TaskQueue,
		WorkflowExecutionTimeout: runTimeoutMax,
		WorkflowIDReusePolicy:    enums.WORKFLOW_ID_REUSE_POLICY_ALLOW_DUPLICATE,
	}, runWorkflow, name, held)

	var running *serviceerror.WorkflowExecutionAlreadyStarted
	if errors.As(err, &running) {
		return nil
	}

	return err
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
	for _, job := range s.registry.Startable() {
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

func (s *Service) Start(ctx context.Context, name string, params ...Param) error {
	job, err := s.registry.Find(name)
	if err != nil {
		return err
	}
	if !job.Startable {
		return fmt.Errorf("%w: %s is only ever enqueued by another job", ErrNotFound, name)
	}

	return s.client.Enqueue(ctx, name, params...)
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

	running, err := executions(ctx, connection, job.Name, runningOnly)
	if err != nil {
		return err
	}

	for _, execution := range running {
		if err := connection.CancelWorkflow(ctx, execution.GetExecution().GetWorkflowId(), ""); err != nil {
			return err
		}
	}

	return nil
}

func executions(
	ctx context.Context,
	connection client.Client,
	name string,
	only string,
) ([]*workflow.WorkflowExecutionInfo, error) {
	query := fmt.Sprintf(
		"(WorkflowId = %q OR WorkflowId STARTS_WITH %q) AND %s",
		name, name+idSeparator, only,
	)

	listed, err := connection.ListWorkflow(ctx, &workflowservice.ListWorkflowExecutionsRequest{Query: query})
	if err != nil {
		return nil, err
	}

	return listed.GetExecutions(), nil
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

	running, err := executions(ctx, connection, job.Name, runningOnly)
	if err != nil {
		return Status{}, err
	}
	if len(running) > 0 {
		status.State = StateRunning

		return status, nil
	}

	finished, err := executions(ctx, connection, job.Name, finishedOnly)
	if err != nil {
		return Status{}, err
	}
	if len(finished) == 0 {
		return status, nil
	}

	info := finished[0]
	cancelled := info.GetStatus() == enums.WORKFLOW_EXECUTION_STATUS_CANCELED ||
		info.GetStatus() == enums.WORKFLOW_EXECUTION_STATUS_TERMINATED
	if cancelled {
		status.State = StateCancelling
	}

	status.Last = &Result{
		Succeeded: info.GetStatus() == enums.WORKFLOW_EXECUTION_STATUS_COMPLETED,
		Cancelled: cancelled,
		StartedAt: info.GetStartTime().AsTime(),
		EndedAt:   info.GetCloseTime().AsTime(),
	}

	return status, nil
}
