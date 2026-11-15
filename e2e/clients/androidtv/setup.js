const fs = require('node:fs');
const path = require('node:path');

const { boot } = require('../../setup/boot');
const { HANDOFF } = require('../../setup/harness');
const { device } = require('../adb');
const { PACKAGE, apk } = require('./app');

// Long enough that a thirty second skip lands in the middle rather than at
// the end, and that a resume point is neither the start nor "watched".
const MEDIA_SECONDS = 180;

module.exports = async function globalSetup() {
  const tv = device();
  tv.adb(['wait-for-device']);

  const app = await boot({ mediaSeconds: MEDIA_SECONDS, playableEpisodes: true });

  // The emulator's own 127.0.0.1 is forwarded to the harness, so the app is
  // given the same address the server publishes and the posters live at.
  tv.reverse(app.facts.port);
  tv.install(await apk());
  tv.clear(PACKAGE);

  fs.mkdirSync(path.dirname(HANDOFF), { recursive: true });
  fs.writeFileSync(HANDOFF, JSON.stringify({ ...app.facts, mediaSeconds: MEDIA_SECONDS }));

  return async () => {
    await app.stop();
  };
};
