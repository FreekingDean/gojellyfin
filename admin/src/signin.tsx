import { useState } from 'preact/hooks';

import { api, setToken } from './api';
import type { Config } from './config';
import { storeToken } from './session';

export function SignIn({ config, onSignedIn }: { config: Config; onSignedIn: () => Promise<void> }) {
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [failure, setFailure] = useState('');
  const [busy, setBusy] = useState(false);

  async function submit(event: Event) {
    event.preventDefault();
    setBusy(true);
    setFailure('');
    setToken(undefined);

    const { data } = await api.POST('/Users/AuthenticateByName', {
      body: { Username: username, Pw: password },
    });
    setBusy(false);
    if (!data?.AccessToken || !data.User?.Id) {
      setFailure('That username and password did not sign in.');
      return;
    }

    storeToken(config.ServerId || data.ServerId || '', data.AccessToken, data.User.Id);
    await onSignedIn();
  }

  return (
    <div class="center">
      <form class="card narrow" onSubmit={submit}>
        <h1>
          go<span class="accent">jellyfin</span> admin
        </h1>
        <label>
          Username
          <input name="username" autocomplete="username" value={username} onInput={(event) => setUsername(event.currentTarget.value)} required />
        </label>
        <label>
          Password
          <input
            name="password"
            type="password"
            autocomplete="current-password"
            value={password}
            onInput={(event) => setPassword(event.currentTarget.value)}
          />
        </label>
        {failure && <p class="failure">{failure}</p>}
        <div class="actions">
          <button class="primary" type="submit" disabled={busy}>
            Sign in
          </button>
          {config.SingleSignOn && (
            <a class="button" href="/sso/oidc/start?redirect=%2Fadmin">
              Single sign-on
            </a>
          )}
        </div>
      </form>
    </div>
  );
}
