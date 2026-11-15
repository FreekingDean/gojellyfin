const fs = require('node:fs');
const path = require('node:path');

const VERSION = '2.6.4';
const PACKAGE = 'org.jellyfin.mobile';
const RELEASE = `https://api.github.com/repos/jellyfin/jellyfin-android/releases/tags/v${VERSION}`;
const CACHE = path.join(__dirname, '..', '..', '.clients');

// The libre release build, which needs no Google services on the device.
// The release also carries debug builds, which install under another
// package. ANDROID_APK points at another build instead.
async function apk() {
  if (process.env.ANDROID_APK) {
    return process.env.ANDROID_APK;
  }

  const target = path.join(CACHE, `jellyfin-android-v${VERSION}.apk`);
  if (fs.existsSync(target)) {
    return target;
  }

  const headers = { Accept: 'application/vnd.github+json' };
  if (process.env.GITHUB_TOKEN) {
    headers.Authorization = `Bearer ${process.env.GITHUB_TOKEN}`;
  }
  const release = await fetch(RELEASE, { headers });
  if (!release.ok) {
    throw new Error(`${RELEASE} answered ${release.status}`);
  }
  const { assets } = await release.json();
  const chosen = assets.find((asset) => /libre-release\.apk$/i.test(asset.name));
  if (!chosen) {
    throw new Error(`v${VERSION} carries no libre release apk: ${assets.map((asset) => asset.name).join(', ')}`);
  }
  console.log(`Installing ${chosen.name}`);

  const response = await fetch(chosen.browser_download_url);
  if (!response.ok) {
    throw new Error(`${chosen.browser_download_url} answered ${response.status}`);
  }

  fs.mkdirSync(CACHE, { recursive: true });
  fs.writeFileSync(target + '.part', Buffer.from(await response.arrayBuffer()));
  fs.renameSync(target + '.part', target);

  return target;
}

module.exports = { VERSION, PACKAGE, apk };
