const { test, expect, view } = require('./fixtures');

async function credentials(page) {
  return page.evaluate(() => JSON.parse(localStorage.getItem('jellyfin_credentials')).Servers[0]);
}

async function idOf(page, harness, headers, name) {
  const response = await page.request.get(`${harness.baseUrl}/Items`, {
    headers,
    params: { searchTerm: name, recursive: true, includeItemTypes: 'Movie' },
  });
  expect(response.ok()).toBe(true);
  const { Items: found } = await response.json();
  const item = found.find((each) => each.Name === name);
  expect(item, `${name} is in the library`).toBeTruthy();

  return item.Id;
}

test.describe('live tv', () => {
  let channel;
  let headers;

  test.beforeEach(async ({ signedIn: page, harness }) => {
    const { AccessToken: token } = await credentials(page);
    headers = { Authorization: `MediaBrowser Token="${token}"` };

    const titles = [await idOf(page, harness, headers, harness.playable), await idOf(page, harness, headers, harness.transcoded)];
    const response = await page.request.post(`${harness.baseUrl}/GoJellyfin/Channels`, {
      headers,
      data: { Number: 42, Name: 'Fixture TV', Order: 'Sequential', ItemIds: titles },
    });
    expect(response.ok(), await response.text()).toBe(true);
    channel = await response.json();
  });

  test.afterEach(async ({ page, harness }) => {
    if (!channel) {
      return;
    }
    const response = await page.request.delete(`${harness.baseUrl}/GoJellyfin/Channels/${channel.Id}`, { headers });
    expect(response.ok()).toBe(true);
    channel = undefined;
  });

  test('plays a channel live from one title into the next', async ({ signedIn: page, harness }) => {
    const playlists = [];
    page.on('response', (response) => {
      const url = new URL(response.url());
      if (url.pathname.toLowerCase() === `/videos/${channel.Id}/main.m3u8`) {
        playlists.push(response);
      }
    });

    await page.goto(`${harness.baseUrl}/web/#/details?id=${channel.Id}`);
    await expect(view(page).locator('h1.itemName')).toContainText('Fixture TV');
    await view(page).locator('.btnPlay').first().click();

    const video = page.locator('video');
    await expect(video).toBeAttached({ timeout: 30000 });
    await expect
      .poll(() => video.evaluate((element) => element.currentTime), { timeout: 60000, intervals: [1000] })
      .toBeGreaterThan(24);

    expect(playlists.length).toBeGreaterThan(1);
    const listed = await playlists[playlists.length - 1].text();
    expect(listed).not.toContain('#EXT-X-ENDLIST');
    expect(listed).toContain('#EXT-X-DISCONTINUITY');
  });
});
