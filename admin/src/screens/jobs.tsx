import { useEffect, useState } from 'preact/hooks';

import { api } from '../api';
import * as mock from '../mock';
import { Card, Cell, Dot, Page, Pill, notify, refused, since } from '../ui';

type Job = mock.Job;

const titles: Record<string, string> = {
  RefreshLibraries: 'Refresh libraries',
  ProbeFiles: 'Probe files',
  RefreshMetadata: 'Refresh metadata',
  SweepItems: 'Sweep removed titles',
};

function took(from: string, to: string): string {
  const seconds = Math.max(0, Math.round((new Date(to).getTime() - new Date(from).getTime()) / 1000));
  if (seconds < 60) {
    return seconds + ' s';
  }
  if (seconds < 3600) {
    return Math.floor(seconds / 60) + ' min ' + (seconds % 60) + ' s';
  }

  return Math.floor(seconds / 3600) + ' h ' + Math.floor((seconds % 3600) / 60) + ' min';
}

async function fetched(): Promise<Job[] | undefined> {
  const { data } = await api.GET('/ScheduledTasks');

  return data?.map((task) => {
    const last = task.LastExecutionResult;

    return {
      name: task.Id ?? '',
      title: titles[task.Id ?? ''] ?? task.Name ?? '',
      does: task.Description ?? '',
      state: task.State === 'Idle' ? 'Idle' : 'Running',
      last:
        last?.StartTimeUtc && last.EndTimeUtc
          ? {
              at: last.EndTimeUtc,
              ok: last.Status === 'Completed',
              took: took(last.StartTimeUtc, last.EndTimeUtc),
              error: last.Status === 'Completed' ? undefined : (last.ErrorMessage ?? (last.Status === 'Cancelled' ? 'Cancelled' : 'Failed')),
            }
          : undefined,
    };
  });
}

export function Jobs() {
  const [jobs, setJobs] = useState<Job[] | undefined>(mock.mocked ? mock.jobs : undefined);
  const [failure, setFailure] = useState('');

  async function load() {
    if (mock.mocked) {
      return;
    }
    const answer = await fetched();
    if (!answer) {
      setFailure('The jobs could not be loaded.');
      return;
    }
    setJobs(answer);
  }

  async function run(job: Job, start: boolean) {
    if (mock.mocked) {
      return;
    }
    const params = { params: { path: { taskId: job.name } } };
    const { response, error } = await (start ? api.POST('/ScheduledTasks/Running/{taskId}', params) : api.DELETE('/ScheduledTasks/Running/{taskId}', params));
    if (response.ok) {
      notify(job.title + (start ? ' started.' : ' cancelled.'));
    } else {
      refused(error, start ? job.title + ' could not be started. Background work is off unless the server is told where Temporal is.' : job.title + ' could not be cancelled.');
    }
    await load();
  }

  useEffect(() => {
    void load();
    if (mock.mocked) {
      return;
    }
    const timer = setInterval(load, 5000);

    return () => clearInterval(timer);
  }, []);

  if (!jobs) {
    return (
      <Page title="Jobs" lead="Background work, run by the worker through Temporal.">
        <p class={failure ? 'failure' : 'muted'}>{failure || 'Loading…'}</p>
      </Page>
    );
  }

  return (
    <Page title="Jobs" lead="Background work, run by the worker through Temporal.">
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
            {jobs.map((job) => (
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
                  <div class="actions right">{job.state === 'Running' ? (
                      <button onClick={() => run(job, false)}>Cancel</button>
                    ) : (
                      <button class="primary" onClick={() => run(job, true)}>
                        Start
                      </button>
                    )}</div>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {mock.mocked && (
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
      )}

      <p class="muted small">
        {mock.mocked && 'Connected to ' + mock.temporal.address + ', namespace ' + mock.temporal.namespace + '. '}
        Schedules are set in Temporal, not here. Cancelling a job stops that job only, not the per-title work it already queued.
      </p>
    </Page>
  );
}
