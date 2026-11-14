import { useState } from 'preact/hooks';

import * as mock from '../mock';
import { Cell, Dot, Page, Pill, Tabs, since } from '../ui';
import type { Tone } from '../ui';

type Tab = 'sessions' | 'devices' | 'log';

const methods: Record<mock.Watching['method'], Tone> = { Direct: 'ok', Remux: 'off', 'Encoding 1080p': 'warn' };

export function Activity() {
  const [tab, setTab] = useState<Tab>('sessions');
  const devices = mock.people.flatMap((person) => person.devices.map((device) => ({ ...device, user: person.name })));

  return (
    <Page title="Activity" lead="Who is connected, what they are signed in on, and what has happened.">
      <Tabs
        tabs={[
          { id: 'sessions', label: 'Sessions', count: mock.sessions.filter((session) => session.playing).length },
          { id: 'devices', label: 'Devices' },
          { id: 'log', label: 'Log' },
        ]}
        selected={tab}
        select={setTab}
      />

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
              {mock.sessions.map((session) => (
                <tr key={session.user}>
                  <Cell main={session.user} sub={session.device + ', ' + session.app} />
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
                    <Cell main={<span class="muted">Nothing</span>} sub={'Last active ' + since(session.seen)} />
                  )}
                  <td>{session.playing && <Pill tone={methods[session.playing.method]}>{session.playing.method}</Pill>}</td>
                  <td class="muted">{session.address}</td>
                  <td>
                    <div class="actions right">
                      <button>Sign out</button>
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
                <tr key={device.user + device.name}>
                  <Cell main={device.name} sub={device.app} />
                  <td>{device.user}</td>
                  <td class="muted">{since(device.seen)}</td>
                  <td>
                    <div class="actions right">
                      <button>Sign out</button>
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
              {mock.activity.map((entry) => (
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
        </div>
      )}
    </Page>
  );
}
