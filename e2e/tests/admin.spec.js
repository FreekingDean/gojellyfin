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

  test('adds a user, limits what they can ask for and deletes them', async ({ page, harness }) => {
    const name = 'admin-spec-' + Date.now();
    page.on('dialog', (dialog) => dialog.accept());

    await page.goto(harness.baseUrl + '/admin');
    await signIn(page, harness);
    await page.getByRole('link', { name: 'Users' }).click();
    await expect(page.locator('tr', { hasText: harness.username })).toContainText('Admin');

    await page.getByRole('link', { name: 'Add user' }).click();
    await page.getByLabel('Name').fill(name);
    await page.getByLabel('Password').fill('admin-spec-password');
    await page.getByRole('button', { name: 'Add user' }).click();
    await expect(page.getByRole('heading', { name })).toBeVisible();

    await page.getByLabel('An administrator approves each request first').check();
    await page.getByLabel('Most titles in seven days').fill('3');
    await page.getByRole('button', { name: 'Save' }).click();

    const row = page.locator('tr', { hasText: name });
    await expect(row).toContainText('0 of 3 this week');
    await expect(row).toContainText('Needs approval');

    await row.getByRole('link', { name: 'Edit' }).click();
    await page.getByRole('button', { name: 'Delete user' }).click();
    await expect(page.locator('tr', { hasText: harness.username })).toBeVisible();
    await expect(page.locator('tr', { hasText: name })).toHaveCount(0);
  });

  test('shows who is signed in, what happened and the background jobs', async ({ page, harness }) => {
    await page.goto(harness.baseUrl + '/admin');
    await signIn(page, harness);

    await page.getByRole('link', { name: 'Activity' }).click();
    await expect(page.locator('tr', { hasText: 'gojellyfin admin' }).first()).toContainText(harness.username);
    await page.getByRole('tab', { name: 'Log' }).click();
    await expect(page.getByText(harness.username + ' has been authenticated').first()).toBeVisible();

    await page.getByRole('link', { name: 'Jobs' }).click();
    const job = page.locator('tr', { hasText: 'Refresh libraries' });
    await expect(job).toContainText('Idle');
    await job.getByRole('button', { name: 'Start' }).click();
    await expect(page.getByRole('alert')).toContainText('Refresh libraries could not be started');
  });

  test('turns a shelf on and shows the environment with secrets withheld', async ({ page, harness }) => {
    await page.goto(harness.baseUrl + '/admin');
    await signIn(page, harness);
    await page.getByRole('link', { name: 'Settings' }).click();

    await expect(page.getByLabel('Trending Movies')).toBeChecked();
    await expect(page.getByLabel('Hidden Gems')).not.toBeChecked();
    await page.getByLabel('Hidden Gems').check();
    await page.getByRole('button', { name: 'Save shelves' }).click();
    await expect(page.getByRole('status')).toContainText('Shelves saved');

    await page.reload();
    await expect(page.getByLabel('Hidden Gems')).toBeChecked();
    await page.getByLabel('Hidden Gems').uncheck();
    await page.getByRole('button', { name: 'Save shelves' }).click();
    await expect(page.getByRole('status')).toContainText('Shelves saved');

    const main = page.locator('main');
    await expect(main.locator('tr', { hasText: 'DATABASE_URL' })).toContainText('Set');
    await expect(main.locator('tr', { hasText: 'SOURCE_API_KEY_FIXTURES' })).toContainText('Set');
    await expect(main).not.toContainText('postgres://');
  });
});
