const fs = require('node:fs');

const { test, expect, leavePlayer } = require('./fixtures');
const { SERVER_LOG } = require('../../setup/boot');
const { sleep } = require('../adb');

const SEEDED_AT = 5;
const LEFT_TO_PLAY = 20;

test.describe('watching a series on the television', () => {
  test.afterEach(async ({ tv, server, harness }, testInfo) => {
    const logcat = fs.readFileSync(tv.logcat(testInfo.outputPath('series-logcat.txt')), 'utf8');
    const player = /PlaybackController|VideoManager|ExoPlayer|PlaybackManager|NextUp|AndroidRuntime|FATAL|MediaCodec/;
    console.log(`What the player logged:\n${logcat.split('\n').filter((line) => player.test(line)).slice(-200).join('\n')}`);
    const served = /Videos|PlaybackInfo|Playing|transcode|ffmpeg/i;
    console.log(`What the server logged:\n${fs.readFileSync(SERVER_LOG, 'utf8').split('\n').filter((line) => served.test(line)).slice(-80).join('\n')}`);
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
    expect(fs.readFileSync(SERVER_LOG, 'utf8')).toMatch(/\/stream\.mp4/);

    await leavePlayer(tv);
  });
});
