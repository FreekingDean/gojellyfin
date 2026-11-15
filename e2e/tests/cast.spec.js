const { test, expect } = require('./fixtures');

const TELEVISION = 'MediaBrowser Client="Cast spec", Device="Television", DeviceId="cast-spec-television", Version="1"';

// jellyfin-web 12 builds the cast menu from the Chromecast receivers it found
// and from /Sessions?ControllableByUserId, and reads Capabilities.SupportedCommands
// off every session that answers. One session without it rejected the whole
// list, so the menu said "No cast targets available" even with a Chromecast on
// the network.
test('the cast menu is built without an error while another device is signed in', async ({ signedIn: page, harness }) => {
  const login = await fetch(harness.baseUrl + '/Users/AuthenticateByName', {
    method: 'POST',
    headers: { Authorization: TELEVISION, 'Content-Type': 'application/json' },
    body: JSON.stringify({ Username: harness.username, Pw: harness.password }),
  });
  expect(login.ok).toBeTruthy();

  const failures = [];
  page.on('console', (message) => {
    if (message.text().includes('unable to get playback targets')) failures.push(message.text());
  });

  const asked = page.waitForResponse((response) => /\/Sessions\?.*controllableByUserId=/i.test(response.url()));
  await page.getByRole('button', { name: 'Cast to Device' }).first().click();
  const response = await asked;

  expect(response.ok()).toBeTruthy();
  expect(await response.json()).toEqual([]);
  await expect(page.locator('#app-remote-play-menu')).toBeVisible();
  expect(failures).toEqual([]);
});
