import * as mock from '../mock';
import { Card, Cell, Dot, Page, Pill, since } from '../ui';

export function Jobs() {
  return (
    <Page
      title="Jobs"
      lead="Background work, run by the worker through Temporal."
      action={
        <a class="button" href="#temporal">
          Open Temporal
        </a>
      }
    >
      <div class="card flush">
        <table class="rows">
          <colgroup>
            <col style="width:42%" />
            <col style="width:14%" />
            <col style="width:28%" />
            <col style="width:16%" />
          </colgroup>
          <thead>
            <tr>
              <th>Job</th>
              <th>State</th>
              <th>Last run</th>
              <th />
            </tr>
          </thead>
          <tbody>
            {mock.jobs.map((job) => (
              <tr key={job.name}>
                <Cell main={job.title} sub={job.does} />
                <td>{job.state === 'Running' ? <Pill tone="ok">Running</Pill> : <Pill>Idle</Pill>}</td>
                {job.state === 'Running' && job.started ? (
                  <Cell main={'Started ' + since(job.started)} sub={job.last && 'Before that: ' + since(job.last.at)} />
                ) : job.last ? (
                  <Cell
                    main={
                      <>
                        <Dot tone={job.last.ok ? 'ok' : 'bad'} />
                        {since(job.last.at)}, took {job.last.took}
                      </>
                    }
                    sub={job.last.error}
                    tone={job.last.error ? 'bad' : undefined}
                  />
                ) : (
                  <Cell main="Never run" />
                )}
                <td>
                  <div class="actions right">{job.state === 'Running' ? <button>Cancel</button> : <button class="primary">Start</button>}</div>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <div class="grid three spaced">
        {mock.queues.map((queue) => (
          <Card key={queue.name} title={queue.name + ' queue'}>
            <div class="stat">
              <span class="big">{queue.waiting}</span>
              <span class="muted">waiting, {queue.running} running</span>
            </div>
            <p class="muted small">{queue.holds}</p>
          </Card>
        ))}
      </div>

      <p class="muted small">
        Connected to {mock.temporal.address}, namespace {mock.temporal.namespace}. Schedules are set in Temporal, not here. Cancelling a job stops that
        job only, not the per-title work it already queued.
      </p>
    </Page>
  );
}
