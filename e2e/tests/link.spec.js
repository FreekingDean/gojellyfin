const { test, expect, view } = require('./fixtures');

const TV = 'MediaBrowser Client="Smoke TV", Device="Living Room", DeviceId="smoke-tv", Version="1.0.0"';

test.use({ viewport: { width: 390, height: 844 }, isMobile: true, hasTouch: true });

async function initiate(request, app) {
  const response = await request.post(app.baseUrl + '/QuickConnect/Initiate', { headers: { Authorization: TV } });
  expect(response.status()).toBe(200);

  return response.json();
}

async function authenticated(request, app, secret) {
  const response = await request.get(app.baseUrl + '/QuickConnect/Connect?secret=' + secret, {
    headers: { Authorization: TV },
  });
  expect(response.status()).toBe(200);

  return (await response.json()).Authenticated;
}

// A phone signs in with Remember Me left on, which is what lets a second
// visit to /link skip the login page: without it the client keeps the token
// for this page load only.
async function signInHere(page, app) {
  await expect(page).toHaveURL(/login\.html/, { timeout: 30000 });
  await page.locator('#loginPage button.card').filter({ hasText: app.username }).click();
  await page.locator('#txtManualPassword').fill(app.password);
  await page.locator('#loginPage .chkRememberLogin').evaluate((box) => {
    box.checked = true;
  });
  await page.locator('#loginPage button[type=submit]').click();
}

// The login route carries the way back as url=%2Fquickconnect, so only the
// unencoded slash says the client is on the page itself.
function codeField(page) {
  return page.locator('#quickConnectPreferencesPage #txtQuickConnectCode');
}

async function onQuickConnect(page) {
  await expect(page).toHaveURL(/#\/quickconnect/, { timeout: 30000 });
  await expect(codeField(page)).toBeVisible();
}

test.describe('linking a TV from a phone', () => {
  test('signs the phone in and returns it to the code the TV showed', async ({ page, request, harness }) => {
    const tv = await initiate(request, harness);

    await page.goto(harness.baseUrl + '/link/' + tv.Code);
    await signInHere(page, harness);
    await onQuickConnect(page);

    await expect(codeField(page)).toHaveValue(tv.Code);
    await expect(page).toHaveScreenshot('quickconnect-prefilled.png', { mask: [codeField(page)] });

    await view(page).locator('button[type=submit]').click();
    await expect(view(page)).toContainText('You have successfully authenticated your device!');

    expect(await authenticated(request, harness, tv.Secret)).toBe(true);
  });

  test('opens an empty code field when no code was typed, and skips the login once signed in', async ({
    page,
    request,
    harness,
  }) => {
    const tv = await initiate(request, harness);
    const spaced = tv.Code.slice(0, 3) + ' ' + tv.Code.slice(3);

    await page.goto(harness.baseUrl + '/link');
    await signInHere(page, harness);
    await onQuickConnect(page);

    await expect(codeField(page)).toHaveValue('');

    await page.goto(harness.baseUrl + '/link?code=' + encodeURIComponent(spaced));
    await onQuickConnect(page);

    await expect(page).not.toHaveURL(/login\.html/);
    await expect(codeField(page)).toHaveValue(tv.Code);
  });
});
