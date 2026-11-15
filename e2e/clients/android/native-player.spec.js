const { test, expect, openDetails, leavePlayer } = require('./fixtures');

// The app's integrated player is ExoPlayer rather than jellyfin-web, and it
// builds its own url from what PlaybackInfo answers. It refuses a transcode
// that is not HLS by throwing, so this is what crashed the app on every play.
// Each title takes a different answer: the WebM plays as the file, the
// MPEG-4 Part 2 has its picture encoded, and an episode, H.264 with E-AC-3
// in Matroska, has its sound converted, which the emulator cannot decode.
async function openEpisode(phone, harness, name) {
  await phone.tapOn({ label: harness.shows });
  await phone.tapOn({ label: harness.series }, { timeout: 60000 });
  await phone.tapOn({ label: harness.season }, { timeout: 60000 });
  await phone.tapOn({ label: new RegExp(`^(\\d+\\. )?${name}$`) }, { timeout: 60000 });
  await phone.waitFor({ label: /^(Play|Resume)$/i }, { timeout: 60000 });
}

async function playsWithoutClosing(phone, server, name, shot) {
  await phone.tapOn({ label: /^(Play|Resume)$/i });

  await expect
    .poll(() => server.positionSeconds(name), { timeout: 120000, intervals: [2000] })
    .toBeGreaterThan(5);
  await shot('native-playing');

  expect(phone.shell('pidof org.jellyfin.mobile', { allowFailure: true }).trim()).not.toBe('');

  await leavePlayer(phone);
}

test.describe("playing on the phone app's own player", () => {
  test('plays a title as the file', async ({ nativePlayer: phone, harness, server, shot }) => {
    await server.forget(harness.playable);
    await openDetails(phone, harness, harness.playable);
    await playsWithoutClosing(phone, server, harness.playable, shot);
    await server.forget(harness.playable);
  });

  test('plays a title whose picture the server encodes', async ({ nativePlayer: phone, harness, server, shot }) => {
    await server.forget(harness.transcoded);
    await openDetails(phone, harness, harness.transcoded);
    await playsWithoutClosing(phone, server, harness.transcoded, shot);
    await server.forget(harness.transcoded);
  });

  test('plays an episode whose sound the server converts', async ({ nativePlayer: phone, harness, server, shot }) => {
    const episode = harness.episodes[1];
    await server.forget(episode);
    await openEpisode(phone, harness, episode);
    await playsWithoutClosing(phone, server, episode, shot);
    await server.forget(episode);
  });
});
