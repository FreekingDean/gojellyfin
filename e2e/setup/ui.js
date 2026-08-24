const fs = require('node:fs');
const path = require('node:path');
const puppeteer = require('puppeteer-core');

const HANDOFF = path.join(__dirname, '..', '.build', 'harness.json');

let cached;

function harness() {
  if (!cached) {
    cached = JSON.parse(fs.readFileSync(HANDOFF, 'utf8'));
  }

  return cached;
}

async function open() {
  const browser = await puppeteer.connect({ browserWSEndpoint: harness().browserWSEndpoint });
  const context = await browser.createBrowserContext();
  const page = await context.newPage();

  await page.setViewport({ width: 1280, height: 900 });

  return {
    page,
    async close() {
      await context.close();
      browser.disconnect();
    },
  };
}

async function loadLogin(page) {
  await page.goto(harness().baseUrl + '/web/', { waitUntil: 'networkidle2' });
  await page.waitForSelector('button.card', { visible: true });
}

async function signIn(page, password = harness().password) {
  await loadLogin(page);
  await page.click('button.card');
  await page.waitForSelector('#txtManualPassword', { visible: true });
  await page.type('#txtManualPassword', password);
  await page.click('button[type=submit]');
}

async function text(page) {
  return page.evaluate(() => document.body.innerText);
}

// The client routes on the hash, so a landing is a hash change rather than a
// navigation puppeteer can wait on.
async function waitForRoute(page, fragment, timeout = 30000) {
  await page.waitForFunction(
    (want) => window.location.hash.includes(want),
    { timeout },
    fragment,
  );
}

async function waitForText(page, wanted, timeout = 30000) {
  await page.waitForFunction(
    (want) => document.body.innerText.includes(want),
    { timeout },
    wanted,
  );
}

async function signInAndLand(page) {
  await signIn(page);
  await waitForRoute(page, 'home.html');
}

module.exports = { harness, open, loadLogin, signIn, signInAndLand, text, waitForRoute, waitForText };
