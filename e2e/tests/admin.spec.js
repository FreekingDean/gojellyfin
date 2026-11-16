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
    await expect(page.getByRole('heading', { name: 'Overview' })).toBeVisible();
    await expect(page.locator('nav')).toContainText('Signed in as ' + harness.username);

    await page.goto(harness.baseUrl + '/admin/requests');
    await expect(page.getByRole('heading', { name: 'Requests and Reports' })).toBeVisible();

    await page.getByRole('button', { name: 'Sign out' }).click();
    await expect(page.getByLabel('Username')).toBeVisible();
  });

  test('says how the server is doing', async ({ page, harness }) => {
    await page.goto(harness.baseUrl + '/admin');
    await signIn(page, harness);

    const health = page.locator('.health');
    await expect(health.locator('div', { hasText: 'Database' })).toContainText('schema current');
    await expect(health.locator('div', { hasText: 'Background work' })).toContainText('Set TEMPORAL_HOSTPORT');
    await expect(health.locator('div', { hasText: 'Single sign-on' })).toContainText('127.0.0.1');
    await expect(health.locator('div', { hasText: 'Transcoding' })).toContainText('ffmpeg found');
    await expect(page.locator('main')).toContainText('Nobody is watching anything.');
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
    await page.getByRole('link', { name: 'Requests and Reports' }).click();
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

  test('adds a Radarr to Movies, tests it and deletes it', async ({ page, harness }) => {
    page.on('dialog', (dialog) => dialog.accept());
    const name = 'Admin spec Radarr';
    const local = harness.mediaDirectory + '/admin-spec';

    await page.goto(harness.baseUrl + '/admin');
    await signIn(page, harness);
    await page.getByRole('link', { name: 'Libraries' }).click();

    const movies = page.locator('section.library', { has: page.getByRole('heading', { name: 'Movies' }) });
    await expect(movies.locator('.card', { hasText: 'Fixtures' })).toContainText('SOURCE_API_KEY_FIXTURES set');
    await expect(page.getByRole('heading', { name: 'Shows' })).toBeVisible();

    await movies.getByRole('link', { name: 'Add Radarr' }).click();
    await page.getByLabel('Name').fill(name);
    await page.getByLabel('API key variable').selectOption('SOURCE_API_KEY_FIXTURES');
    await page.getByLabel('Address').fill(harness.downloaderUrl.replace('127.0.0.1', 'localhost'));
    await page.getByRole('button', { name: 'Test connection' }).click();
    await expect(page.getByText('Radarr ' + harness.downloaderVersion + ' answered')).toBeVisible();
    await page.getByLabel('Root folder in Radarr').selectOption(harness.downloaderRootFolder);
    await page.getByLabel('Path on this server').fill('/outside-every-media-directory');
    await page.getByRole('button', { name: 'Save' }).click();
    await expect(page.getByRole('alert')).toContainText('The downloader could not be saved. The server said:');
    await page.getByRole('alert').getByRole('button', { name: 'Dismiss' }).click();
    await expect(page.getByRole('alert')).toHaveCount(0);

    await page.getByLabel('Path on this server').fill(local);
    await page.getByRole('button', { name: 'Save' }).click();
    await expect(page.getByRole('status').filter({ hasText: name + ' added to' })).toBeVisible();

    const card = movies.locator('.card', { hasText: name });
    await expect(card).toContainText(harness.downloaderRootFolder + ' opened here as ' + local);
    await card.getByRole('button', { name: 'Test' }).click();
    await expect(card).toContainText('Radarr ' + harness.downloaderVersion);

    await card.getByRole('link', { name: 'Edit' }).click();
    await page.getByRole('button', { name: 'Delete downloader' }).click();
    await expect(page.getByRole('status').filter({ hasText: name + ' deleted.' })).toBeVisible();
    await expect(movies.locator('.card', { hasText: 'Fixtures' })).toBeVisible();
    await expect(movies.locator('.card', { hasText: name })).toHaveCount(0);
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

  test('shows what a session is playing, how it is served and where it connects from', async ({ page, harness }) => {
    const { user, headers } = await session(harness);
    const search = new URLSearchParams({ searchTerm: harness.playable, recursive: 'true', includeItemTypes: 'Movie', userId: user });
    const [movie] = (await (await fetch(`${harness.baseUrl}/Items?${search}`, { headers })).json()).Items;
    const report = (path, body) => fetch(harness.baseUrl + path, { method: 'POST', headers, body: JSON.stringify(body) });
    const profile = { DirectPlayProfiles: [{ Container: 'mp4', VideoCodec: 'vp9', AudioCodec: 'opus', Type: 'Video' }] };
    expect((await report(`/Items/${movie.Id}/PlaybackInfo`, { DeviceProfile: profile })).ok).toBe(true);
    expect((await report('/Sessions/Playing', { ItemId: movie.Id, PositionTicks: 0, PlayMethod: 'Transcode' })).ok).toBe(true);

    try {
      await page.goto(harness.baseUrl + '/admin');
      await signIn(page, harness);
      await expect(page.locator('main')).toContainText(harness.playable);

      await page.getByRole('link', { name: 'Activity' }).click();
      const row = page.locator('tr', { hasText: 'Playwright' }).filter({ hasText: harness.playable });
      await expect(row).toContainText('Remux');
      await expect(row).toContainText('WEBM → MP4');
      await expect(row).toContainText('WEBM container not supported');
      await expect(row).toContainText('127.0.0.1');
    } finally {
      await report('/Sessions/Playing/Stopped', { ItemId: movie.Id, PositionTicks: 0 });
    }
  });

  test('saves the sign-in message without the sign-on button', async ({ page, harness }) => {
    const { headers } = await session(harness);
    const stored = async () => (await (await fetch(harness.baseUrl + '/System/Configuration/branding', { headers })).json()).LoginDisclaimer ?? '';
    const before = await stored();

    await page.goto(harness.baseUrl + '/admin');
    await signIn(page, harness);
    await page.getByRole('link', { name: 'Settings' }).click();

    try {
      await page.getByLabel('Sign-in message').fill('Ask the admin spec for an account.');
      await page.getByRole('button', { name: 'Save branding' }).click();
      await expect(page.getByRole('status').filter({ hasText: 'Branding saved' })).toBeVisible();
      expect(await stored()).toBe('Ask the admin spec for an account.');

      const shown = await (await fetch(harness.baseUrl + '/Branding/Configuration')).json();
      expect(shown.LoginDisclaimer).toContain('Ask the admin spec for an account.');
      expect(shown.LoginDisclaimer).toContain('/sso/oidc/start');
    } finally {
      await fetch(harness.baseUrl + '/System/Configuration/branding', {
        method: 'POST',
        headers,
        body: JSON.stringify({ ...(await (await fetch(harness.baseUrl + '/System/Configuration/branding', { headers })).json()), LoginDisclaimer: before }),
      });
    }
  });

  test('makes an API key, shows it once and revokes it', async ({ page, harness }) => {
    await page.goto(harness.baseUrl + '/admin');
    await signIn(page, harness);
    await page.getByRole('link', { name: 'Settings' }).click();

    await page.getByRole('button', { name: 'New key' }).click();
    await page.getByLabel('Which app is it for').fill('Admin spec app');
    await page.getByRole('button', { name: 'Make key' }).click();
    const made = page.getByRole('status').filter({ hasText: 'The key for Admin spec app is' });
    await expect(made).toBeVisible();
    const token = (await made.locator('code').textContent()).trim();
    expect(token.length).toBeGreaterThan(16);

    const row = page.locator('tr', { hasText: 'Admin spec app' });
    await row.getByRole('button', { name: 'Revoke' }).click();
    await expect(row).toHaveCount(0);
    await expect(made).toHaveCount(0);
  });
});
