const { test, expect, view, open } = require('./fixtures');

const TICKS = 10_000_000;

function itemId(page) {
  return new URL(page.url().replace('#', '')).searchParams.get('id');
}

const CLIENT = 'MediaBrowser Client="Jellyfin Web", Device="Chrome", DeviceId="seek-spec", Version="12.4"';

async function authorization(page, harness) {
  const login = await page.request.post(harness.baseUrl + '/Users/AuthenticateByName', {
    headers: { Authorization: CLIENT },
    data: { Username: harness.username, Pw: harness.password },
  });
  expect(login.ok()).toBe(true);
  const { AccessToken: token } = await login.json();

  return { Authorization: `${CLIENT}, Token="${token}"` };
}

async function setPosition(page, harness, id, seconds) {
  const response = await page.request.post(harness.baseUrl + `/UserItems/${id}/UserData`, {
    headers: await authorization(page, harness),
    data: { PlaybackPositionTicks: Math.round(seconds * TICKS) },
  });
  expect(response.ok()).toBe(true);
}

async function playing(page) {
  const video = page.locator('video');
  await expect(video).toBeAttached({ timeout: 30000 });
  await expect.poll(() => video.evaluate((element) => !element.paused && element.readyState >= 3), { timeout: 30000 }).toBe(true);
  return video;
}

function progress(page) {
  return page
    .waitForRequest((request) => /\/Sessions\/Playing\/Progress/i.test(request.url()) && request.method() === 'POST', { timeout: 30000 })
    .then((request) => request.postDataJSON().PositionTicks / TICKS);
}

test.describe('seeking', () => {
  test('a seek on the bar moves the picture and the clock together', async ({ signedIn: page, harness }) => {
    await open(page, harness.playable);
    await expect(view(page).locator('h1.itemName')).toHaveText(harness.playable);
    await view(page).locator('.btnPlay').first().click();

    const video = await playing(page);
    const duration = await video.evaluate((element) => element.duration);

    await page.mouse.move(640, 450);
    const slider = page.locator('.osdPositionSlider');
    await expect(slider).toBeVisible();
    await slider.evaluate((element) => {
      element.value = 60;
      element.dispatchEvent(new Event('change', { bubbles: true }));
    });

    const target = duration * 0.6;
    await expect.poll(() => video.evaluate((element) => element.currentTime), { timeout: 15000 }).toBeGreaterThan(target - 1);

    const reported = progress(page);
    await page.keyboard.press('k');
    const at = await video.evaluate((element) => element.currentTime);
    expect(Math.abs((await reported) - at)).toBeLessThan(1.5);
    expect(at).toBeLessThan(target + 3);
  });

  test('a resume starts where it stopped and reports that position, not twice it', async ({ signedIn: page, harness }) => {
    await open(page, harness.playable);
    await expect(page).toHaveURL(/[?&]id=/);
    const id = itemId(page);
    const resumeAt = 8;

    try {
      await setPosition(page, harness, id, resumeAt);
      await page.reload();
      const resume = view(page).getByRole('button', { name: 'Resume' });
      await expect(resume).toBeVisible({ timeout: 30000 });
      await resume.click();

      const video = await playing(page);
      await expect.poll(() => video.evaluate((element) => element.currentTime), { timeout: 15000 }).toBeGreaterThan(resumeAt - 0.5);

      const reported = progress(page);
      await page.mouse.move(640, 450);
      await page.keyboard.press('k');
      const at = await video.evaluate((element) => element.currentTime);
      const position = await reported;
      expect(at).toBeLessThan(resumeAt + 5);
      expect(Math.abs(position - at)).toBeLessThan(1.5);
    } finally {
      await setPosition(page, harness, id, 0);
    }
  });

  test('the client may stop the encode a seek replaces', async ({ page, harness }) => {
    const headers = await authorization(page, harness);

    const stop = await page.request.delete(harness.baseUrl + '/Videos/ActiveEncodings?PlaySessionId=seek', {
      headers,
    });
    expect(stop.status()).toBe(204);
  });
});
