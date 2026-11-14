const { execFileSync } = require('node:child_process');

const { test, expect, view } = require('./fixtures');

// Named to sort last: the migration adds Jellyfin's users to the database every
// other spec shares, and the login page lists them.
test.describe('taking over from Jellyfin', () => {
  test.beforeAll(({ harness }) => {
    const { command, args, env } = harness.jellyfin.migrate;
    execFileSync(command, args, { env: { ...process.env, ...env }, encoding: 'utf8' });
  });

  // What a browser that was signed in to the old server holds: jellyfin-web
  // files the token under the server's id, and its device id beside it.
  test('a browser signed in to Jellyfin is still signed in', async ({ page, harness }) => {
    const { serverId, userId, token, deviceId } = harness.jellyfin;

    await page.addInitScript(
      ({ serverId, userId, token, deviceId, address }) => {
        localStorage.setItem('_deviceId2', deviceId);
        localStorage.setItem(
          'jellyfin_credentials',
          JSON.stringify({
            Servers: [
              {
                Id: serverId,
                Name: 'Jellyfin',
                ManualAddress: address,
                LastConnectionMode: 2,
                DateLastAccessed: Date.now(),
                AccessToken: token,
                UserId: userId.replaceAll('-', ''),
              },
            ],
          }),
        );
      },
      { serverId, userId, token, deviceId, address: harness.baseUrl },
    );

    await page.goto(harness.baseUrl + '/web/');

    await expect(page).toHaveURL(/home\.html/, { timeout: 30000 });
    await expect(view(page)).toContainText('My Media');
    await expect(page.locator('#loginPage')).toHaveCount(0);

    const signedInAs = await page.evaluate(() => window.ApiClient.getCurrentUserId());
    expect(signedInAs.replaceAll('-', '').toLowerCase()).toBe(userId.replaceAll('-', ''));
  });
});
