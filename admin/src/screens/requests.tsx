import { useEffect, useState } from 'preact/hooks';

import { api } from '../api';
import type { Schemas } from '../api';
import * as mock from '../mock';
import { Served, attempted, client } from '../playback';
import { Cell, Dot, Page, Pill, Tabs, notify, refused, since } from '../ui';
import type { Tone } from '../ui';

type Title = Schemas['RequestedTitle'];
type Report = Schemas['Report'];
type Tab = 'requests' | 'reports';
type Closing = 'Replace' | 'Resolve' | 'Dismiss';

function standing(title: Title): { label: string; tone: Tone } {
  if (title.Requests.some((entry) => entry.Status === 'Pending')) {
    return { label: 'Awaiting approval', tone: 'warn' };
  }
  if (title.Requests.every((entry) => entry.Status === 'Declined')) {
    return { label: 'Declined', tone: 'off' };
  }
  if (title.Submissions.length === 0) {
    return { label: 'Not sent', tone: 'bad' };
  }
  if (title.Submissions.some((submission) => !submission.AcceptedAt)) {
    return { label: 'Partly refused', tone: 'bad' };
  }

  return { label: 'Sent to downloader', tone: 'ok' };
}

function refusal(title: Title): string | undefined {
  const refused = title.Submissions.filter((submission) => !submission.AcceptedAt);
  if (refused.length === 0) {
    return undefined;
  }

  return refused.map((submission) => submission.SourceName + ': ' + (submission.Error || 'no reason given')).join(', ');
}

const reportTones: Record<string, Tone> = { Open: 'warn', Resolved: 'ok', Dismissed: 'off' };

export function Requests() {
  const [tab, setTab] = useState<Tab>('requests');
  const [titles, setTitles] = useState<Title[]>(mock.mocked ? mock.titles : []);
  const [reports, setReports] = useState<Report[]>(mock.mocked ? mock.reports : []);
  const [failure, setFailure] = useState('');

  async function load() {
    if (mock.mocked) {
      return;
    }
    const [requested, reported] = await Promise.all([api.GET('/GoJellyfin/Requests'), api.GET('/GoJellyfin/Reports')]);
    if (!requested.data || !reported.data) {
      setFailure('The requests and reports could not be loaded.');
      return;
    }
    setTitles(requested.data);
    setReports(reported.data);
  }

  async function decide(title: Title, decision: 'Approve' | 'Decline') {
    const pending = title.Requests.filter((entry) => entry.Status === 'Pending');
    const entries = pending.length > 0 ? pending : title.Requests.slice(0, 1);

    if (mock.mocked) {
      const status = decision === 'Approve' ? 'Approved' : 'Declined';
      setTitles(
        titles.map((each) =>
          each === title
            ? {
                ...each,
                Requests: each.Requests.map((entry) => (entries.includes(entry) ? { ...entry, Status: status } : entry)),
                Submissions: decision === 'Approve' ? [{ SourceName: each.Kind === 'Movie' ? 'Radarr HD' : 'Sonarr', AcceptedAt: new Date().toISOString() }] : each.Submissions,
              }
            : each,
        ),
      );
      return;
    }

    const answers = await Promise.all(
      entries.map((entry) => {
        const params = { params: { path: { requestId: entry.Id } } };

        return decision === 'Approve'
          ? api.POST('/GoJellyfin/Requests/{requestId}/Approve', params)
          : api.POST('/GoJellyfin/Requests/{requestId}/Decline', params);
      }),
    );
    const failed = answers.find((answer) => !answer.response.ok);
    if (failed) {
      refused(
        failed.error,
        decision === 'Approve'
          ? 'The request for ' + title.Name + ' could not be approved. Check that a downloader takes requests for this kind of title.'
          : 'The request for ' + title.Name + ' could not be declined.',
      );
    } else {
      notify(title.Name + (decision === 'Approve' ? ' approved.' : ' declined.'));
    }
    await load();
  }

  async function close(report: Report, decision: Closing) {
    if (mock.mocked) {
      const status = decision === 'Dismiss' ? 'Dismissed' : 'Resolved';
      setReports(reports.map((each) => (each === report ? { ...each, Status: status } : each)));
      return;
    }

    const params = { params: { path: { reportId: report.Id } } };
    const { response, error } = await (decision === 'Replace'
      ? api.POST('/GoJellyfin/Reports/{reportId}/Replace', params)
      : decision === 'Resolve'
        ? api.POST('/GoJellyfin/Reports/{reportId}/Resolve', params)
        : api.POST('/GoJellyfin/Reports/{reportId}/Dismiss', params));
    if (!response.ok) {
      refused(error, decision === 'Replace' ? 'The downloader did not replace this copy of ' + report.ItemName + '. Check that it is reachable.' : 'The report on ' + report.ItemName + ' could not be closed.');
    } else {
      notify(
        decision === 'Replace'
          ? 'Asked the downloader for another copy of ' + report.ItemName + '.'
          : 'Report on ' + report.ItemName + (decision === 'Resolve' ? ' resolved.' : ' dismissed.'),
      );
    }
    await load();
  }

  useEffect(() => {
    void load();
  }, []);

  const waiting = titles.filter((title) => title.Requests.some((entry) => entry.Status === 'Pending')).length;
  const open = reports.filter((report) => report.Status === 'Open').length;

  return (
    <Page title="Requests and Reports" lead="Titles people asked for, and copies people say are broken.">
      <Tabs
        tabs={[
          { id: 'requests', label: 'Requests', count: waiting },
          { id: 'reports', label: 'Reports', count: open },
        ]}
        selected={tab}
        select={setTab}
      />

      {failure && (
        <p class="failure" role="alert">
          {failure}
        </p>
      )}

      {tab === 'requests' && <RequestTable titles={titles} decide={decide} />}
      {tab === 'reports' && <ReportTable reports={reports} close={close} />}
    </Page>
  );
}

function RequestTable({ titles, decide }: { titles: Title[]; decide: (title: Title, decision: 'Approve' | 'Decline') => Promise<void> }) {
  if (titles.length === 0) {
    return <p class="card muted">Nobody has asked for anything yet.</p>;
  }

  return (
    <>
      <div class="card flush">
        <table class="rows">
          <colgroup>
            <col style="width:24%" />
            <col style="width:16%" />
            <col style="width:26%" />
            <col style="width:14%" />
            <col style="width:20%" />
          </colgroup>
          <thead>
            <tr>
              <th>Title</th>
              <th>Asked by</th>
              <th>Downloaders</th>
              <th>Status</th>
              <th />
            </tr>
          </thead>
          <tbody>
            {titles.map((title) => {
              const state = standing(title);
              const first = title.Requests[0];

              return (
                <tr key={title.ItemId}>
                  <Cell main={<a href={'/web/#/details?id=' + title.ItemId}>{title.Name}</a>} sub={title.Kind} />
                  <Cell main={title.Requests.map((entry) => entry.UserName).join(', ')} sub={first && since(first.CreatedAt)} />
                  <Cell
                    main={
                      title.Submissions.length === 0 ? (
                        <span class="muted">Not asked yet</span>
                      ) : (
                        title.Submissions.map((submission) => (
                          <span class="answer" key={submission.SourceName}>
                            <Dot tone={submission.AcceptedAt ? 'ok' : 'bad'} />
                            {submission.SourceName}
                          </span>
                        ))
                      )
                    }
                    sub={refusal(title)}
                    tone={refusal(title) ? 'bad' : undefined}
                  />
                  <td>
                    <Pill tone={state.tone}>{state.label}</Pill>
                  </td>
                  <td>
                    <div class="actions right">
                      {state.label === 'Awaiting approval' && (
                        <>
                          <button onClick={() => decide(title, 'Decline')}>Decline</button>
                          <button class="primary" onClick={() => decide(title, 'Approve')}>
                            Approve
                          </button>
                        </>
                      )}
                      {(state.label === 'Partly refused' || state.label === 'Not sent') && <button onClick={() => decide(title, 'Approve')}>Ask again</button>}
                      {state.label === 'Declined' && <button onClick={() => decide(title, 'Approve')}>Approve after all</button>}
                    </div>
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>
      <p class="muted small">
        Approving sends the title to every downloader of its kind that takes requests. Declining tells no downloader, and the request still counts
        against that person's week.
      </p>
    </>
  );
}

function ReportTable({ reports, close }: { reports: Report[]; close: (report: Report, decision: Closing) => Promise<void> }) {
  if (reports.length === 0) {
    return <p class="card muted">Nothing has been reported.</p>;
  }

  return (
    <>
      <div class="card flush">
        <table class="rows">
          <colgroup>
            <col style="width:24%" />
            <col style="width:16%" />
            <col style="width:26%" />
            <col style="width:14%" />
            <col style="width:20%" />
          </colgroup>
          <thead>
            <tr>
              <th>Title</th>
              <th>Reported by</th>
              <th>What is wrong</th>
              <th>Status</th>
              <th />
            </tr>
          </thead>
          <tbody>
            {reports.map((report) => (
              <tr key={report.Id}>
                <Cell main={<a href={'/web/#/details?id=' + report.ItemId}>{report.ItemName}</a>} sub={<span title={report.Path ?? ''}>{report.Path}</span>} />
                <td>
                  <div class="line">{report.UserName || report.Attempt?.UserName || (report.Origin === 'Probe' ? 'The probe' : 'Playback')}</div>
                  <div class="line small muted">{since(report.CreatedAt)}</div>
                  {client(report.Attempt) && (
                    <div class="line small muted" title={client(report.Attempt)}>
                      {client(report.Attempt)}
                    </div>
                  )}
                </td>
                <td>
                  <div class="line">{report.Kind}</div>
                  <div class="line small muted">{report.Note || (report.Origin === 'Delete' ? 'Delete was pressed in a client' : ' ')}</div>
                  <Served how={attempted(report.Attempt)} />
                </td>
                <td>
                  <Pill tone={reportTones[report.Status]}>{report.Status}</Pill>
                </td>
                <td>
                  <div class="actions right">
                    {report.Status === 'Open' && (
                      <>
                        <button onClick={() => close(report, 'Dismiss')}>Dismiss</button>
                        <button onClick={() => close(report, 'Resolve')}>Resolve</button>
                        <button class="primary" disabled={!report.Replaceable} title={report.Replaceable ? '' : 'This file has no downloader record yet. Refresh the library first.'} onClick={() => close(report, 'Replace')}>
                          Replace
                        </button>
                      </>
                    )}
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      <p class="muted small">
        Replace deletes the file in the downloader, blocklists the release it came from and searches for another. Resolve and Dismiss only close the
        report.
      </p>
    </>
  );
}
