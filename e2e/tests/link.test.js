const ui = require('../setup/ui');

const TV = 'MediaBrowser Client="Smoke TV", Device="Living Room", DeviceId="smoke-tv", Version="1.0.0"';

async function initiate() {
  const response = await fetch(ui.harness().baseUrl + '/QuickConnect/Initiate', {
    method: 'POST',
    headers: { Authorization: TV },
  });
  expect(response.status).toBe(200);

  return response.json();
}

async function state(secret) {
  const response = await fetch(ui.harness().baseUrl + '/QuickConnect/Connect?secret=' + secret, {
    headers: { Authorization: TV },
  });
  expect(response.status).toBe(200);

  return response.json();
}

// A phone signs in with Remember Me left on, which is what lets a second
// visit to /link skip the login page: without it the client keeps the token
// for this page load only.
async function signInHere(page) {
  await page.waitForSelector('#loginPage button.card', { visible: true });
  await page.click('#loginPage button.card');
  await page.waitForSelector('#txtManualPassword', { visible: true });
  await page.type('#txtManualPassword', ui.harness().password);
  await page.$eval('#loginPage .chkRememberLogin', (box) => {
    box.checked = true;
  });
  await page.click('#loginPage button[type=submit]');
}

// The login route carries the way back as url=%2Fquickconnect, so only the
// unencoded slash says the client is on the page itself.
async function onQuickConnect(page) {
  await ui.waitForRoute(page, '#/quickconnect');
}

async function codeField(page) {
  await page.waitForSelector('#quickConnectPreferencesPage #txtQuickConnectCode', { visible: true });

  return page.$eval('#quickConnectPreferencesPage #txtQuickConnectCode', (input) => input.value);
}

describe('linking a TV from a phone', () => {
  let session;

  beforeEach(async () => {
    session = await ui.open();
    await session.page.setViewport({ width: 390, height: 844, isMobile: true, hasTouch: true });
  });

  afterEach(async () => {
    await session.close();
  });

  it('signs the phone in and returns it to the code the TV showed', async () => {
    const tv = await initiate();
    const { page } = session;

    await page.goto(ui.harness().baseUrl + '/link/' + tv.Code, { waitUntil: 'networkidle2' });
    await ui.waitForRoute(page, 'login.html');
    await signInHere(page);
    await onQuickConnect(page);

    expect(await codeField(page)).toBe(tv.Code);

    await page.click('#quickConnectPreferencesPage button[type=submit]');
    await ui.waitForText(page, 'You have successfully authenticated your device!', '#quickConnectPreferencesPage');

    expect((await state(tv.Secret)).Authenticated).toBe(true);
  });

  it('opens an empty code field when no code was typed, and skips the login once signed in', async () => {
    const tv = await initiate();
    const { page } = session;
    const spaced = tv.Code.slice(0, 3) + ' ' + tv.Code.slice(3);

    await page.goto(ui.harness().baseUrl + '/link', { waitUntil: 'networkidle2' });
    await ui.waitForRoute(page, 'login.html');
    await signInHere(page);
    await onQuickConnect(page);

    expect(await codeField(page)).toBe('');

    await page.goto(ui.harness().baseUrl + '/link?code=' + encodeURIComponent(spaced), { waitUntil: 'networkidle2' });
    await onQuickConnect(page);

    expect(page.url()).not.toContain('login.html');
    expect(await codeField(page)).toBe(tv.Code);
  });
});
