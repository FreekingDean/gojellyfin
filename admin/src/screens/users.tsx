import * as mock from '../mock';
import { Link } from '../router';
import { Card, Cell, Page, Pill, since } from '../ui';

function access(person: mock.Person): string {
  return person.libraries === 'all' ? 'Movies and Shows' : person.libraries.join(', ') + ' only';
}

function asking(person: mock.Person): { main: string; sub: string } {
  if (!person.canRequest) {
    return { main: 'Cannot request', sub: '' };
  }

  return {
    main: person.quota === undefined ? person.used + ' this week, no limit' : person.used + ' of ' + person.quota + ' this week',
    sub: person.needsApproval ? 'Needs approval' : 'Sent straight to the downloader',
  };
}

export function Users({ id }: { id?: string }) {
  const person = mock.people.find((each) => each.id === id);
  if (person) {
    return <Detail person={person} />;
  }

  return (
    <Page title="Users" lead="Who can sign in, what they can see, and what they can ask for." action={<button class="primary">Add user</button>}>
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
            {mock.people.map((each) => (
              <tr key={each.id} class={each.disabled ? 'dim' : ''}>
                <Cell
                  main={
                    <>
                      {each.name}
                      {each.admin && <Pill tone="ok">Admin</Pill>}
                      {each.disabled && <Pill tone="bad">Disabled</Pill>}
                    </>
                  }
                  sub={[each.sso && 'Single sign-on', each.password && 'Password'].filter(Boolean).join(' and ')}
                />
                <Cell main={access(each)} />
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

function Detail({ person }: { person: mock.Person }) {
  return (
    <Page
      title={person.name}
      lead={[person.sso && 'Signs in with single sign-on', person.password && 'has a password'].filter(Boolean).join(', ') + '.'}
      action={
        <span class="actions">
          <Link to="/users" class="button">
            Cancel
          </Link>
          <button class="primary">Save</button>
        </span>
      }
    >
      <div class="grid two">
        <Card title="Account">
          <label class="check">
            <input type="checkbox" checked={person.admin} /> Administrator
          </label>
          {person.sso && person.admin && <p class="muted small">Set from the single sign-on administrator group at every sign-in.</p>}
          <label class="check">
            <input type="checkbox" checked={person.disabled} /> Disabled
          </label>
          <p class="muted small">Disabling stops every device this user is signed in on at once.</p>
          <div class="actions spaced">
            <button>{person.password ? 'Change password' : 'Set a password'}</button>
            <button>Sign out everywhere</button>
          </div>
        </Card>

        <Card title="Requests">
          <label class="check">
            <input type="checkbox" checked={person.canRequest} /> Can ask for titles the library lacks
          </label>
          <label class="check">
            <input type="checkbox" checked={person.needsApproval} /> An administrator approves each request first
          </label>
          <div class="form">
            <label>
              Most titles in seven days
              <input type="number" min="0" value={person.quota ?? ''} placeholder="No limit" />
            </label>
          </div>
          <p class="muted small">{person.used} asked for in the last seven days.</p>
        </Card>

        <Card title="Can see">
          {mock.libraries.map((library) => (
            <label class="check" key={library.id}>
              <input type="checkbox" checked={person.libraries === 'all' || person.libraries.includes(library.name)} /> {library.name}
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
                  <tr key={device.name}>
                    <Cell main={device.name} sub={device.app} />
                    <td class="muted">{since(device.seen)}</td>
                    <td class="end">
                      <button>Sign out</button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </Card>
      </div>
      <p class="danger">
        <button class="bad">Delete user</button>
        <span class="muted small">Deleting removes their watch history, favourites and sessions.</span>
      </p>
    </Page>
  );
}
