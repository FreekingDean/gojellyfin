const { test, expect, leavePlayer } = require('./fixtures');
const { sleep } = require('../adb');

const SEEDED_AT = 5;
const LEFT_TO_PLAY = 20;

test.describe('watching a series on the television', () => {
  test.afterEach(async ({ server, harness }) => {
    const [first, second] = harness.episodes;
    await server.leaveAt(first, SEEDED_AT);
    await server.forget(second);
  });

  // The first episode is left close to its end, so playing it from the home
  // screen's Continue Watching row runs out in seconds and the app has to
  // carry on to the second by itself.
  test('moves on to the next episode when one ends', async ({ signedIn: tv, harness, server, shot }) => {
    const [first, second] = harness.episodes;
    await server.leaveAt(first, harness.mediaSeconds - LEFT_TO_PLAY);
    await server.forget(second);

    // A tap on a card only moves the focus onto it, and the card grows as it
    // takes it, so the remote walks there: down to Continue Watching, then
    // along it to the episode, which the seed leaves third.
    await tv.waitFor({ label: first });
    for (const direction of ['down', 'right', 'right']) {
      tv.key(direction);
      await sleep(1000);
    }
    await shot('continue-watching');
    tv.key('center');
    await tv.tapOn({ label: /^Resume from / }, { timeout: 60000 });
    await shot('first-episode');

    await expect
      .poll(() => server.positionSeconds(second), { timeout: 240000, intervals: [3000] })
      .toBeGreaterThan(1);
    await shot('second-episode');

    await leavePlayer(tv);
  });
});
