const { test, expect, openDetails, leavePlayer } = require('./fixtures');
const { sleep } = require('../adb');

test.describe('watching a series on the television', () => {
  test.afterEach(async ({ server, harness }) => {
    for (const episode of harness.episodes) {
      await server.forget(episode);
    }
  });

  test('moves on to the next episode when one ends', async ({ signedIn: tv, harness, server, shot }) => {
    const [first, second] = harness.episodes;
    for (const episode of harness.episodes) {
      await server.forget(episode);
    }

    await openDetails(tv, { ...harness, library: harness.shows }, harness.series);
    await shot('series');
    await tv.tapOn({ label: /^(Play|Resume from .*)$/ });

    await expect
      .poll(() => server.positionSeconds(first), { timeout: 120000, intervals: [2000] })
      .toBeGreaterThan(1);
    await shot('first-episode');

    // Skipping to the end, one thirty second jump at a time, since the player
    // drops a skip asked for while the last one is still seeking.
    for (let jump = 0; jump < 12 && (await server.positionSeconds(second)) === 0; jump++) {
      tv.key('fastForward');
      await sleep(5000);
    }
    await shot('after-first-episode');

    await expect
      .poll(() => server.positionSeconds(second), { timeout: 120000, intervals: [2000] })
      .toBeGreaterThan(1);
    await shot('second-episode');

    await leavePlayer(tv);
  });
});
