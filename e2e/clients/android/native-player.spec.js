const { test, expect, openDetails, leavePlayer } = require('./fixtures');

// The app's integrated player is ExoPlayer rather than jellyfin-web, and it
// builds its own url from what PlaybackInfo answers. It refuses a transcode
// that is not HLS by throwing, so this is what crashed the app on every play.
test.describe("playing on the phone app's own player", () => {
  test.afterEach(async ({ server, harness }) => {
    await server.forget(harness.playable);
    await server.forget(harness.transcoded);
  });

  test('plays a title without the app closing', async ({ nativePlayer: phone, harness, server, shot }) => {
    await server.forget(harness.playable);
    await openDetails(phone, harness, harness.playable);
    await phone.tapOn({ label: /^Play$/i });

    await expect
      .poll(() => server.positionSeconds(harness.playable), { timeout: 120000, intervals: [2000] })
      .toBeGreaterThan(5);
    await shot('native-playing');

    expect(phone.shell('pidof org.jellyfin.mobile', { allowFailure: true }).trim()).not.toBe('');

    await leavePlayer(phone);
  });

  test('plays a title it has to encode, which reaches it as HLS', async ({ nativePlayer: phone, harness, server, shot }) => {
    await server.forget(harness.transcoded);
    await openDetails(phone, harness, harness.transcoded);
    await phone.tapOn({ label: /^Play$/i });

    await expect
      .poll(() => server.positionSeconds(harness.transcoded), { timeout: 120000, intervals: [2000] })
      .toBeGreaterThan(5);
    await shot('native-playing-hls');

    expect(phone.shell('pidof org.jellyfin.mobile', { allowFailure: true }).trim()).not.toBe('');

    await leavePlayer(phone);
  });
});
