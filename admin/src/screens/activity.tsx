import { useEffect, useState } from 'preact/hooks';

import { api } from '../api';
import * as mock from '../mock';
import { Cell, Dot, Page, Pill, Tabs, since } from '../ui';
import type { Tone } from '../ui';

type Tab = 'sessions' | 'devices' | 'log';
type Playing = { title: string; progress: number; method: string };
type Session = { id: string; device: string; user: string; where: string; app: string; address?: string; seen?: string; playing?: Playing };
type Device = { id: string; name: string; app: string; user: string; seen?: string };
type Entry = { at: string; text: string; tone?: Tone };
type Loaded = { sessions: Session[]; devices: Device[]; log: Entry[] };

const methods: Record<string, Tone> = { Direct: 'ok', DirectPlay: 'ok', Remux: 'off', DirectStream: 'off', Transcode: 'warn', 'Encoding 1080p': 'warn' };
const severities: Record<string, Tone> = { Warning: 'warn', Error: 'bad', Critical: 'bad' };

function mocked(): Loaded {
  return {
    sessions: mock.sessions.map((session) => ({ ...session, id: session.user, where: session.device })),
    devices: mock.people.flatMap((person) => person.devices.map((device) => ({ ...device, id: person.name + device.name, user: person.name }))),
    log: mock.activity,
  };
}

async function fetched(): Promise<Loaded | undefined> {
  const [sessions, devices, log] = await Promise.all([
    api.GET('/Sessions'),
    api.GET('/Devices'),
    api.GET('/System/ActivityLog/Entries', { params: { query: { limit: 100 } } }),
  ]);
  if (!sessions.data || !devices.data || !log.data) {
    return undefined;
  }

  return {
    sessions: sessions.data.map((session) => {
      const item = session.NowPlayingItem;
      const length = item?.RunTimeTicks ?? 0;

      return {
        id: session.Id ?? '',
        device: session.DeviceId ?? '',
        user: session.UserName ?? 'Nobody',
        where: session.DeviceName ?? '',
        app: [session.Client, session.ApplicationVersion].filter(Boolean).join(' '),
        address: session.RemoteEndPoint ?? undefined,
        seen: session.LastActivityDate,
        playing: item
          ? {
              title: [item.SeriesName, item.Name].filter(Boolean).join(', '),
              progress: length > 0 ? Math.min(100, ((session.PlayState?.PositionTicks ?? 0) / length) * 100) : 0,
              method: session.PlayState?.PlayMethod ?? '',
            }
          : undefined,
      };
    }),
    devices: (devices.data.Items ?? []).map((device) => ({
      id: device.Id ?? '',
      name: device.CustomName || device.Name || 'Unnamed device',
      app: [device.AppName, device.AppVersion].filter(Boolean).join(' '),
      user: device.LastUserName ?? '',
      seen: device.DateLastActivity ?? undefined,
    })),
    log: (log.data.Items ?? []).map((entry) => ({
      at: entry.Date ?? '',
      text: [entry.Name, entry.ShortOverview].filter(Boolean).join(', '),
      tone: severities[entry.Severity ?? ''],
    })),
  };
}

export function Activity() {
  const [tab, setTab] = useState<Tab>('sessions');
  const [loaded, setLoaded] = useState<Loaded | undefined>(mock.mocked ? mocked() : undefined);
  const [failure, setFailure] = useState('');

  async function load() {
    if (mock.mocked) {
      return;
    }
    const answer = await fetched();
    if (!answer) {
      setFailure('The activity could not be loaded.');
      return;
    }
    setLoaded(answer);
  }

  async function signOut(device: string) {
    if (mock.mocked) {
      return;
    }
    const { response } = await api.DELETE('/Devices', { params: { query: { id: [device] } } });
    setFailure(response.ok ? '' : 'The device could not be signed out.');
    await load();
  }

  useEffect(() => {
    void load();
  }, []);

  if (!loaded) {
    return (
      <Page title="Activity" lead="Who is connected, what they are signed in on, and what has happened.">
        <p class={failure ? 'failure' : 'muted'}>{failure || 'Loading…'}</p>
      </Page>
    );
  }
  const { sessions, devices, log } = loaded;

  return (
    <Page title="Activity" lead="Who is connected, what they are signed in on, and what has happened.">
      <Tabs
        tabs={[
          { id: 'sessions', label: 'Sessions', count: sessions.filter((session) => session.playing).length },
          { id: 'devices', label: 'Devices' },
          { id: 'log', label: 'Log' },
        ]}
        selected={tab}
        select={setTab}
      />

      {failure && (
        <p class="failure" role="alert">
          {failure}
        </p>
      )}

      {tab === 'sessions' && (
        <div class="card flush">
          <table class="rows">
            <colgroup>
              <col style="width:22%" />
              <col style="width:34%" />
              <col style="width:16%" />
              <col style="width:16%" />
              <col style="width:12%" />
            </colgroup>
            <thead>
              <tr>
                <th>User</th>
                <th>Playing</th>
                <th>How</th>
                <th>Address</th>
                <th />
              </tr>
            </thead>
            <tbody>
              {sessions.map((session) => (
                <tr key={session.id}>
                  <Cell main={session.user} sub={[session.where, session.app].filter(Boolean).join(', ')} />
                  {session.playing ? (
                    <Cell
                      main={session.playing.title}
                      sub={
                        <div class="bar">
                          <span style={{ width: session.playing.progress + '%' }} />
                        </div>
                      }
                    />
                  ) : (
                    <Cell main={<span class="muted">Nothing</span>} sub={session.seen && 'Last active ' + since(session.seen)} />
                  )}
                  <td>{session.playing?.method && <Pill tone={methods[session.playing.method]}>{session.playing.method}</Pill>}</td>
                  <td class="muted">{session.address}</td>
                  <td>
                    <div class="actions right">
                      <button onClick={() => signOut(session.device)}>Sign out</button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {tab === 'devices' && (
        <div class="card flush">
          <table class="rows">
            <colgroup>
              <col style="width:30%" />
              <col style="width:22%" />
              <col style="width:28%" />
              <col style="width:20%" />
            </colgroup>
            <thead>
              <tr>
                <th>Device</th>
                <th>User</th>
                <th>Last used</th>
                <th />
              </tr>
            </thead>
            <tbody>
              {devices.map((device) => (
                <tr key={device.id}>
                  <Cell main={device.name} sub={device.app} />
                  <td>{device.user}</td>
                  <td class="muted">{device.seen ? since(device.seen) : ''}</td>
                  <td>
                    <div class="actions right">
                      <button onClick={() => signOut(device.id)}>Sign out</button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {tab === 'log' && (
        <div class="card flush">
          <table class="rows plain">
            <tbody>
              {log.map((entry) => (
                <tr key={entry.at + entry.text}>
                  <td class="when muted">{since(entry.at)}</td>
                  <td>
                    {entry.tone && <Dot tone={entry.tone} />}
                    {entry.text}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </Page>
  );
}
