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
      <Page title="Settings" lead="Shelves are edited here. Everything else is set in the environment, so it is shown here and changed in your deployment.">
        <p class={failure ? 'failure' : 'muted'}>{failure || 'Loading…'}</p>
      </Page>
    );
  }

  return (
    <Page
      title="Settings"
      lead={
        mock.mocked
          ? 'Shelves, branding and API keys are edited here. Everything else is set in the environment, so it is shown here and changed in your deployment.'
          : 'Shelves are edited here. Everything else is set in the environment, so it is shown here and changed in your deployment.'
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

        {mock.mocked && (
          <>
            <Card title="Branding">
              <div class="form one">
                <label>
                  Sign-in message
                  <textarea rows={2}>{mock.branding.disclaimer}</textarea>
                </label>
                <label>
                  Custom CSS for the web client
                  <textarea rows={3} class="mono">
                    {mock.branding.css}
                  </textarea>
                </label>
              </div>
              <div class="actions spaced">
                <button class="primary">Save branding</button>
              </div>
            </Card>

            <Card>
              <header class="cardhead">
                <b>API keys</b>
                <button>New key</button>
              </header>
              <table class="rows plain">
                <tbody>
                  {mock.apiKeys.map((key) => (
                    <tr key={key.name}>
                      <Cell main={key.name} sub={'Created ' + since(key.created)} />
                      <td class="muted">{key.used ? 'Used ' + since(key.used) : 'Never used'}</td>
                      <td class="end">
                        <button>Revoke</button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </Card>
          </>
        )}
      </div>
    </Page>
  );
}
