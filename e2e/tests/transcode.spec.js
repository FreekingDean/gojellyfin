const { test, expect, view, open } = require('./fixtures');

test.describe('transcoding', () => {
  let played;

  test('encodes a picture the browser cannot decode to 1080p H.264 as HLS and plays it', async ({ signedIn: page, harness }) => {
    const playlists = [];
    const segments = [];
    page.on('response', (response) => {
      const url = new URL(response.url());
      const master = url.pathname.match(/\/Videos\/([^/]+)\/master\.m3u8$/i);
      if (master) {
        played = master[1];
        playlists.push({ url, status: response.status(), type: response.headers()['content-type'] });
      }
      if (/\/Videos\/[^/]+\/hls1\/main\/\d+\.mp4$/i.test(url.pathname)) {
        segments.push({ url, status: response.status(), type: response.headers()['content-type'] });
      }
    });

    await open(page, harness.transcoded);
    await expect(view(page).locator('h1.itemName')).toHaveText(harness.transcoded);
    await view(page).locator('.btnPlay').first().click();

    const video = page.locator('video');
    await expect(video).toBeAttached({ timeout: 30000 });
    await expect
      .poll(() => video.evaluate((element) => element.currentTime), { timeout: 45000 })
      .toBeGreaterThan(1);

    expect(playlists.length).toBeGreaterThan(0);
    const [master] = playlists;
    expect(master.status).toBe(200);
    expect(master.type).toBe('application/vnd.apple.mpegurl');
    expect(master.url.searchParams.get('videoCodec')).toBe('h264');
    expect(master.url.searchParams.get('maxHeight')).toBe('1080');

    expect(segments.length).toBeGreaterThan(0);
    expect(segments[0].status).toBe(200);
    expect(segments[0].type).toBe('video/mp4');

    const size = await video.evaluate((element) => ({ width: element.videoWidth, height: element.videoHeight }));
    expect(size).toEqual({ width: 1440, height: 1080 });
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
