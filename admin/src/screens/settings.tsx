import { useEffect, useState } from 'preact/hooks';

import { api } from '../api';
import type { Schemas } from '../api';
import * as mock from '../mock';
import { Card, Cell, Page, Pill, since } from '../ui';

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
  const [notice, setNotice] = useState('');

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
    setNotice('');
    setShelves(shelves?.map((shelf) => (shelf.slug === slug ? { ...shelf, on } : shelf)));
  }

  async function save() {
    if (mock.mocked || !shelves) {
      return;
    }
    const { response } = await api.PUT('/GoJellyfin/Shelves', { body: { Enabled: shelves.filter((shelf) => shelf.on).map((shelf) => shelf.slug) } });
    setFailure(response.ok ? '' : 'The shelves could not be saved.');
    setNotice(response.ok ? 'Shelves saved. They show on the next visit to the home screen.' : '');
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
      {failure && (
        <p class="failure" role="alert">
          {failure}
        </p>
      )}
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
        {notice && (
          <p class="muted small" role="status">
            {notice}
          </p>
        )}
      </Card>

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

type Stored = { LoginDisclaimer?: string; CustomCss?: string; SplashscreenEnabled?: boolean } & Record<string, unknown>;

function Branding() {
  const [stored, setStored] = useState<Stored | undefined>(mock.mocked ? { LoginDisclaimer: mock.branding.disclaimer, CustomCss: mock.branding.css } : undefined);
  const [failure, setFailure] = useState('');
  const [notice, setNotice] = useState('');

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
      setNotice('');
      setStored({ ...stored, [field]: (event.currentTarget as HTMLTextAreaElement).value });
    };
  }

  async function save() {
    if (mock.mocked || !stored) {
      return;
    }
    const { response } = await api.POST('/System/Configuration/{key}', { params: { path: { key: 'branding' } }, body: stored });
    setFailure(response.ok ? '' : 'The branding could not be saved.');
    setNotice(response.ok ? 'Branding saved. Clients pick it up the next time they load.' : '');
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
          {notice && (
            <p class="muted small" role="status">
              {notice}
            </p>
          )}
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
    const { response } = await api.POST('/Auth/Keys', { params: { query: { app: name } } });
    if (!response.ok) {
      setFailure('The key could not be made.');
      return;
    }
    setFailure('');
    setNaming(undefined);
    setMade((await load()).find((key) => !before.has(key.token)));
  }

  async function revoke(key: Key) {
    if (mock.mocked) {
      return;
    }
    const { response } = await api.DELETE('/Auth/Keys/{key}', { params: { path: { key: key.token } } });
    setFailure(response.ok ? '' : 'The key could not be revoked.');
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
      <p class="muted small">A key is for an app such as Home Assistant, and is shown in full once, when it is made. This server does not accept one in place of a sign-in yet (#722).</p>
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
