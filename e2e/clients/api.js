const HARNESS = 'MediaBrowser Client="e2e harness", Device="harness", DeviceId="e2e-harness", Version="1.0.0"';

// The server's own account of what a client did, asked as a second device
// signed in as the same user: a native app has no page to evaluate script in,
// so what it reported is read back from the API it reported to.
async function api(app) {
  const session = await call(app, 'POST', '/Users/AuthenticateByName', {
    headers: { Authorization: HARNESS },
    body: { Username: app.username, Pw: app.password },
  });
  const user = session.User.Id;
  const headers = { Authorization: `${HARNESS}, Token="${session.AccessToken}"` };

  async function item(name) {
    const query = new URLSearchParams({ userId: user, recursive: 'true', searchTerm: name, enableUserData: 'true' });
    const found = await call(app, 'GET', `/Items?${query}`, { headers });
    const match = found.Items.find((candidate) => candidate.Name === name);
    if (!match) {
      throw new Error(`the server holds no item named ${name}`);
    }

    return match;
  }

  return {
    user,
    item,
    async positionSeconds(name) {
      return ((await item(name)).UserData?.PlaybackPositionTicks || 0) / 1e7;
    },
    async forget(name) {
      const { Id: id } = await item(name);
      await call(app, 'POST', `/UserItems/${id}/UserData?userId=${user}`, {
        headers,
        body: { PlaybackPositionTicks: 0, Played: false, PlayCount: 0, LastPlayedDate: null },
      });
    },
    async leaveAt(name, seconds) {
      const { Id: id } = await item(name);
      await call(app, 'POST', `/UserItems/${id}/UserData?userId=${user}`, {
        headers,
        body: { PlaybackPositionTicks: seconds * 1e7, Played: false },
      });
    },
    authorizeQuickConnect(code) {
      return call(app, 'POST', `/QuickConnect/Authorize?code=${code}&userId=${user}`, { headers });
    },
  };
}

async function call(app, method, path, { headers, body } = {}) {
  const response = await fetch(app.baseUrl + path, {
    method,
    headers: { ...headers, ...(body ? { 'Content-Type': 'application/json' } : {}) },
    body: body ? JSON.stringify(body) : undefined,
  });
  const text = await response.text();
  if (!response.ok) {
    throw new Error(`${method} ${path} answered ${response.status}: ${text.slice(0, 500)}`);
  }

  return text ? JSON.parse(text) : null;
}

module.exports = { api };
