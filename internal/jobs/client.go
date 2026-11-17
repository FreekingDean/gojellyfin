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
	"go.temporal.io/sdk/contrib/opentelemetry"
	"go.temporal.io/sdk/interceptor"
	"google.golang.org/grpc"

	"github.com/FreekingDean/gojellyfin/internal/env"
	"github.com/FreekingDean/gojellyfin/internal/observability/tracing"
)

const (
	TaskQueue     = "gojellyfin"
	MetadataQueue = "gojellyfin-metadata"
	RefreshQueue  = "gojellyfin-refresh"
	SegmentsQueue = "gojellyfin-segments"
	ProbeQueue    = "gojellyfin-probe"
)

var ErrNotConfigured = errors.New("jobs: TEMPORAL_HOSTPORT is not set")

var ErrNoNamespace = errors.New("jobs: TEMPORAL_HOSTPORT is set but TEMPORAL_NAMESPACE is not")

var ErrNotFound = errors.New("jobs: no such job")

const (
	runningOnly  = `ExecutionStatus = "Running"`
	finishedOnly = `ExecutionStatus != "Running"`
)

var (
	queued   = []string{"RunWorkflow:", "StartActivity:"}
	executed = []string{"RunActivity:"}
)

type Client struct {
	client client.Client
}

func NewClient(config env.Config, traces *tracing.Tracing) (*Client, error) {
	if config.Temporal.HostPort == "" {
		return &Client{}, nil
	}
	if config.Temporal.Namespace == "" {
		return nil, ErrNoNamespace
	}

	options := client.Options{
		HostPort:  config.Temporal.HostPort,
		Namespace: config.Temporal.Namespace,
		ConnectionOptions: client.ConnectionOptions{
			DialOptions: []grpc.DialOption{grpc.WithChainUnaryInterceptor(tracing.UnaryClientInterceptor)},
		},
	}
	if traces.Enabled() {
		traced, err := tracer(traces)
		if err != nil {
			return nil, err
		}
		options.Interceptors = []interceptor.ClientInterceptor{traced}
	}

	connected, err := client.Dial(options)
	if err != nil {
		return nil, err
	}

	return &Client{client: connected}, nil
}

func tracer(traces *tracing.Tracing) (interceptor.Interceptor, error) {
	return opentelemetry.NewTracingInterceptor(opentelemetry.TracerOptions{
		Tracer:            traces.Tracer(),
		TextMapPropagator: traces.Propagator(),
		SpanStarter:       tracing.Starts(queued, executed),
	})
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
	Job      Job
	State    State
	Last     *Result
	Progress *float64
}

func (c *Client) start(ctx context.Context, queue string, limit time.Duration, name string, params ...Param) error {
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
		TaskQueue:                queue,
		WorkflowExecutionTimeout: limit,
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

	return s.client.start(ctx, job.queue(), runTimeoutMax, name, params...)
}

func (s *Service) Enqueue(ctx context.Context, name string, params ...Param) error {
	job, err := s.registry.Find(name)
	if err != nil {
		return err
	}

	return s.client.start(ctx, job.queue(), runTimeoutMax, name, params...)
}

func (s *Service) Enabled() bool {
	return s.client.Enabled()
}

func (s *Service) Pollers(ctx context.Context) (map[string]int, error) {
	connection, err := s.client.connection()
	if err != nil {
		return nil, err
	}

	pollers := map[string]int{}
	for _, queue := range s.registry.Queues() {
		described, err := connection.DescribeTaskQueue(ctx, queue, enums.TASK_QUEUE_TYPE_WORKFLOW)
		if err != nil {
			return nil, err
		}
		pollers[queue] = len(described.GetPollers())
	}

	return pollers, nil
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
		id := running[0].GetExecution().GetWorkflowId()
		status.State = s.runningState(ctx, connection, id)
		if job.Outstanding != nil {
			status.Progress = s.progress(ctx, id)
		}

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

	status.Last = &Result{
		Succeeded: info.GetStatus() == enums.WORKFLOW_EXECUTION_STATUS_COMPLETED,
		Cancelled: cancelled,
		StartedAt: info.GetStartTime().AsTime(),
		EndedAt:   info.GetCloseTime().AsTime(),
	}

	return status, nil
}

func (s *Service) runningState(ctx context.Context, connection client.Client, id string) State {
	described, err := connection.DescribeWorkflow(ctx, id, "")
	if err != nil {
		return StateRunning
	}

	for _, pending := range described.RawResponse.GetPendingActivities() {
		if pending.GetState() == enums.PENDING_ACTIVITY_STATE_CANCEL_REQUESTED {
			return StateCancelling
		}
	}

	return StateRunning
}
