const { test, expect, view, open } = require('./fixtures');

test.describe('intro', () => {
  let played;

  test('offers to skip the intro and jumps past it', async ({ signedIn: page, harness }) => {
    page.on('response', (response) => {
      const stream = new URL(response.url()).pathname.match(/\/Videos\/([^/]+)\/stream/i);
      if (stream) {
        played = stream[1];
      }
    });

    await open(page, harness.playable);
    await view(page).locator('.btnPlay').first().click();

    const video = page.locator('video');
    await expect(video).toBeAttached({ timeout: 30000 });

    const skip = page.locator('.skip-button');
    await expect(skip).toBeVisible({ timeout: 30000 });
    await expect(skip).toContainText('Skip Intro');
    const at = await video.evaluate((element) => element.currentTime);
    expect(at).toBeGreaterThanOrEqual(harness.intro.start - 0.5);
    expect(at).toBeLessThan(harness.intro.end);

    await skip.click();
    await expect
      .poll(() => video.evaluate((element) => element.currentTime), { timeout: 15000 })
      .toBeGreaterThanOrEqual(harness.intro.end - 0.5);
  });

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
