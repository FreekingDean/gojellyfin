const { test, expect, view, card, open } = require('./fixtures');

test.describe('playback', () => {
  test('starts the video from the detail page', async ({ signedIn: page, harness }) => {
    const streams = [];
    page.on('response', (response) => {
      if (/\/Videos\/[^/]+\/stream/i.test(new URL(response.url()).pathname)) {
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
});
