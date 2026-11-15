const fs = require('node:fs');
const path = require('node:path');

const VERSION = '2.7.3';
const PACKAGE = 'org.jellyfin.mobile';
const RELEASES = 'https://api.github.com/repos/jellyfin/jellyfin-android/releases';
const CACHE = path.join(__dirname, '..', '..', '.clients');

// The libre release build, which needs no Google services on the device.
// The release also carries debug builds, which install under another
// package. ANDROID_APK points at another build instead.
// CLIENT_RELEASE=latest installs whatever the project released last instead
// of the pinned version, which is what the nightly run does so that a
// release changing what the app sends is caught the morning after.
async function apk() {
  if (process.env.ANDROID_APK) {
    return process.env.ANDROID_APK;
  }

  const headers = { Accept: 'application/vnd.github+json' };
  if (process.env.GITHUB_TOKEN) {
    headers.Authorization = `Bearer ${process.env.GITHUB_TOKEN}`;
  }
  const latest = process.env.CLIENT_RELEASE === 'latest';
  const url = latest ? `${RELEASES}/latest` : `${RELEASES}/tags/v${VERSION}`;
  const release = await fetch(url, { headers });
  if (!release.ok) {
    throw new Error(`${url} answered ${release.status}`);
  }
  const { tag_name: tag, assets } = await release.json();

  const target = path.join(CACHE, `jellyfin-android-${tag}.apk`);
  if (fs.existsSync(target)) {
    return target;
  }

  const chosen = assets.find((asset) => /libre-release\.apk$/i.test(asset.name));
  if (!chosen) {
    throw new Error(`${tag} carries no libre release apk: ${assets.map((asset) => asset.name).join(', ')}`);
  }
  console.log(`Installing ${chosen.name}${latest ? ' (latest release)' : ''}`);

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
