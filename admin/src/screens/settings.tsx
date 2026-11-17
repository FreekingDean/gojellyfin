import { useEffect, useState } from 'preact/hooks';

import { api } from '../api';
import type { Schemas } from '../api';
import * as mock from '../mock';
import { Card, Cell, Page, Pill, notify, refused, since } from '../ui';

type Variable = Schemas['Setting'];
type Shelf = { slug: string; name: string; shows: string; on: boolean; note?: string };
type Layout = { title: string; gate?: string; why?: string; settings: { label: string; variable: string }[] };

const layout: Layout[] = [
  {
    title: 'Server',
    settings: [
      { label: 'Published address', variable: 'PUBLISHED_SERVER_URL' },
      { label: 'Server id', variable: 'SERVER_ID' },
      { label: 'Port', variable: 'HTTP_PORT' },
      { label: 'Media folders', variable: 'MEDIA_DIRECTORIES' },
      { label: 'Database', variable: 'DATABASE_URL' },
      { label: 'Origins allowed to call the API', variable: 'CORS_ORIGINS' },
    ],
  },
  {
    title: 'Single sign-on',
    gate: 'OIDC_ISSUER_URL',
    why: 'Set OIDC_ISSUER_URL, OIDC_CLIENT_ID and PUBLISHED_SERVER_URL to let people sign in through an identity provider.',
    settings: [
      { label: 'Provider', variable: 'OIDC_ISSUER_URL' },
      { label: 'Client id', variable: 'OIDC_CLIENT_ID' },
      { label: 'Client secret', variable: 'OIDC_CLIENT_SECRET' },
      { label: 'Scopes', variable: 'OIDC_SCOPES' },
      { label: 'Username claim', variable: 'OIDC_USERNAME_CLAIM' },
      { label: 'Groups claim', variable: 'OIDC_GROUPS_CLAIM' },
      { label: 'Allowed group', variable: 'OIDC_ALLOWED_GROUP' },
      { label: 'Administrator group', variable: 'OIDC_ADMIN_GROUP' },
      { label: 'Link existing local users', variable: 'OIDC_LINK_EXISTING_USERS' },
    ],
  },
  {
    title: 'Metadata',
    gate: 'TMDB_API_KEY',
    why: 'Set TMDB_API_KEY to identify titles, fetch artwork and offer titles the library lacks.',
    settings: [
      { label: 'TMDB key', variable: 'TMDB_API_KEY' },
      { label: 'TMDB address', variable: 'TMDB_BASE_URL' },
    ],
  },
  {
    title: 'Transcoding',
    settings: [
      { label: 'Encodes at once, per pod', variable: 'TRANSCODER_JOBS' },
      { label: 'Stalled client timeout', variable: 'TRANSCODER_STALL_TIMEOUT' },
    ],
  },
  {
    title: 'Background work',
    gate: 'TEMPORAL_HOSTPORT',
    why: 'Set TEMPORAL_HOSTPORT and TEMPORAL_NAMESPACE to run library refreshes, probes and metadata in a worker.',
    settings: [
      { label: 'Temporal address', variable: 'TEMPORAL_HOSTPORT' },
      { label: 'Namespace', variable: 'TEMPORAL_NAMESPACE' },
      { label: 'Worker drain time', variable: 'TEMPORAL_WORKER_STOP_TIMEOUT' },
    ],
  },
  {
    title: 'Tracing',
    gate: 'OTEL_EXPORTER_OTLP_ENDPOINT',
    why: 'Set OTEL_EXPORTER_OTLP_ENDPOINT to send a span per operation to a collector.',
    settings: [{ label: 'Collector', variable: 'OTEL_EXPORTER_OTLP_ENDPOINT' }],
  },
];

function shown(variable: Variable): string {
  if (variable.Secret) {
    return variable.Set ? 'Set' : 'Not set';
  }

  return variable.Set ? variable.Value : 'Not set';
}

function grouped(variables: Variable[]): mock.SettingGroup[] {
  const named = new Map(variables.map((variable) => [variable.Name, variable]));
  const placed = new Set(layout.flatMap((group) => group.settings.map((setting) => setting.variable)));
  const rest = variables.filter((variable) => !placed.has(variable.Name));

  const groups = layout.map((group): mock.SettingGroup => {
    const on = !group.gate || Boolean(named.get(group.gate)?.Set);

    return {
      title: group.title,
      state: group.gate ? (on ? 'On' : 'Off') : '',
      why: on ? undefined : group.why,
      settings: on
        ? group.settings.flatMap((setting) => {
            const variable = named.get(setting.variable);

            return variable ? [{ label: setting.label, value: shown(variable), variable: variable.Name }] : [];
          })
        : [],
    };
  });
  if (rest.length > 0) {
    groups.push({
      title: 'Downloader keys',
      state: '',
      settings: rest.map((variable) => ({ label: variable.Name.replace('SOURCE_API_KEY_', ''), value: shown(variable), variable: variable.Name })),
    });
  }

  return groups;
}

function described(slug: string): { shows: string; note?: string } {
  const shelf = mock.shelves.find((each) => each.slug === slug);

  return { shows: shelf?.shows ?? '', note: shelf?.note };
}

export function Settings() {
  const [shelves, setShelves] = useState<Shelf[] | undefined>(mock.mocked ? mock.shelves : undefined);
  const [groups, setGroups] = useState<mock.SettingGroup[]>(mock.mocked ? mock.settings : []);
  const [failure, setFailure] = useState('');

  async function load() {
    if (mock.mocked) {
      return;
    }
    const [choices, variables] = await Promise.all([api.GET('/GoJellyfin/Shelves'), api.GET('/GoJellyfin/Settings')]);
    if (!choices.data || !variables.data) {
      setFailure('The settings could not be loaded.');
      return;
    }
    setShelves(choices.data.map((choice) => ({ slug: choice.Slug, name: choice.Name, on: choice.Enabled, ...described(choice.Slug) })));
    setGroups(grouped(variables.data));
  }

  function toggle(slug: string, on: boolean) {
    setShelves(shelves?.map((shelf) => (shelf.slug === slug ? { ...shelf, on } : shelf)));
  }

  async function save() {
    if (mock.mocked || !shelves) {
      return;
    }
    const { response, error } = await api.PUT('/GoJellyfin/Shelves', { body: { Enabled: shelves.filter((shelf) => shelf.on).map((shelf) => shelf.slug) } });
    if (response.ok) {
      notify('Shelves saved. They show on the next visit to the home screen.');
    } else {
      refused(error, 'The shelves could not be saved.');
    }
    await load();
  }

  useEffect(() => {
    void load();
  }, []);

  if (!shelves) {
    return (
      <Page title="Settings" lead="Shelves, branding and API keys are edited here. Everything else is set in the environment, so it is shown here and changed in your deployment.">
        <p class={failure ? 'failure' : 'muted'}>{failure || 'Loading…'}</p>
      </Page>
    );
  }

  return (
    <Page
      title="Settings"
      lead={
        'Shelves, branding and API keys are edited here. Everything else is set in the environment, so it is shown here and changed in your deployment.'
      }
    >
      <Card class="shelves">
        <header class="cardhead">
          <b>Home shelves</b>
          <button class="primary" onClick={save}>
            Save shelves
          </button>
        </header>
        <p class="muted small">Extra rows on everyone's home screen. Each one that is on also appears as a tile under My Media.</p>
        <div class="shelfgrid">
          {shelves.map((shelf) => (
            <label class="check shelf" key={shelf.slug}>
              <input type="checkbox" checked={shelf.on} onChange={(event) => toggle(shelf.slug, event.currentTarget.checked)} />
              <span>
                <span class="line">
                  {shelf.name}
                  {shelf.note && <Pill>{shelf.note}</Pill>}
                </span>
                <span class="line small muted">{shelf.shows}</span>
              </span>
            </label>
          ))}
        </div>
      </Card>

      <Override />

      <div class="grid two spaced">
        {groups.map((group) => (
          <Card key={group.title}>
            <header class="cardhead">
              <b>{group.title}</b>
              {group.state && <Pill tone={group.state === 'On' ? 'ok' : 'off'}>{group.state}</Pill>}
            </header>
            {group.why && <p class="muted">{group.why}</p>}
            {group.settings.length > 0 && (
              <table class="rows plain settings">
                <tbody>
                  {group.settings.map((setting) => (
                    <tr key={setting.label}>
                      <td class="muted">{setting.label}</td>
                      <td>
                        <div class="line">{setting.value}</div>
                        <code class="small">{setting.variable}</code>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </Card>
        ))}

        <Branding />
        <Keys />
      </div>
    </Page>
  );
}

type Standing = Schemas['PlaybackOverride'];
type Person = { id: string; name: string };
type Found = { id: string; name: string; detail: string };

const mockedOverride: Standing = { Enabled: true, ItemId: 'm1', ItemName: 'An Invented Feature', From: '2027-04-01T00:00:00Z', Until: '2027-04-02T00:00:00Z', UserIds: [], StartTicks: 0 };

const ticksPerSecond = 10_000_000;

function clock(ticks: number | undefined): string {
  const seconds = Math.floor((ticks ?? 0) / ticksPerSecond);
  if (seconds <= 0) {
    return '';
  }
  const pad = (value: number) => String(value).padStart(2, '0');
  const hours = Math.floor(seconds / 3600);
  const minutes = Math.floor((seconds % 3600) / 60);

  return (hours > 0 ? hours + ':' + pad(minutes) : String(minutes)) + ':' + pad(seconds % 60);
}

function ticksOf(text: string): number | undefined {
  const trimmed = text.trim();
  if (trimmed === '') {
    return 0;
  }
  const parts = trimmed.split(':');
  if (parts.length > 3 || parts.some((part) => !/^\d+$/.test(part))) {
    return undefined;
  }

  return parts.reduce((total, part) => total * 60 + Number(part), 0) * ticksPerSecond;
}

function local(stamp: string | null | undefined): string {
  if (!stamp) {
    return '';
  }
  const at = new Date(stamp);
  at.setMinutes(at.getMinutes() - at.getTimezoneOffset());

  return at.toISOString().slice(0, 16);
}

function stamped(value: string): string | undefined {
  return value ? new Date(value).toISOString() : undefined;
}

function Override() {
  const [standing, setStanding] = useState<Standing | undefined>(mock.mocked ? mockedOverride : undefined);
  const [people, setPeople] = useState<Person[]>(mock.mocked ? mock.people.map((each) => ({ id: each.id, name: each.name })) : []);
  const [search, setSearch] = useState('');
  const [found, setFound] = useState<Found[]>([]);
  const [failure, setFailure] = useState('');
  const [start, setStart] = useState(clock(mock.mocked ? mockedOverride.StartTicks : 0));

  async function load() {
    const [override, users] = await Promise.all([api.GET('/GoJellyfin/PlaybackOverride'), api.GET('/Users')]);
    if (!override.data || !users.data) {
      setFailure('The playback override could not be loaded.');
      return;
    }
    setStanding(override.data);
    setStart(clock(override.data.StartTicks));
    setPeople(users.data.map((user) => ({ id: user.Id ?? '', name: user.Name ?? '' })));
  }

  useEffect(() => {
    if (!mock.mocked) {
      void load();
    }
  }, []);

  useEffect(() => {
    const term = search.trim();
    if (term.length < 2 || mock.mocked) {
      setFound([]);
      return;
    }
    const timer = window.setTimeout(async () => {
      const { data } = await api.GET('/Items', {
        params: { query: { searchTerm: term, recursive: true, includeItemTypes: ['Movie', 'Episode'], limit: 10 } },
      });
      setFound(
        (data?.Items ?? []).map((item) => ({
          id: item.Id ?? '',
          name: item.Name ?? '',
          detail: item.Type === 'Episode' ? [item.SeriesName, item.ParentIndexNumber && item.IndexNumber ? 'S' + item.ParentIndexNumber + 'E' + item.IndexNumber : ''].filter(Boolean).join(' ') : String(item.ProductionYear ?? ''),
        })),
      );
    }, 250);

    return () => window.clearTimeout(timer);
  }, [search]);

  if (!standing) {
    return <Card title="Playback override">{failure ? <p class="failure">{failure}</p> : <p class="muted">Loading…</p>}</Card>;
  }

  function change(patch: Partial<Standing>) {
    setStanding({ ...(standing as Standing), ...patch });
  }

  function pick(each: Found) {
    change({ ItemId: each.id, ItemName: each.name });
    setSearch('');
  }

  function choose(id: string, on: boolean) {
    const chosen = (standing as Standing).UserIds.filter((each) => each !== id);
    change({ UserIds: on ? [...chosen, id] : chosen });
  }

  async function save(next: Standing) {
    if (mock.mocked) {
      setStanding(next);
      return;
    }
    const startTicks = ticksOf(start);
    if (startTicks === undefined) {
      setFailure('Write the start as minutes and seconds, like 12:30, or hours too, like 1:02:03.');
      return;
    }
    setFailure('');
    const body = { Enabled: next.Enabled, ItemId: next.ItemId, From: next.From, Until: next.Until, UserIds: next.UserIds, StartTicks: startTicks };
    const { response, error } = await api.PUT('/GoJellyfin/PlaybackOverride', { body });
    if (response.ok) {
      notify(next.Enabled ? 'The override is on. Plays started from now on are swapped while the window is open.' : 'The override is off. Plays go back to what was picked from the next request.');
    } else {
      refused(error, 'The playback override could not be saved.');
    }
    await load();
  }

  return (
    <Card class="override">
      <header class="cardhead">
        <span class="line">
          <b>Playback override</b> <Pill tone={standing.Enabled ? 'ok' : 'off'}>{standing.Enabled ? 'On' : 'Off'}</Pill>
        </span>
        <span class="actions">
          {standing.Enabled && <button onClick={() => save({ ...standing, Enabled: false })}>Turn off now</button>}
          <button class="primary" onClick={() => save(standing)}>
            Save override
          </button>
        </span>
      </header>
      <p class="muted small">
        Whatever movie or episode someone plays, this title plays instead, administrators included, so pick yourself alone to try it first. Music and channels play as they are, and nothing is marked watched or resumed on either title while it is swapped.
      </p>
      {failure && (
        <p class="failure" role="alert">
          {failure}
        </p>
      )}
      <div class="form">
        <label class="check">
          <input type="checkbox" checked={standing.Enabled} onChange={(event) => change({ Enabled: event.currentTarget.checked })} />
          <span>On</span>
        </label>
        <label>
          Plays instead
          <input placeholder="Search for a movie or an episode" value={search} onInput={(event) => setSearch(event.currentTarget.value)} />
          <span class="line small">{standing.ItemName ? standing.ItemName : <span class="muted">Nothing picked yet</span>}</span>
        </label>
        {found.length > 0 && (
          <div class="picker">
            <ul>
              {found.map((each) => (
                <li key={each.id}>
                  <button onClick={() => pick(each)}>
                    {each.name} <span class="muted small">{each.detail}</span>
                  </button>
                </li>
              ))}
            </ul>
          </div>
        )}
        <label>
          Starts at
          <input placeholder="From the beginning" value={start} onInput={(event) => setStart(event.currentTarget.value)} />
          <span class="line small muted">How far into the title it begins, like 12:30 or 1:02:03.</span>
        </label>
        <label>
          From
          <input type="datetime-local" value={local(standing.From)} onInput={(event) => change({ From: stamped(event.currentTarget.value) })} />
        </label>
        <label>
          Until
          <input type="datetime-local" value={local(standing.Until)} onInput={(event) => change({ Until: stamped(event.currentTarget.value) })} />
        </label>
      </div>
      <p class="muted small">Times are in this browser's zone. Leave both empty to run for as long as it is on.</p>
      <fieldset class="people">
        <legend>Who {standing.UserIds.length === 0 ? '(everyone)' : ''}</legend>
        {people.map((person) => (
          <label class="check" key={person.id}>
            <input type="checkbox" checked={standing.UserIds.includes(person.id)} onChange={(event) => choose(person.id, event.currentTarget.checked)} />
            <span>{person.name}</span>
          </label>
        ))}
      </fieldset>
    </Card>
  );
}

type Stored = { LoginDisclaimer?: string; CustomCss?: string; SplashscreenEnabled?: boolean } & Record<string, unknown>;

function Branding() {
  const [stored, setStored] = useState<Stored | undefined>(mock.mocked ? { LoginDisclaimer: mock.branding.disclaimer, CustomCss: mock.branding.css } : undefined);
  const [failure, setFailure] = useState('');

  useEffect(() => {
    if (mock.mocked) {
      return;
    }
    void (async () => {
      const { data } = await api.GET('/System/Configuration/{key}', { params: { path: { key: 'branding' } } });
      if (!data) {
        setFailure('The branding could not be loaded.');
        return;
      }
      setStored(data as unknown as Stored);
    })();
  }, []);

  function edit(field: 'LoginDisclaimer' | 'CustomCss') {
    return (event: Event) => {
      setStored({ ...stored, [field]: (event.currentTarget as HTMLTextAreaElement).value });
    };
  }

  async function save() {
    if (mock.mocked || !stored) {
      return;
    }
    const { response, error } = await api.POST('/System/Configuration/{key}', { params: { path: { key: 'branding' } }, body: stored });
    if (response.ok) {
      notify('Branding saved. Clients pick it up the next time they load.');
    } else {
      refused(error, 'The branding could not be saved.');
    }
  }

  return (
    <Card title="Branding">
      {failure && (
        <p class="failure" role="alert">
          {failure}
        </p>
      )}
      {stored && (
        <>
          <div class="form one">
            <label>
              Sign-in message
              <textarea rows={2} value={stored.LoginDisclaimer ?? ''} onInput={edit('LoginDisclaimer')} />
            </label>
            <label>
              Custom CSS for the web client
              <textarea rows={3} class="mono" value={stored.CustomCss ?? ''} onInput={edit('CustomCss')} />
            </label>
          </div>
          <p class="muted small">The single sign-on button and the home shelves' titles are added on the way out, so they are not part of what is saved here.</p>
          <div class="actions spaced">
            <button class="primary" onClick={save}>
              Save branding
            </button>
          </div>
        </>
      )}
    </Card>
  );
}

type Key = { token: string; name: string; created: string };

function Keys() {
  const [keys, setKeys] = useState<Key[] | undefined>(
    mock.mocked ? mock.apiKeys.map((key, i) => ({ token: 'mock' + i, name: key.name, created: key.created })) : undefined,
  );
  const [made, setMade] = useState<Key | undefined>();
  const [naming, setNaming] = useState<string | undefined>();
  const [failure, setFailure] = useState('');

  async function load(): Promise<Key[]> {
    const { data } = await api.GET('/Auth/Keys');
    if (!data) {
      setFailure('The API keys could not be loaded.');
      return [];
    }
    const active = (data.Items ?? [])
      .filter((key) => key.IsActive !== false && key.AccessToken)
      .map((key) => ({ token: key.AccessToken ?? '', name: key.AppName ?? '', created: key.DateCreated ?? '' }));
    setKeys(active);

    return active;
  }

  useEffect(() => {
    if (!mock.mocked) {
      void load();
    }
  }, []);

  async function create() {
    const name = naming?.trim();
    if (mock.mocked || !name) {
      return;
    }
    const before = new Set(keys?.map((key) => key.token));
    const { response, error } = await api.POST('/Auth/Keys', { params: { query: { app: name } } });
    if (!response.ok) {
      refused(error, 'The key for ' + name + ' could not be made.');
      return;
    }
    setNaming(undefined);
    setMade((await load()).find((key) => !before.has(key.token)));
  }

  async function revoke(key: Key) {
    if (mock.mocked) {
      return;
    }
    const { response, error } = await api.DELETE('/Auth/Keys/{key}', { params: { path: { key: key.token } } });
    if (response.ok) {
      notify('The key for ' + key.name + ' was revoked.');
    } else {
      refused(error, 'The key for ' + key.name + ' could not be revoked.');
    }
    if (made?.token === key.token) {
      setMade(undefined);
    }
    await load();
  }

  return (
    <Card>
      <header class="cardhead">
        <b>API keys</b>
        {naming === undefined && <button onClick={() => setNaming('')}>New key</button>}
      </header>
      <p class="muted small">A key is for an app such as Home Assistant, and is shown in full once, when it is made. It signs in as the administrator who made it, and stops working if that account is demoted, disabled or deleted.</p>
      {failure && (
        <p class="failure" role="alert">
          {failure}
        </p>
      )}
      {naming !== undefined && (
        <div class="form one">
          <label>
            Which app is it for
            <input value={naming} onInput={(event) => setNaming(event.currentTarget.value)} />
          </label>
          <div class="actions">
            <button class="primary" onClick={create} disabled={!naming.trim()}>
              Make key
            </button>
            <button onClick={() => setNaming(undefined)}>Cancel</button>
          </div>
        </div>
      )}
      {made && (
        <p role="status">
          The key for {made.name} is <code class="mono">{made.token}</code>
        </p>
      )}
      {keys && keys.length === 0 && <p class="muted">No app has a key.</p>}
      {keys && keys.length > 0 && (
        <table class="rows plain">
          <tbody>
            {keys.map((key) => (
              <tr key={key.token}>
                <Cell main={key.name} sub={key.created ? 'Made ' + since(key.created) : undefined} />
                <td class="muted">
                  <code class="small">{key.token.slice(0, 6)}…</code>
                </td>
                <td class="end">
                  <button onClick={() => revoke(key)}>Revoke</button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </Card>
  );
}
