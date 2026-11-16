package jobs

import (
	"context"
	"time"

	sdktemporal "go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

const (
	progressChange = "progress"
	progressQuery  = "progress"
	countSuffix    = "/Outstanding"

	progressEvery    = time.Minute
	progressPatience = 10 * time.Minute
	countTimeout     = time.Minute
	countAttempts    = 3
)

type Progress struct {
	StartedWith int
	Outstanding int
}

func (p Progress) Percent() *float64 {
	if p.StartedWith <= 0 {
		return nil
	}

	done := float64(p.StartedWith-min(p.Outstanding, p.StartedWith)) / float64(p.StartedWith) * 100

	return &done
}

func countName(name string) string {
	return name + countSuffix
}

func count(ctx workflow.Context, name string) (int, error) {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: countTimeout,
		RetryPolicy:         &sdktemporal.RetryPolicy{MaximumAttempts: countAttempts},
	})

	var outstanding int
	err := workflow.ExecuteActivity(ctx, countName(name)).Get(ctx, &outstanding)

	return outstanding, err
}

func watched(ctx workflow.Context, name string, params Params) error {
	progress := Progress{}
	if err := workflow.SetQueryHandler(ctx, progressQuery, func() (Progress, error) { return progress, nil }); err != nil {
		return err
	}

	started, err := count(ctx, name)
	if err != nil {
		return workflow.ExecuteActivity(ctx, name, params).Get(ctx, nil)
	}
	progress = Progress{StartedWith: started, Outstanding: started}

	running := workflow.ExecuteActivity(ctx, name, params)
	finished := false
	var failed error
	lowest, since := started, workflow.Now(ctx)

	for {
		waiting := workflow.NewSelector(ctx)
		if !finished {
			waiting.AddFuture(running, func(done workflow.Future) {
				finished = true
				failed = done.Get(ctx, nil)
			})
		}
		waiting.AddFuture(workflow.NewTimer(ctx, progressEvery), func(workflow.Future) {})
		waiting.Select(ctx)

		if failed != nil {
			return failed
		}
		if ctx.Err() != nil {
			if !finished {
				return running.Get(ctx, nil)
			}

			return sdktemporal.NewCanceledError()
		}

		outstanding, err := count(ctx, name)
		if err != nil {
			if finished {
				return nil
			}

			continue
		}
		progress.Outstanding = outstanding

		if outstanding < lowest {
			lowest, since = outstanding, workflow.Now(ctx)
		}
		if finished && (outstanding == 0 || workflow.Now(ctx).Sub(since) >= progressPatience) {
			return nil
		}
	}
}

func (s *Service) progress(ctx context.Context, id string) *float64 {
	connection, err := s.client.connection()
	if err != nil {
		return nil
	}

	answered, err := connection.QueryWorkflow(ctx, id, "", progressQuery)
	if err != nil {
		return nil
	}

	var progress Progress
	if err := answered.Get(&progress); err != nil {
		return nil
	}

	return progress.Percent()
}
