import { useEffect, useState } from 'preact/hooks';

import { api } from '../api';
import * as mock from '../mock';
import { Served, how } from '../playback';
import type { How } from '../playback';
import { Link } from '../router';
import { Card, Dot, Page, since } from '../ui';
import type { Tone } from '../ui';

type Check = { name: string; tone: Tone; detail: string; at?: string };
type Watching = { id: string; user: string; device: string; title: string; how?: How; progress: number };
type Entry = { at: string; text: string; tone?: Tone };
type Library = { movies: number; shows: number; episodes: number; refreshed?: string; unprobed: number; unidentified: number };
type Loaded = { checks: Check[]; watching: Watching[]; waiting: number; open: number; library: Library; activity: Entry[] };

const tones: Record<string, Tone> = { Ok: 'ok', Warn: 'warn', Bad: 'bad', Off: 'off' };
const severities: Record<string, Tone> = { Warning: 'warn', Error: 'bad', Critical: 'bad' };

function mocked(): Loaded {
  return {
    checks: mock.health,
    watching: mock.watching.map((stream) => ({ ...stream, id: stream.user })),
    waiting: mock.titles.filter((title) => title.Requests.some((entry) => entry.Status === 'Pending')).length,
    open: mock.reports.filter((report) => report.Status === 'Open').length,
    library: mock.library,
    activity: mock.activity,
  };
}

async function fetched(): Promise<Loaded | undefined> {
  const [health, sessions, requested, reported, log] = await Promise.all([
    api.GET('/GoJellyfin/Health'),
    api.GET('/Sessions'),
    api.GET('/GoJellyfin/Requests'),
    api.GET('/GoJellyfin/Reports'),
    api.GET('/System/ActivityLog/Entries', { params: { query: { limit: 5 } } }),
  ]);
  if (!health.data || !sessions.data || !requested.data || !reported.data || !log.data) {
    return undefined;
  }
  const library = health.data.Library;

  return {
    checks: health.data.Checks.map((check) => ({
      name: check.Name,
      tone: tones[check.Status] ?? 'off',
      detail: check.Detail,
      at: check.At ?? undefined,
    })),
    watching: sessions.data.flatMap((session) => {
      const item = session.NowPlayingItem;
      if (!item) {
        return [];
      }
      const length = item.RunTimeTicks ?? 0;

      return [
        {
          id: session.Id ?? '',
          user: session.UserName ?? '',
          device: session.DeviceName ?? '',
          title: [item.SeriesName, item.Name].filter(Boolean).join(', '),
          how: how(session),
          progress: length > 0 ? Math.min(100, ((session.PlayState?.PositionTicks ?? 0) / length) * 100) : 0,
        },
      ];
    }),
    waiting: requested.data.filter((title) => title.Requests.some((entry) => entry.Status === 'Pending')).length,
    open: reported.data.filter((report) => report.Status === 'Open').length,
    library: {
      movies: library.Movies,
      shows: library.Shows,
      episodes: library.Episodes,
      refreshed: library.RefreshedAt ?? undefined,
      unprobed: library.Unprobed,
      unidentified: library.Unidentified,
    },
    activity: (log.data.Items ?? []).map((entry) => ({
      at: entry.Date ?? '',
      text: [entry.Name, entry.ShortOverview].filter(Boolean).join(', '),
      tone: severities[entry.Severity ?? ''],
    })),
  };
}

export function Overview() {
  const [loaded, setLoaded] = useState<Loaded | undefined>(mock.mocked ? mocked() : undefined);
  const [failure, setFailure] = useState('');

  useEffect(() => {
    if (mock.mocked) {
      return;
    }
    void (async () => {
      const answer = await fetched();
      if (!answer) {
        setFailure('The overview could not be loaded.');
        return;
      }
      setLoaded(answer);
    })();
  }, []);

  if (!loaded) {
    return (
      <Page title="Overview" lead="How the server is doing.">
        <p class={failure ? 'failure' : 'muted'}>{failure || 'Loading…'}</p>
      </Page>
    );
  }

  const { checks, watching, waiting, open, library, activity } = loaded;
  const troubled = checks.filter((check) => check.tone === 'warn' || check.tone === 'bad');
  const broken = checks.some((check) => check.tone === 'bad');

  return (
    <Page
      title="Overview"
      lead={
        troubled.length === 0
          ? 'Everything is running.'
          : (broken ? 'Something is not running. ' : 'Everything is running. ') + troubled.map((check) => check.name).join(', ') + ' needs attention.'
      }
    >
      <div class="grid overview">
        <Card title="Health" class="wide">
          <div class="health">
            {checks.map((check) => (
              <div key={check.name}>
                <Dot tone={check.tone} />
                {check.name}
                <small>
                  {check.detail}
                  {check.at && ', ' + since(check.at)}
                </small>
              </div>
            ))}
          </div>
        </Card>

        <Card title="Watching now">
          {watching.length === 0 ? (
            <p class="muted">Nobody is watching anything.</p>
          ) : (
            <table class="rows plain">
              <tbody>
                {watching.map((stream) => (
                  <tr key={stream.id}>
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
                      <Served how={stream.how} />
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
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
            <span class="big">{library.movies.toLocaleString()}</span>
            <span class="muted">
              movies, and {library.shows.toLocaleString()} shows with {library.episodes.toLocaleString()} episodes
            </span>
          </div>
          <dl class="fields tight">
            <dt>Last refresh</dt>
            <dd>{library.refreshed ? since(library.refreshed) : 'never'}</dd>
            <dt>Unprobed files</dt>
            <dd>{library.unprobed.toLocaleString()}</dd>
            <dt>Unidentified</dt>
            <dd>{library.unidentified.toLocaleString()}</dd>
          </dl>
        </Card>

        <Card title="Recent activity" class="wide">
          <table class="rows plain">
            <tbody>
              {activity.map((entry, i) => (
                <tr key={entry.at + i}>
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
