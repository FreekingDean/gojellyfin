const fs = require('node:fs');
const { test: base, expect } = require('@playwright/test');

const { harness } = require('../setup/harness');
const { SERVER_LOG } = require('../setup/boot');

// Each view the client has shown stays in the DOM, hidden, after it routes
// away. Scoping to the one on screen keeps text the outgoing view still carries
// from satisfying an assertion meant for the incoming one.
const VIEW = '.page:visible';

const test = base.extend({
  harness: [
    async ({}, use) => {
      await use(harness());
    },
    { scope: 'worker' },
  ],

  // The client asks gstatic.com for the Chromecast sender, which would make a
  // run depend on the network and on Google being up.
  page: async ({ page, harness: app }, use, testInfo) => {
    await page.route((url) => !url.href.startsWith(app.baseUrl), (route) => route.abort());
    await use(page);

    if (testInfo.status !== testInfo.expectedStatus && fs.existsSync(SERVER_LOG)) {
      await testInfo.attach('server.log', { path: SERVER_LOG, contentType: 'text/plain' });
    }
  },

  signedIn: async ({ page, harness: app }, use) => {
    await signIn(page, app);
    await expect(page).toHaveURL(/#\/home/, { timeout: 30000 });
    await use(page);
  },
});

async function loadLogin(page, app) {
  await page.goto(app.baseUrl + '/web/');
  await expect(page.locator('#loginPage #txtManualName')).toBeVisible({ timeout: 30000 });
}

async function signIn(page, app, password = app.password) {
  await loadLogin(page, app);
  await page.locator('#txtManualName').fill(app.username);
  await page.locator('#txtManualPassword').fill(password);
  await page.getByRole('button', { name: 'Sign In', exact: true }).click();
}

function view(page) {
  return page.locator(VIEW);
}

function card(page, name) {
  return view(page).locator('.card').filter({ has: page.locator('.cardText', { hasText: name }) }).first();
}

// A poster's centre carries a play button, so a click there starts playback
// instead of opening the item. Its corner opens it. The outgoing view fades
// rather than vanishing, so a click can land on its copy of the same card
// while it still counts as visible; the click is retried until the route
// moves. Both views stay visible for a moment once it has, so the views on
// screen before the click are marked and the helper waits until the one
// left visible is the new one.
async function open(page, name) {
  const from = page.url();
  await page.locator(VIEW).evaluateAll((views) => {
    document.querySelectorAll('[data-outgoing]').forEach((view) => delete view.dataset.outgoing);
    views.forEach((view) => (view.dataset.outgoing = 'true'));
  });

  await expect(async () => {
    await card(page, name).locator('.cardScalable').click({ position: { x: 10, y: 10 }, timeout: 5000 });
    await expect(page).not.toHaveURL(from, { timeout: 5000 });
  }).toPass({ timeout: 30000 });
  await expect(page.locator(VIEW + ':not([data-outgoing])')).toHaveCount(1);
  await expect(page.locator(VIEW)).toHaveCount(1);
}

// The client draws its own app bar on a desktop-sized window, so the
// dashboard is reached through the user menu rather than the side drawer.
async function openDashboard(page) {
  await page.getByRole('button', { name: 'User Menu' }).click();
  await page.getByRole('menuitem', { name: 'Dashboard' }).click();
  await expect(page).toHaveURL(/dashboard/);
}

module.exports = { test, expect, loadLogin, signIn, view, card, open, openDashboard };
