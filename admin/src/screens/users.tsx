import { useEffect, useState } from 'preact/hooks';

import { api } from '../api';
import type { Schemas } from '../api';
import * as mock from '../mock';
import { Link, navigate } from '../router';
import { Card, Cell, Page, Pill, since } from '../ui';

type Folder = { id: string; name: string };
type Device = { id: string; name: string; app: string; seen?: string };
type Person = {
  id: string;
  name: string;
  admin: boolean;
  disabled: boolean;
  sso?: boolean;
  password: boolean;
  folders: string[] | 'all';
  canRequest: boolean;
  needsApproval: boolean;
  quota?: number;
  used: number;
  seen?: string;
  devices: Device[];
  policy?: Schemas['UserPolicy'];
};
type Loaded = { people: Person[]; folders: Folder[] };

function mocked(): Loaded {
  const folders = mock.libraries.map((library) => ({ id: library.id, name: library.name }));

  return {
    folders,
    people: mock.people.map((person) => ({
      ...person,
      folders: person.libraries === 'all' ? 'all' : folders.filter((folder) => person.libraries.includes(folder.name)).map((folder) => folder.id),
      devices: person.devices.map((device) => ({ ...device, id: device.name })),
    })),
  };
}

async function fetched(): Promise<Loaded | undefined> {
  const [users, allowances, devices, libraries] = await Promise.all([
    api.GET('/Users'),
    api.GET('/GoJellyfin/RequestUsers'),
    api.GET('/Devices'),
    api.GET('/Library/VirtualFolders'),
  ]);
  if (!users.data || !allowances.data || !devices.data || !libraries.data) {
    return undefined;
  }

  return {
    folders: libraries.data.map((library) => ({ id: library.ItemId ?? '', name: library.Name ?? '' })),
    people: users.data.map((user) => {
      const allowance = allowances.data.find((each) => each.Id === user.Id);

      return {
        id: user.Id ?? '',
        name: user.Name ?? '',
        admin: Boolean(user.Policy?.IsAdministrator),
        disabled: Boolean(user.Policy?.IsDisabled),
        password: Boolean(user.HasPassword),
        folders: user.Policy?.EnableAllFolders ? 'all' : (user.Policy?.EnabledFolders ?? []),
        canRequest: allowance?.CanRequest ?? true,
        needsApproval: allowance?.NeedsApproval ?? false,
        quota: allowance?.Quota ?? undefined,
        used: allowance?.Used ?? 0,
        seen: user.LastActivityDate ?? undefined,
        policy: user.Policy ?? undefined,
        devices: (devices.data.Items ?? [])
          .filter((device) => device.LastUserId === user.Id)
          .map((device) => ({
            id: device.Id ?? '',
            name: device.CustomName || device.Name || 'Unnamed device',
            app: [device.AppName, device.AppVersion].filter(Boolean).join(' '),
            seen: device.DateLastActivity ?? undefined,
          })),
      };
    }),
  };
}

function access(person: Person, folders: Folder[]): string {
  if (person.folders === 'all') {
    return 'Everything';
  }
  const names = folders.filter((folder) => person.folders.includes(folder.id)).map((folder) => folder.name);

  return names.length === 0 ? 'Nothing' : names.join(', ') + ' only';
}

function asking(person: Person): { main: string; sub: string } {
  if (!person.canRequest) {
    return { main: 'Cannot request', sub: '' };
  }

  return {
    main: person.quota === undefined ? person.used + ' this week, no limit' : person.used + ' of ' + person.quota + ' this week',
    sub: person.needsApproval ? 'Needs approval' : 'Sent straight to the downloader',
  };
}

function signsInWith(person: Person): string {
  if (person.sso === undefined) {
    return person.password ? 'Password' : 'No password';
  }

  return [person.sso && 'Single sign-on', person.password && 'Password'].filter(Boolean).join(' and ');
}

export function Users({ id }: { id?: string }) {
  const [loaded, setLoaded] = useState<Loaded | undefined>(mock.mocked ? mocked() : undefined);
  const [failure, setFailure] = useState('');

  async function load() {
    if (mock.mocked) {
      return;
    }
    const answer = await fetched();
    if (!answer) {
      setFailure('The users could not be loaded.');
      return;
    }
    setFailure('');
    setLoaded(answer);
  }

  useEffect(() => {
    void load();
  }, []);

  if (!loaded) {
    return (
      <Page title="Users" lead="Who can sign in, what they can see, and what they can ask for.">
        <p class={failure ? 'failure' : 'muted'}>{failure || 'Loading…'}</p>
      </Page>
    );
  }
  if (id === 'new') {
    return <Add reload={load} />;
  }

  const person = loaded.people.find((each) => each.id === id);
  if (person) {
    return <Detail key={person.id} person={person} folders={loaded.folders} reload={load} />;
  }

  return (
    <Page
      title="Users"
      lead="Who can sign in, what they can see, and what they can ask for."
      action={
        <Link to="/users/new" class="button primary">
          Add user
        </Link>
      }
    >
      <div class="card flush">
        <table class="rows">
          <colgroup>
            <col style="width:24%" />
            <col style="width:22%" />
            <col style="width:26%" />
            <col style="width:16%" />
            <col style="width:12%" />
          </colgroup>
          <thead>
            <tr>
              <th>User</th>
              <th>Can see</th>
              <th>Requests</th>
              <th>Last seen</th>
              <th />
            </tr>
          </thead>
          <tbody>
            {loaded.people.map((each) => (
              <tr key={each.id} class={each.disabled ? 'dim' : ''}>
                <Cell
                  main={
                    <>
                      {each.name}
                      {each.admin && <Pill tone="ok">Admin</Pill>}
                      {each.disabled && <Pill tone="bad">Disabled</Pill>}
                    </>
                  }
                  sub={signsInWith(each)}
                />
                <Cell main={access(each, loaded.folders)} />
                <Cell main={asking(each).main} sub={asking(each).sub} />
                <Cell main={each.seen ? since(each.seen) : 'Never'} sub={each.devices.length + (each.devices.length === 1 ? ' device' : ' devices')} />
                <td>
                  <div class="actions right">
                    <Link to={'/users/' + each.id} class="button">
                      Edit
                    </Link>
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </Page>
  );
}

function Add({ reload }: { reload: () => Promise<void> }) {
  const [name, setName] = useState('');
  const [password, setPassword] = useState('');
  const [failure, setFailure] = useState('');

  async function submit(event: Event) {
    event.preventDefault();
    if (!mock.mocked) {
      const { data } = await api.POST('/Users/New', { body: { Name: name, Password: password } });
      if (!data?.Id) {
        setFailure('The user could not be added. The name may already be taken.');
        return;
      }
      await reload();
      navigate('/users/' + data.Id);
      return;
    }
    navigate('/users');
  }

  return (
    <Page title="Add user" lead="They can see everything and ask for anything until you say otherwise on the next screen.">
      <form class="card narrow" onSubmit={submit}>
        <label>
          Name
          <input name="name" autocomplete="off" value={name} onInput={(event) => setName(event.currentTarget.value)} required />
        </label>
        <label>
          Password
          <input name="password" type="password" autocomplete="new-password" value={password} onInput={(event) => setPassword(event.currentTarget.value)} />
        </label>
        <p class="muted small">Leave the password empty for someone who only signs in with single sign-on.</p>
        {failure && (
          <p class="failure" role="alert">
            {failure}
          </p>
        )}
        <div class="actions">
          <Link to="/users" class="button">
            Cancel
          </Link>
          <button class="primary" type="submit">
            Add user
          </button>
        </div>
      </form>
    </Page>
  );
}

function Detail({ person, folders, reload }: { person: Person; folders: Folder[]; reload: () => Promise<void> }) {
  const [draft, setDraft] = useState(person);
  const [password, setPassword] = useState<string | undefined>(undefined);
  const [failure, setFailure] = useState('');
  const [notice, setNotice] = useState('');

  function change(patch: Partial<Person>) {
    setDraft({ ...draft, ...patch });
  }

  function see(folder: Folder, on: boolean) {
    const current = draft.folders === 'all' ? folders.map((each) => each.id) : draft.folders;
    const next = on ? [...current, folder.id] : current.filter((each) => each !== folder.id);
    change({ folders: next.length === folders.length ? 'all' : next });
  }

  async function finish(ok: boolean, refused: string, done?: string) {
    if (!ok) {
      setNotice('');
      setFailure(refused);
      return false;
    }
    setFailure('');
    setNotice(done ?? '');
    await reload();

    return true;
  }

  async function save() {
    if (mock.mocked || !person.policy) {
      navigate('/users');
      return;
    }
    const [policy, allowance] = await Promise.all([
      api.POST('/Users/{userId}/Policy', {
        params: { path: { userId: person.id } },
        body: {
          ...person.policy,
          IsAdministrator: draft.admin,
          IsDisabled: draft.disabled,
          EnableAllFolders: draft.folders === 'all',
          EnabledFolders: draft.folders === 'all' ? [] : draft.folders,
        },
      }),
      api.PUT('/GoJellyfin/RequestUsers/{userId}', {
        params: { path: { userId: person.id } },
        body: { CanRequest: draft.canRequest, NeedsApproval: draft.needsApproval, Quota: draft.quota ?? null },
      }),
    ]);
    const refused = draft.disabled && draft.admin ? 'An administrator cannot be disabled. Take administrator away first.' : 'The changes could not be saved.';
    if (await finish(policy.response.ok && allowance.response.ok, refused)) {
      navigate('/users');
    }
  }

  async function setNewPassword(event: Event) {
    event.preventDefault();
    const { response } = await api.POST('/Users/Password', {
      params: { query: { userId: person.id } },
      body: { NewPw: password ?? '', ResetPassword: true },
    });
    if (await finish(response.ok, 'The password could not be changed.', 'Password changed. Every device ' + person.name + ' was signed in on has been signed out.')) {
      setPassword(undefined);
    }
  }

  async function signOut(devices: Device[]) {
    const { response } = await api.DELETE('/Devices', { params: { query: { id: devices.map((device) => device.id) } } });
    await finish(response.ok, 'The device could not be signed out.');
  }

  async function remove() {
    if (!window.confirm('Delete ' + person.name + '? Their watch history, favourites and sessions go with them.')) {
      return;
    }
    const { response } = await api.DELETE('/Users/{userId}', { params: { path: { userId: person.id } } });
    if (await finish(response.ok, 'The user could not be deleted.')) {
      navigate('/users');
    }
  }

  return (
    <Page
      title={person.name}
      lead={person.sso === undefined ? (person.password ? 'Has a password.' : 'Has no password, so signs in with single sign-on only.') : signsInWith(person) + '.'}
      action={
        <span class="actions">
          <Link to="/users" class="button">
            Cancel
          </Link>
          <button class="primary" onClick={save}>
            Save
          </button>
        </span>
      }
    >
      {failure && (
        <p class="failure" role="alert">
          {failure}
        </p>
      )}
      {notice && <p class="muted">{notice}</p>}
      <div class="grid two">
        <Card title="Account">
          <label class="check">
            <input type="checkbox" checked={draft.admin} onChange={(event) => change({ admin: event.currentTarget.checked })} /> Administrator
          </label>
          {person.sso && person.admin && <p class="muted small">Set from the single sign-on administrator group at every sign-in.</p>}
          <label class="check">
            <input type="checkbox" checked={draft.disabled} onChange={(event) => change({ disabled: event.currentTarget.checked })} /> Disabled
          </label>
          <p class="muted small">Disabling stops every device this user is signed in on at once.</p>
          {password === undefined ? (
            <div class="actions spaced">
              <button disabled={mock.mocked} onClick={() => setPassword('')}>
                {person.password ? 'Change password' : 'Set a password'}
              </button>
              <button disabled={mock.mocked || person.devices.length === 0} onClick={() => signOut(person.devices)}>
                Sign out everywhere
              </button>
            </div>
          ) : (
            <form class="form one" onSubmit={setNewPassword}>
              <label>
                New password
                <input name="new-password" type="password" autocomplete="new-password" value={password} onInput={(event) => setPassword(event.currentTarget.value)} required />
              </label>
              <div class="actions">
                <button type="button" onClick={() => setPassword(undefined)}>
                  Cancel
                </button>
                <button class="primary" type="submit">
                  Change password
                </button>
              </div>
            </form>
          )}
        </Card>

        <Card title="Requests">
          <label class="check">
            <input type="checkbox" checked={draft.canRequest} onChange={(event) => change({ canRequest: event.currentTarget.checked })} /> Can ask for titles the
            library lacks
          </label>
          <label class="check">
            <input type="checkbox" checked={draft.needsApproval} onChange={(event) => change({ needsApproval: event.currentTarget.checked })} /> An administrator
            approves each request first
          </label>
          <div class="form">
            <label>
              Most titles in seven days
              <input
                name="quota"
                type="number"
                min="0"
                value={draft.quota ?? ''}
                placeholder="No limit"
                onInput={(event) => change({ quota: event.currentTarget.value === '' ? undefined : Number(event.currentTarget.value) })}
              />
            </label>
          </div>
          <p class="muted small">{person.used} asked for in the last seven days.</p>
        </Card>

        <Card title="Can see">
          {folders.length === 0 && <p class="muted">There are no libraries yet.</p>}
          {folders.map((folder) => (
            <label class="check" key={folder.id}>
              <input
                type="checkbox"
                checked={draft.folders === 'all' || draft.folders.includes(folder.id)}
                onChange={(event) => see(folder, event.currentTarget.checked)}
              />{' '}
              {folder.name}
            </label>
          ))}
        </Card>

        <Card title="Devices">
          {person.devices.length === 0 ? (
            <p class="muted">Not signed in anywhere.</p>
          ) : (
            <table class="rows plain">
              <tbody>
                {person.devices.map((device) => (
                  <tr key={device.id}>
                    <Cell main={device.name} sub={device.app} />
                    <td class="muted">{device.seen ? since(device.seen) : ''}</td>
                    <td class="end">
                      <button disabled={mock.mocked} onClick={() => signOut([device])}>
                        Sign out
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </Card>
      </div>
      <p class="danger">
        <button class="bad" disabled={mock.mocked} onClick={remove}>
          Delete user
        </button>
        <span class="muted small">Deleting removes their watch history, favourites and sessions.</span>
      </p>
    </Page>
  );
}
