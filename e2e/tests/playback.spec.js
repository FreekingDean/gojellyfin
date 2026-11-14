const { test, expect, view, card, open } = require('./fixtures');

test.describe('playback', () => {
  let played;

  test('starts the video from the detail page', async ({ signedIn: page, harness }) => {
    const streams = [];
    page.on('response', (response) => {
      const stream = new URL(response.url()).pathname.match(/\/Videos\/([^/]+)\/stream/i);
      if (stream) {
        played = stream[1];
        streams.push(response.status());
      }
    });

    await open(page, harness.playable);
    await expect(view(page).locator('h1.itemName')).toHaveText(harness.playable);
    await view(page).locator('.btnPlay').first().click();

    const video = page.locator('video');
    await expect(video).toBeAttached({ timeout: 30000 });
    await expect
      .poll(() => video.evaluate((element) => element.currentTime), { timeout: 30000 })
      .toBeGreaterThan(1);
    expect(streams.length).toBeGreaterThan(0);
    expect(streams.every((status) => status === 200 || status === 206)).toBe(true);
  });

  // Playing the item leaves a resume position, which puts a Continue Watching
  // row on the home screen of every test that runs after this one.
  test.afterEach(async ({ page, harness }) => {
    if (!played) {
      return;
    }
    const { AccessToken: token, UserId: user } = await page.evaluate(
      () => JSON.parse(localStorage.getItem('jellyfin_credentials')).Servers[0],
    );
    await page.goto('about:blank');

    const response = await page.request.post(`${harness.baseUrl}/UserItems/${played}/UserData?userId=${user}`, {
      headers: { Authorization: `MediaBrowser Token="${token}"` },
      data: { PlaybackPositionTicks: 0, Played: false, PlayCount: 0, LastPlayedDate: null },
    });
    expect(response.ok()).toBe(true);
  });
});
