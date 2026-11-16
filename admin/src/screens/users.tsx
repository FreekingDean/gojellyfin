import { useEffect, useState } from 'preact/hooks';

import { api } from '../api';
import * as mock from '../mock';
import { Link, navigate } from '../router';
import { Card, Cell, Page, Pill, notify, refused, since } from '../ui';
import { Schedules, grantsOf, levels, permissions, ratingOf, saveAllowance, savePolicy, schedulesOf, syncPlay, unratedKinds, when } from './policy';
import type { Device, Folder, Level, Person, SyncPlay } from './policy';
import { Bulk } from './users-bulk';

type Loaded = { people: Person[]; folders: Folder[]; ratings: Level[] };

function mocked(): Loaded {
  const folders = mock.libraries.map((library) => ({ id: library.id, name: library.name }));

  return {
    folders,
    ratings: levels(mock.ratings),
    people: mock.people.map((person) => ({
      ...person,
      folders: person.libraries === 'all' ? 'all' : folders.filter((folder) => person.libraries.includes(folder.name)).map((folder) => folder.id),
      devices: person.devices.map((device) => ({ ...device, id: device.name })),
      schedules: person.schedules ?? [],
      grants: { ...grantsOf(), ...person.grants },
      rating: person.rating ?? ratingOf(),
    })),
  };
}

async function fetched(): Promise<Loaded | undefined> {
  const [users, allowances, devices, libraries, ratings] = await Promise.all([
    api.GET('/Users'),
    api.GET('/GoJellyfin/RequestUsers'),
    api.GET('/Devices'),
    api.GET('/Library/VirtualFolders'),
    api.GET('/Localization/ParentalRatings'),
  ]);
  if (!users.data || !allowances.data || !devices.data || !libraries.data) {
    return undefined;
  }

  return {
    folders: libraries.data.map((library) => ({ id: library.ItemId ?? '', name: library.Name ?? '' })),
    ratings: levels(ratings.data ?? []),
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
        schedules: schedulesOf(user.Policy ?? undefined),
        grants: grantsOf(user.Policy ?? undefined),
        rating: ratingOf(user.Policy ?? undefined),
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
  const [picked, setPicked] = useState<string[]>([]);
  const [bulk, setBulk] = useState(false);

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
    return <Detail key={person.id} person={person} folders={loaded.folders} ratings={loaded.ratings} reload={load} />;
  }

  const chosen = loaded.people.filter((each) => picked.includes(each.id));
  if (bulk && chosen.length > 0) {
    return (
      <Bulk
        people={chosen}
        folders={loaded.folders}
        ratings={loaded.ratings}
        cancel={() => setBulk(false)}
        finish={async (failed, drafts) => {
          if (mock.mocked) {
            setLoaded({ ...loaded, people: loaded.people.map((each) => drafts.find((draft) => draft.id === each.id) ?? each) });
          } else {
            await load();
          }
          setPicked(failed);
          setBulk(failed.length > 0);
        }}
      />
    );
  }

  const everyone = picked.length > 0 && picked.length === loaded.people.length;

  function pick(who: string, on: boolean) {
    setPicked(on ? [...picked, who] : picked.filter((each) => each !== who));
  }

  return (
    <Page
      title="Users"
      lead="Who can sign in, what they can see, and what they can ask for."
      action={
        picked.length > 0 ? (
          <span class="actions">
            <span class="muted">{picked.length} selected</span>
            <button onClick={() => setPicked([])}>Clear</button>
            <button class="primary" onClick={() => setBulk(true)}>
              Edit {chosen.length === 1 ? '1 user' : chosen.length + ' users'}
            </button>
          </span>
        ) : (
          <Link to="/users/new" class="button primary">
            Add user
          </Link>
        )
      }
    >
      <div class="card flush">
        <table class="rows">
          <colgroup>
            <col style="width:44px" />
            <col style="width:24%" />
            <col style="width:22%" />
            <col style="width:26%" />
            <col style="width:16%" />
            <col style="width:12%" />
          </colgroup>
          <thead>
            <tr>
              <th>
                <input
                  type="checkbox"
                  aria-label="Select everyone"
                  checked={everyone}
                  onChange={(event) => setPicked(event.currentTarget.checked ? loaded.people.map((each) => each.id) : [])}
                />
              </th>
              <th>User</th>
              <th>Can see</th>
              <th>Requests</th>
              <th>Last seen</th>
              <th />
            </tr>
          </thead>
          <tbody>
            {loaded.people.map((each) => (
              <tr key={each.id} class={[each.disabled && 'dim', picked.includes(each.id) && 'picked'].filter(Boolean).join(' ')}>
                <td>
                  <input type="checkbox" aria-label={'Select ' + each.name} checked={picked.includes(each.id)} onChange={(event) => pick(each.id, event.currentTarget.checked)} />
                </td>
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
                <Cell main={access(each, loaded.folders)} sub={each.schedules.length > 0 ? when(each.schedules) : undefined} />
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

  async function submit(event: Event) {
    event.preventDefault();
    if (!mock.mocked) {
      const { data, error } = await api.POST('/Users/New', { body: { Name: name, Password: password } });
      if (!data?.Id) {
        refused(error, 'The user could not be added. The name may already be taken.');
        return;
      }
      notify(name + ' added.');
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

function Detail({ person, folders, ratings, reload }: { person: Person; folders: Folder[]; ratings: Level[]; reload: () => Promise<void> }) {
  const [draft, setDraft] = useState(person);
  const [password, setPassword] = useState<string | undefined>(undefined);

  function change(patch: Partial<Person>) {
    setDraft({ ...draft, ...patch });
  }

  function see(folder: Folder, on: boolean) {
    const current = draft.folders === 'all' ? folders.map((each) => each.id) : draft.folders;
    const next = on ? [...current, folder.id] : current.filter((each) => each !== folder.id);
    change({ folders: next.length === folders.length ? 'all' : next });
  }

  async function finish(answer: { response: Response; error?: unknown }, fallback: string, done: string) {
    if (!answer.response.ok) {
      refused(answer.error, fallback);
      return false;
    }
    notify(done);
    await reload();

    return true;
  }

  async function save() {
    if (mock.mocked || !person.policy) {
      navigate('/users');
      return;
    }
    const [policy, allowance] = await Promise.all([savePolicy(person, draft), saveAllowance(draft)]);
    if (!policy) {
      navigate('/users');
      return;
    }
    const fallback = draft.disabled && draft.admin ? 'An administrator cannot be disabled. Take administrator away first.' : 'The changes to ' + person.name + ' could not be saved.';
    if (await finish(policy.response.ok ? allowance : policy, fallback, person.name + ' saved.')) {
      navigate('/users');
    }
  }

  async function setNewPassword(event: Event) {
    event.preventDefault();
    const answer = await api.POST('/Users/Password', {
      params: { query: { userId: person.id } },
      body: { NewPw: password ?? '', ResetPassword: true },
    });
    if (await finish(answer, 'The password could not be changed.', 'Password changed. Every device ' + person.name + ' was signed in on has been signed out.')) {
      setPassword(undefined);
    }
  }

  async function signOut(devices: Device[]) {
    const answer = await api.DELETE('/Devices', { params: { query: { id: devices.map((device) => device.id) } } });
    await finish(answer, 'The device could not be signed out.', 'Signed out.');
  }

  async function remove() {
    if (!window.confirm('Delete ' + person.name + '? Their watch history, favourites and sessions go with them.')) {
      return;
    }
    const answer = await api.DELETE('/Users/{userId}', { params: { path: { userId: person.id } } });
    if (await finish(answer, person.name + ' could not be deleted.', person.name + ' deleted.')) {
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

        <Card title="When they can sign in">
          <Schedules schedules={draft.schedules} change={(schedules) => change({ schedules })} />
        </Card>

        <Card title="Permissions">
          {permissions.map((permission) => (
            <label class="check" key={permission.key}>
              <input
                type="checkbox"
                checked={draft.grants[permission.key]}
                onChange={(event) => change({ grants: { ...draft.grants, [permission.key]: event.currentTarget.checked } })}
              />{' '}
              {permission.label}
            </label>
          ))}
          <div class="form one">
            <label>
              Watch parties
              <select value={draft.grants.SyncPlayAccess} onChange={(event) => change({ grants: { ...draft.grants, SyncPlayAccess: event.currentTarget.value as SyncPlay } })}>
                {syncPlay.map((each) => (
                  <option key={each.value} value={each.value}>
                    {each.label}
                  </option>
                ))}
              </select>
            </label>
          </div>
          {draft.admin && <p class="muted small">An administrator can do all of these whatever is ticked.</p>}
        </Card>

        <Card title="Parental controls">
          <div class="form one">
            <label>
              Highest rating they can see
              <select
                value={draft.rating.max === undefined ? '' : String(draft.rating.max)}
                onChange={(event) =>
                  change({ rating: { ...draft.rating, max: event.currentTarget.value === '' ? undefined : Number(event.currentTarget.value) } })
                }
              >
                <option value="">No limit</option>
                {ratings.map((level) => (
                  <option key={level.value} value={String(level.value)}>
                    {level.label}
                  </option>
                ))}
              </select>
            </label>
          </div>
          {unratedKinds.map((each) => (
            <label class="check" key={each.kind}>
              <input
                type="checkbox"
                checked={draft.rating.unrated.includes(each.kind)}
                onChange={(event) =>
                  change({
                    rating: {
                      ...draft.rating,
                      unrated: event.currentTarget.checked ? [...draft.rating.unrated, each.kind] : draft.rating.unrated.filter((kind) => kind !== each.kind),
                    },
                  })
                }
              />{' '}
              Hide {each.label.toLowerCase()}
            </label>
          ))}
          <p class="muted small">{draft.admin ? 'An administrator sees everything whatever is set here.' : 'An episode with no rating of its own takes its show\'s.'}</p>
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

