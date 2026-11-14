import { useEffect, useState } from 'preact/hooks';

import { api, setToken } from './api';
import type { Schemas } from './api';
import type { Config } from './config';
import * as mock from './mock';
import { Link, navigate, useRoute } from './router';
import { Activity } from './screens/activity';
import { Downloaders } from './screens/downloaders';
import { Jobs } from './screens/jobs';
import { Libraries } from './screens/libraries';
import { Overview } from './screens/overview';
import { Requests } from './screens/requests';
import { Settings } from './screens/settings';
import { Users } from './screens/users';
import { forgetToken, storedToken } from './session';
import { SignIn } from './signin';

type User = Schemas['UserDto'];

type State =
  | { stage: 'loading' }
  | { stage: 'signedOut' }
  | { stage: 'refused'; user: User }
  | { stage: 'ready'; user: User };

const screens = [
  { path: 'overview', label: 'Overview' },
  { path: 'requests', label: 'Requests and Reports' },
  { path: 'downloaders', label: 'Downloaders' },
  { path: 'libraries', label: 'Libraries' },
  { path: 'users', label: 'Users' },
  { path: 'jobs', label: 'Jobs' },
  { path: 'activity', label: 'Activity' },
  { path: 'settings', label: 'Settings' },
];

function Screen({ route }: { route: string[] }) {
  switch (route[0]) {
    case 'requests':
      return <Requests />;
    case 'downloaders':
      return <Downloaders id={route[1]} />;
    case 'libraries':
      return <Libraries />;
    case 'users':
      return <Users id={route[1]} />;
    case 'jobs':
      return <Jobs />;
    case 'activity':
      return <Activity />;
    case 'settings':
      return <Settings />;
    default:
      return <Overview />;
  }
}

export function App({ config }: { config: Config }) {
  const [state, setState] = useState<State>({ stage: 'loading' });
  const route = useRoute();
  const waiting = mock.mocked
    ? mock.titles.filter((title) => title.Requests.some((entry) => entry.Status === 'Pending')).length +
      mock.reports.filter((report) => report.Status === 'Open').length
    : 0;

  async function resolve() {
    if (mock.mocked) {
      setState({ stage: 'ready', user: mock.user });
      return;
    }
    const token = storedToken(config.ServerId);
    setToken(token);
    if (!token) {
      setState({ stage: 'signedOut' });
      return;
    }

    const { data } = await api.GET('/Users/Me');
    if (!data) {
      setToken(undefined);
      setState({ stage: 'signedOut' });
      return;
    }
    setState({ stage: data.Policy?.IsAdministrator ? 'ready' : 'refused', user: data });
  }

  async function signOut() {
    await api.POST('/Sessions/Logout');
    forgetToken(config.ServerId);
    setToken(undefined);
    setState({ stage: 'signedOut' });
    navigate('/');
  }

  useEffect(() => {
    void resolve();
  }, []);

  if (state.stage === 'loading') {
    return <div class="center muted">Loading…</div>;
  }
  if (state.stage === 'signedOut') {
    return <SignIn config={config} onSignedIn={resolve} />;
  }
  if (state.stage === 'refused') {
    return (
      <div class="center">
        <div class="card narrow">
          <h1>Administrators only</h1>
          <p class="muted">{state.user.Name} is signed in, and is not an administrator of this server.</p>
          <div class="actions">
            <a class="button" href="/web/">Open Jellyfin</a>
            <button onClick={signOut}>Sign out</button>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div class="shell">
      <nav>
        <div class="brand">
          go<span>jellyfin</span> admin
        </div>
        {screens.map((screen) => (
          <Link key={screen.path} to={'/' + screen.path} class={(route[0] ?? 'overview') === screen.path ? 'on' : ''}>
            {screen.label}
            {screen.path === 'requests' && waiting > 0 && <span class="count">{waiting}</span>}
          </Link>
        ))}
        <div class="foot">
          {mock.mocked && <div>{mock.build}</div>}
          <div>Signed in as {state.user.Name}</div>
          <div class="actions">
            <a href="/web/">Open Jellyfin</a>
            <button class="link" onClick={signOut}>
              Sign out
            </button>
          </div>
        </div>
      </nav>
      <main>
        <Screen route={route} />
      </main>
    </div>
  );
}
