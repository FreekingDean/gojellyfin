import * as mock from '../mock';
import { Link } from '../router';
import { Card, Dot, Page, Pill, since } from '../ui';
import type { Tone } from '../ui';

const methods: Record<mock.Watching['method'], Tone> = { Direct: 'ok', Remux: 'off', 'Encoding 1080p': 'warn' };

export function Overview() {
  const waiting = mock.titles.filter((title) => title.Requests.some((entry) => entry.Status === 'Pending')).length;
  const open = mock.reports.filter((report) => report.Status === 'Open').length;
  const troubled = mock.health.filter((item) => item.tone === 'warn' || item.tone === 'bad');

  return (
    <Page
      title="Overview"
      lead={troubled.length === 0 ? 'Everything is running.' : 'Everything is running. ' + troubled.map((item) => item.name).join(', ') + ' needs attention.'}
    >
      <div class="grid overview">
        <Card title="Health" class="wide">
          <div class="health">
            {mock.health.map((item) => (
              <div key={item.name}>
                <Dot tone={item.tone} />
                {item.name}
                <small>{item.detail}</small>
              </div>
            ))}
          </div>
        </Card>

        <Card title="Watching now">
          <table class="rows plain">
            <tbody>
              {mock.watching.map((stream) => (
                <tr key={stream.user}>
                  <td>
                    <b>{stream.user}</b> <span class="muted">{stream.device}</span>
                  </td>
                  <td>
                    {stream.title}
                    <div class="bar">
                      <span style={{ width: stream.progress + '%' }} />
                    </div>
                  </td>
                  <td class="end">
                    <Pill tone={methods[stream.method]}>{stream.method}</Pill>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </Card>

        <Card title="Waiting on you">
          <Link to="/requests" class="stat">
            <span class="big">{waiting}</span>
            <span class="muted">requests awaiting approval</span>
          </Link>
          <Link to="/requests" class="stat">
            <span class="big">{open}</span>
            <span class="muted">open reports</span>
          </Link>
        </Card>

        <Card title="Library">
          <div class="stat">
            <span class="big">{mock.library.movies.toLocaleString()}</span>
            <span class="muted">
              movies, and {mock.library.shows} shows with {mock.library.episodes.toLocaleString()} episodes
            </span>
          </div>
          <dl class="fields tight">
            <dt>Last refresh</dt>
            <dd>{since(mock.library.refreshed)}</dd>
            <dt>Unprobed files</dt>
            <dd>{mock.library.unprobed}</dd>
            <dt>Unidentified</dt>
            <dd>{mock.library.unidentified}</dd>
          </dl>
        </Card>

        <Card title="Recent activity" class="wide">
          <table class="rows plain">
            <tbody>
              {mock.activity.slice(0, 5).map((entry) => (
                <tr key={entry.at}>
                  <td class="when muted">{since(entry.at)}</td>
                  <td>
                    {entry.tone && <Dot tone={entry.tone} />}
                    {entry.text}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </Card>
      </div>
    </Page>
  );
}
