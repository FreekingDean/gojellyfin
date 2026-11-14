const { test, expect, view, loadLogin } = require('./fixtures');

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

// The TV is a second browser: jellyfin-web on a television, which is what the
// webOS app is. Its own context keeps its cookies apart from the phone's.
async function television(browser, app) {
  const context = await browser.newContext({ viewport: { width: 1280, height: 720 } });
  const tv = await context.newPage();
  await tv.route((url) => !url.href.startsWith(app.baseUrl), (route) => route.abort());
  await loadLogin(tv, app);

  return { context, tv };
}

test.describe('scanning the code off the TV', () => {
  test('draws a QR beside the code that signs the TV in from a phone', async ({ page, browser, harness }) => {
    const { context, tv } = await television(browser, harness);

    await tv.locator('#loginPage .btnQuick').click();
    const dialog = tv.locator('#quickConnectAlert');
    await expect(dialog.locator('.text')).toContainText(/Enter code \d{6} to login/);
    const code = (await dialog.locator('.text').innerText()).match(/\d{6}/)[0];

    const qr = await dialog.locator('.dialogContentInner').evaluate((inner) => {
      const drawn = getComputedStyle(inner, '::after');

      return { image: drawn.backgroundImage, width: parseFloat(drawn.width) };
    });
    expect(qr.image).toContain('/link/qr.svg');
    expect(qr.width).toBeGreaterThan(150);
    await expect(tv).toHaveScreenshot('tv-quickconnect-qr.png', { mask: [dialog.locator('.dialogContentInner')] });

    const scanned = (await context.cookies(harness.baseUrl + '/link/qr.svg')).find(
      (cookie) => cookie.name === 'gojellyfin_quickconnect',
    );
    expect(scanned).toBeTruthy();

    await page.goto(harness.baseUrl + '/link/tv/' + scanned.value);
    await signInHere(page, harness);
    await onQuickConnect(page);
    await expect(codeField(page)).toHaveValue(code);

    await view(page).locator('button[type=submit]').click();
    await expect(view(page)).toContainText('You have successfully authenticated your device!');

    await expect(tv).toHaveURL(/home\.html/, { timeout: 30000 });
    await context.close();
  });

  test('follows the TV to a second code without a new QR', async ({ page, browser, harness }) => {
    const { context, tv } = await television(browser, harness);
    const dialog = tv.locator('#quickConnectAlert');

    await tv.locator('#loginPage .btnQuick').click();
    await expect(dialog.locator('.text')).toContainText(/\d{6}/);
    const first = (await dialog.locator('.text').innerText()).match(/\d{6}/)[0];
    await dialog.locator('button').click();
    await expect(dialog).toBeHidden();

    await tv.locator('#loginPage .btnQuick').click();
    await expect(dialog.locator('.text')).toContainText(/\d{6}/);
    const second = (await dialog.locator('.text').innerText()).match(/\d{6}/)[0];
    expect(second).not.toBe(first);

    const scanned = (await context.cookies(harness.baseUrl + '/link/qr.svg')).find(
      (cookie) => cookie.name === 'gojellyfin_quickconnect',
    );
    await page.goto(harness.baseUrl + '/link/tv/' + scanned.value);
    await signInHere(page, harness);
    await onQuickConnect(page);

    await expect(codeField(page)).toHaveValue(second);
    await context.close();
  });

  test('offers native TV apps a splashscreen carrying the same QR', async ({ request, harness }) => {
    const branding = await request.get(harness.baseUrl + '/Branding/Configuration');
    expect((await branding.json()).SplashscreenEnabled).toBe(true);

    const splash = await request.get(harness.baseUrl + '/Branding/Splashscreen', { headers: { Authorization: TV } });
    expect(splash.status()).toBe(200);
    expect(splash.headers()['content-type']).toBe('image/png');
  });
});

