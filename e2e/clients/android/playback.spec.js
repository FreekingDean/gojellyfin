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
    phone.key('fastForward');
    await expect
      .poll(() => server.positionSeconds(harness.playable), { timeout: 90000, intervals: [2000] })
      .toBeGreaterThan(before + 20);
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

    await expect
      .poll(() => server.positionSeconds(harness.playable), { timeout: 120000, intervals: [2000] })
      .not.toBe(RESUME_AT);
    expect(await server.positionSeconds(harness.playable)).toBeGreaterThan(RESUME_AT - 15);
    await shot('resumed');

    await leavePlayer(phone);
  });
});
