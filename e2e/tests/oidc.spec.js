const { test, expect, loadLogin, view } = require('./fixtures');

const TV = 'MediaBrowser Client="Android TV", Device="Living Room", DeviceId="e2e-tv", Version="1.0.0"';

test.describe('single sign-on', () => {
  // The page fixture refuses every origin but the client's, and the sign-in
  // has to be redirected through the provider and back.
  test.beforeEach(async ({ page, harness }) => {
    await page.route((url) => url.href.startsWith(harness.issuer), (route) => route.continue());
  });

  test('offers the button on the login page', async ({ page, harness }) => {
    await loadLogin(page, harness);

    await expect(page.getByRole('button', { name: 'Sign in with SSO' })).toBeVisible();
  });

  test('signs in as the account the provider vouched for', async ({ page, harness }) => {
    await signInWithSSO(page, harness);

    await expect(page).toHaveURL(/home\.html/, { timeout: 30000 });
    await expect(view(page)).toContainText('My Media');
    expect(await signedInAs(page, harness)).toBe(harness.ssoUsername);
  });

  test('approves a TV code typed into /link on a phone that was signed out', async ({ page, harness, playwright }) => {
    const tv = await playwright.request.newContext({ baseURL: harness.baseUrl, extraHTTPHeaders: { Authorization: TV } });
    const initiated = await tv.post('/QuickConnect/Initiate');
    expect(initiated.ok()).toBe(true);
    const { Code: code, Secret: secret } = await initiated.json();

    await page.goto(`${harness.baseUrl}/link/${code}`);
    await expect(page.getByRole('button', { name: 'Sign in with SSO' })).toBeVisible({ timeout: 30000 });
    await page.getByRole('button', { name: 'Sign in with SSO' }).click();

    await expect(page).toHaveURL(new RegExp(`#/quickconnect\\?code=${code}`), { timeout: 30000 });
    await expect(view(page).locator('#txtQuickConnectCode')).toHaveValue(code);
    await view(page).locator('button[type=submit]').click();

    await expect(async () => {
      const state = await tv.get(`/QuickConnect/Connect?secret=${secret}`);
      expect((await state.json()).Authenticated).toBe(true);
    }).toPass({ timeout: 15000 });

    const signedIn = await tv.post('/Users/AuthenticateWithQuickConnect', { data: { Secret: secret } });
    expect(signedIn.ok()).toBe(true);
    expect((await signedIn.json()).User.Name).toBe(harness.ssoUsername);

    await tv.dispose();
  });
});

async function signInWithSSO(page, harness) {
  await loadLogin(page, harness);
  await page.getByRole('button', { name: 'Sign in with SSO' }).click();
}

async function signedInAs(page, harness) {
  const token = await page.evaluate(() => {
    const credentials = JSON.parse(localStorage.getItem('jellyfin_credentials') || '{}');

    return (credentials.Servers || []).map((server) => server.AccessToken).find(Boolean);
  });
  const me = await page.request.get(`${harness.baseUrl}/Users/Me`, {
    headers: { Authorization: `MediaBrowser Token="${token}"` },
  });

  return (await me.json()).Name;
}
