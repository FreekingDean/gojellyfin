const { test, expect, openDetails, leavePlayer } = require('./fixtures');

const RESUME_AT = 60;

test.describe('playing on a phone', () => {
  test.afterEach(async ({ server, harness }) => {
    await server.forget(harness.playable);
  });

  test('plays a title and skips ahead in it', async ({ signedIn: phone, harness, server, shot }) => {
    await server.forget(harness.playable);
    await openDetails(phone, harness, harness.playable);
    await shot('details');
    await phone.tapOn({ label: /^Play$/i });

    await expect
      .poll(() => server.positionSeconds(harness.playable), { timeout: 120000, intervals: [2000] })
      .toBeGreaterThan(1);
    await shot('playing');

    const before = await server.positionSeconds(harness.playable);
    // The page skips ahead on the arrow key, the way it does on a keyboard.
    // Taking the time played since off the position is what keeps ordinary
    // playback from passing for a skip.
    const pressed = Date.now();
    phone.key('right');
    await expect
      .poll(async () => (await server.positionSeconds(harness.playable)) - (Date.now() - pressed) / 1000, {
        timeout: 60000,
        intervals: [2000],
      })
      .toBeGreaterThan(before + 15);
    await shot('after-skip');

    await leavePlayer(phone);
    const stopped = await server.positionSeconds(harness.playable);
    expect(stopped).toBeGreaterThan(20);
    expect(stopped).toBeLessThan(harness.mediaSeconds);
  });

  test('resumes a title where it was left', async ({ signedIn: phone, harness, server, shot }) => {
    await server.leaveAt(harness.playable, RESUME_AT);
    await openDetails(phone, harness, harness.playable);
    await phone.waitFor({ label: /^Resume$/i });
    await shot('details-resume');
    await phone.tapOn({ label: /^Resume$/i });

    // The player's first report can be the 0 it held before seeking, so what
    // proves the resume is passing the point it was left at sooner than a
    // play from the start could.
    await expect
      .poll(() => server.positionSeconds(harness.playable), { timeout: (RESUME_AT - 15) * 1000, intervals: [2000] })
      .toBeGreaterThan(RESUME_AT);
    await shot('resumed');

    await leavePlayer(phone);
  });
});
