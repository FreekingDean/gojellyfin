const fs = require('node:fs');
const path = require('node:path');

const { boot } = require('../setup/boot');
const { HANDOFF } = require('../setup/harness');
const { device } = require('./adb');

// Long enough that a thirty second skip lands in the middle rather than at
// the end, and that a resume point is neither the start nor "watched".
const MEDIA_SECONDS = 180;

// Boots the harness for an app on whatever device adb lists. The device's own
// 127.0.0.1 is forwarded to the harness, so the app is given the same address
// the server publishes and the posters live at.
function emulatorSetup({ PACKAGE, apk }) {
  return async function globalSetup() {
    const phone = device();
    phone.adb(['wait-for-device']);

    const app = await boot({ mediaSeconds: MEDIA_SECONDS });

    phone.reverse(app.facts.port);
    phone.shell('settings put secure immersive_mode_confirmations confirmed', { allowFailure: true });
    phone.install(await apk());
    phone.clear(PACKAGE);

    fs.mkdirSync(path.dirname(HANDOFF), { recursive: true });
    fs.writeFileSync(HANDOFF, JSON.stringify({ ...app.facts, mediaSeconds: MEDIA_SECONDS }));

    return async () => {
      await app.stop();
    };
  };
}

module.exports = { emulatorSetup };
