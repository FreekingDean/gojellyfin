const fs = require('node:fs');
const path = require('node:path');

const VERSION = '0.20.0';
const PACKAGE = 'org.jellyfin.androidtv';
const ACTIVITY = `${PACKAGE}/.ui.startup.StartupActivity`;
const RELEASE = `https://github.com/jellyfin/jellyfin-androidtv/releases/download/v${VERSION}/jellyfin-androidtv-v${VERSION}-release.apk`;
const CACHE = path.join(__dirname, '..', '..', '.clients');

// The release build, as a television installs it. ANDROIDTV_APK points at
// another build instead.
async function apk() {
  if (process.env.ANDROIDTV_APK) {
    return process.env.ANDROIDTV_APK;
  }

  const target = path.join(CACHE, `jellyfin-androidtv-v${VERSION}.apk`);
  if (fs.existsSync(target)) {
    return target;
  }

  const response = await fetch(RELEASE);
  if (!response.ok) {
    throw new Error(`${RELEASE} answered ${response.status}`);
  }

  fs.mkdirSync(CACHE, { recursive: true });
  fs.writeFileSync(target + '.part', Buffer.from(await response.arrayBuffer()));
  fs.renameSync(target + '.part', target);

  return target;
}

module.exports = { VERSION, PACKAGE, ACTIVITY, apk };
