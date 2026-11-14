const { test, expect } = require('./fixtures');

const CLIENT = 'MediaBrowser Client="Admin spec", Device="Playwright", DeviceId="admin-spec", Version="1"';

async function session(harness) {
  const response = await fetch(harness.baseUrl + '/Users/AuthenticateByName', {
    method: 'POST',
    headers: { Authorization: CLIENT, 'Content-Type': 'application/json' },
    body: JSON.stringify({ Username: harness.username, Pw: harness.password }),
  });
  const { AccessToken: token, User: user } = await response.json();

  return { user: user.Id, headers: { Authorization: `${CLIENT}, Token="${token}"`, 'Content-Type': 'application/json' } };
}

async function signIn(page, harness, password = harness.password) {
  await page.getByLabel('Username').fill(harness.username);
  await page.getByLabel('Password').fill(password);
  await page.getByRole('button', { name: 'Sign in', exact: true }).click();
}

test.describe('the admin app', () => {
  test('signs an administrator in and keeps them signed in', async ({ page, harness }) => {
    await page.goto(harness.baseUrl + '/admin');

    await signIn(page, harness, 'not-the-password');
    await expect(page.locator('.failure')).toHaveText('That username and password did not sign in.');

    await signIn(page, harness);
    await expect(page.getByRole('heading', { name: 'Requests and Reports' })).toBeVisible();
    await expect(page.locator('nav')).toContainText('Signed in as ' + harness.username);

    await page.goto(harness.baseUrl + '/admin/requests');
    await expect(page.getByRole('heading', { name: 'Requests and Reports' })).toBeVisible();

    await page.getByRole('button', { name: 'Sign out' }).click();
    await expect(page.getByLabel('Username')).toBeVisible();
  });

  test('closes a report somebody filed', async ({ page, harness }) => {
    const { user, headers } = await session(harness);
    const search = new URLSearchParams({ searchTerm: harness.playable, recursive: 'true', includeItemTypes: 'Movie', userId: user });
    const [movie] = (await (await fetch(`${harness.baseUrl}/Items?${search}`, { headers })).json()).Items;
    const filed = await fetch(harness.baseUrl + '/GoJellyfin/Reports', {
      method: 'POST',
      headers,
      body: JSON.stringify({ ItemId: movie.Id, Kind: 'Subtitles', Note: 'filed by the admin spec' }),
    });
    expect(filed.ok).toBe(true);

    await page.goto(harness.baseUrl + '/admin');
    await signIn(page, harness);
    await page.getByRole('tab', { name: 'Reports' }).click();

    const report = page.locator('tr', { hasText: 'filed by the admin spec' });
    await expect(report).toContainText(harness.playable);
    await expect(report).toContainText('Open');

    await report.getByRole('button', { name: 'Dismiss' }).click();
    await expect(report).toContainText('Dismissed');
    await expect(report.getByRole('button', { name: 'Dismiss' })).toHaveCount(0);
  });
});
