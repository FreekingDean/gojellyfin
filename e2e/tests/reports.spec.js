const { test, expect } = require('./fixtures');
const { GRAB } = require('../setup/radarr');

async function replaced(harness) {
  const response = await fetch(harness.downloaderUrl + '/replaced');

  return response.json();
}

function queued(page, name) {
  return page.locator('#gojellyfinRequestsPage [data-id=reportsHost] tr', { hasText: name });
}

const CLIENT = 'MediaBrowser Client="Reports spec", Device="Playwright", DeviceId="reports-spec", Version="1"';

async function session(harness) {
  const response = await fetch(harness.baseUrl + '/Users/AuthenticateByName', {
    method: 'POST',
    headers: { Authorization: CLIENT, 'Content-Type': 'application/json' },
    body: JSON.stringify({ Username: harness.username, Pw: harness.password }),
  });
  const { AccessToken: token, User: user, ServerId: server } = await response.json();

  return { token, user: user.Id, server, headers: { Authorization: `${CLIENT}, Token="${token}"` } };
}

// The page reads the token a remembered sign-in leaves in localStorage, which
// the fixture's sign-in was not seen to leave there.
async function remember(page, signed) {
  await page.evaluate(({ token, user, server }) => {
    const stored = JSON.parse(localStorage.getItem('jellyfin_credentials') || '{}');
    const servers = stored.Servers || [];
    const entry = servers.find((known) => known.Id === server) || servers[0] || servers[servers.push({ Id: server }) - 1];
    entry.AccessToken = token;
    entry.UserId = user;
    localStorage.setItem('jellyfin_credentials', JSON.stringify({ ...stored, Servers: servers }));
  }, signed);
}

async function openQueue(page) {
  await page.locator('.headerButton.mainDrawerButton').click();
  await page.locator('.mainDrawer').getByText('Dashboard').click();
  await expect(page).toHaveURL(/dashboard/);

  await page.getByRole('link', { name: 'Requests and Reports' }).click();
  await expect(page).toHaveURL(/configurationpage\?name=requests/i);
}

test.describe('a broken copy', () => {
  test('is reported from /report and replaced from the dashboard', async ({ signedIn: page, harness }) => {
    await page.goto(harness.baseUrl + '/report');
    await remember(page, await session(harness));
    await page.reload();
    await page.locator('#term').fill(harness.playable);
    await page.locator('.title', { hasText: harness.playable }).click();

    await expect(page.locator('#chosen')).toHaveText(harness.playable);
    await page.getByLabel('The sound is wrong or missing').check();
    await page.locator('#note').fill('silent after the titles');
    await page.getByRole('button', { name: 'Send report' }).click();
    await expect(page.locator('#done')).toContainText(harness.playable);

    await page.goto(harness.baseUrl + '/web/');
    await openQueue(page);
    const report = queued(page, harness.playable).filter({ hasText: 'Audio' });
    await expect(report).toContainText('silent after the titles');
    await expect(report).toContainText(harness.username);

    await report.locator('[data-id=replace]').click();
    await expect(report).toContainText('Resolved');
    await expect(report.locator('[data-id=replace]')).toBeHidden();

    expect(await replaced(harness)).toEqual([
      'DELETE /api/v3/moviefile/72',
      `POST /api/v3/history/failed/${GRAB}`,
      'POST /api/v3/command',
    ]);
  });

  test('is reported, and kept, when a client deletes it', async ({ signedIn: page, harness }) => {
    const { user, headers } = await session(harness);
    const search = new URLSearchParams({ searchTerm: harness.transcoded, recursive: 'true', includeItemTypes: 'Movie', fields: 'CanDelete', userId: user });
    const [movie] = (await (await fetch(`${harness.baseUrl}/Items?${search}`, { headers })).json()).Items;
    const deleted = await fetch(`${harness.baseUrl}/Items/${movie.Id}`, { method: 'DELETE', headers });
    const kept = await (await fetch(`${harness.baseUrl}/Items/${movie.Id}?userId=${user}`, { headers })).json();

    expect(deleted.status).toBe(204);
    expect(movie.CanDelete).toBe(true);
    expect(kept.Name).toBe(harness.transcoded);

    await openQueue(page);
    const report = queued(page, harness.transcoded);
    await expect(report).toContainText('Unplayable');
    await expect(report).toContainText('Delete was pressed in a client');
  });
});
