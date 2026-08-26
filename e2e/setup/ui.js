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
  await keepOnOrigin(page);

  return {
    page,
    async close() {
      await context.close();
      browser.disconnect();
    },
  };
}

// The client asks gstatic.com for the Chromecast sender, which would make a
// run depend on the network and on Google being up.
async function keepOnOrigin(page) {
  const origin = harness().baseUrl;

  await page.setRequestInterception(true);
  page.on('request', (request) => {
    if (request.url().startsWith(origin)) {
      request.continue();
      return;
    }

    request.abort();
  });
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

async function signInAndLand(page) {
  await signIn(page);
  await waitForRoute(page, 'home.html');
}

async function text(page) {
  return page.evaluate(() => document.body.innerText);
}

// The client routes on the hash, so a landing is a hash change rather than a
// navigation puppeteer can wait on.
async function waitForRoute(page, fragment, timeout = 30000) {
  await waitFor(
    page,
    (want) => window.location.hash.includes(want),
    [fragment],
    timeout,
    () => `the client never routed to ${fragment}`,
  );
}

// It answers with the text that satisfied it rather than reading the page a
// second time, because the client swaps views by hiding one and showing the
// next and leaves the outgoing one in the DOM: a second read can land in the
// gap where neither is on screen.
//
// `within` scopes the wait to a view that is on screen, so text the outgoing
// view still carries cannot satisfy a wait meant for the incoming one. The
// pages carry stable ids (#loginPage, #indexPage, #moviesPage); without it the
// whole body is read, which is what anything outside a view needs.
async function waitForText(page, wanted, within = '', timeout = 30000) {
  return waitFor(
    page,
    (want, selector) => {
      const shown = selector
        ? Array.from(document.querySelectorAll(selector))
            .filter((view) => view.offsetParent !== null)
            .map((view) => view.innerText)
            .join('\n')
        : document.body.innerText;

      return shown.includes(want) ? shown : null;
    },
    [wanted, within],
    timeout,
    () => `${within || 'the page'} never showed ${JSON.stringify(wanted)}`,
  );
}

async function textOf(page, selector, timeout = 30000) {
  await waitFor(
    page,
    (want) => {
      const found = Array.from(document.querySelectorAll(want)).find((element) => element.offsetParent !== null);

      return Boolean(found && found.innerText.trim());
    },
    [selector],
    timeout,
    () => `nothing visible matched ${selector}`,
  );

  return page.$$eval(selector, (elements) => {
    const found = elements.find((element) => element.offsetParent !== null);

    return found ? found.innerText.trim() : '';
  });
}

// A bare puppeteer timeout says only that thirty seconds passed, so what the
// page was showing instead is read back and reported.
async function waitFor(page, predicate, args, timeout, describe) {
  try {
    const satisfied = await page.waitForFunction(predicate, { timeout }, ...args);

    return await satisfied.jsonValue();
  } catch (error) {
    if (error.name !== 'TimeoutError') {
      throw error;
    }

    throw new Error(`${describe()}. It was at ${page.url()} showing:\n${await text(page)}`);
  }
}

module.exports = { harness, open, loadLogin, signIn, signInAndLand, text, textOf, waitForRoute, waitForText };
