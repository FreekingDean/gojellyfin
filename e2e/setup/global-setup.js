const fs = require('node:fs');
const path = require('node:path');
const puppeteer = require('puppeteer-core');

const { boot } = require('./boot');
const chrome = require('./chrome');

const HANDOFF = path.join(__dirname, '..', '.build', 'harness.json');

module.exports = async function globalSetup() {
  const app = await boot();

  const browser = await puppeteer.launch({
    executablePath: chrome.executablePath(),
    headless: true,
    args: ['--no-sandbox', '--disable-dev-shm-usage'],
  });

  globalThis.__APP__ = app;
  globalThis.__BROWSER__ = browser;

  fs.mkdirSync(path.dirname(HANDOFF), { recursive: true });
  fs.writeFileSync(
    HANDOFF,
    JSON.stringify({
      baseUrl: app.baseUrl,
      username: app.username,
      password: app.password,
      library: app.library,
      movies: app.movies,
      browserWSEndpoint: browser.wsEndpoint(),
    }),
  );
};

module.exports.HANDOFF = HANDOFF;
