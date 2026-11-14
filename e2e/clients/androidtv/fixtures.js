const fs = require('node:fs');
const { test: base, expect } = require('@playwright/test');

const { harness } = require('../../setup/harness');
const { SERVER_LOG } = require('../../setup/boot');
const { device, sleep } = require('../adb');
const { api } = require('../api');
const { PACKAGE, ACTIVITY } = require('./app');

const test = base.extend({
  harness: [
    async ({}, use) => {
      await use(harness());
    },
    { scope: 'worker' },
  ],

  server: [
    async ({ harness: app }, use) => {
      await use(await api(app));
    },
    { scope: 'worker' },
  ],

  tv: async ({}, use, testInfo) => {
    const tv = device();
    tv.clearLog();
    await use(tv);

    if (testInfo.status !== testInfo.expectedStatus) {
      console.log(`On screen when "${testInfo.title}" ended:\n${tv.summary()}`);
      const name = 'failed-' + testInfo.title.toLowerCase().replace(/[^a-z0-9]+/g, '-');
      await testInfo.attach(`${name}-current.png`, { path: tv.screenshot(testInfo.outputPath('failure.png')) });
      await testInfo.attach('logcat.txt', { path: tv.logcat(testInfo.outputPath('logcat.txt')) });
      if (fs.existsSync(SERVER_LOG)) {
        await testInfo.attach('server.log', { path: SERVER_LOG, contentType: 'text/plain' });
      }
    }
  },

  // What the television shows, published beside the browser's screenshots.
  // Nothing is compared: an emulator's frame carries a clock and whatever the
  // GPU emulation made of a gradient.
  shot: async ({ tv }, use, testInfo) => {
    await use(async (name) => {
      await sleep(1500);
      await testInfo.attach(`${name}-current.png`, { path: tv.screenshot(testInfo.outputPath(`${name}.png`)) });
    });
  },

  signedIn: async ({ tv, harness: app }, use) => {
    tv.stop(PACKAGE);
    tv.start(ACTIVITY);

    const landed = await tv.waitForAny(
      {
        home: { label: app.library },
        servers: { id: 'enter_server_address' },
        users: { id: 'add_user_button' },
        login: { id: 'login_method' },
      },
      { timeout: 90000 },
    );
    if (landed.name !== 'home') {
      await freshStart(tv);
      await connect(tv, app);
      await openSignIn(tv, app);
      await signInWithPassword(tv, app);
      await home(tv, app);
    }

    await use(tv);
  },
});

async function freshStart(tv) {
  tv.stop(PACKAGE);
  tv.clear(PACKAGE);
  tv.start(ACTIVITY);
  await tv.waitFor({ id: 'enter_server_address' }, { timeout: 90000 });
}

async function connect(tv, app) {
  await tv.tapOn({ id: 'enter_server_address' });
  await tv.fill({ id: 'address' }, app.baseUrl);
  await tv.tapOn({ id: 'confirm' });
  await tv.waitFor({ id: 'add_user_button' }, { timeout: 60000 });
}

// A server that lists its users publicly shows a card for each; one that
// does not offers only "Add account". Either leads to the same sign-in.
async function openSignIn(tv, app) {
  const [card] = tv.find({ label: app.username });
  tv.tap(card || (await tv.waitFor({ id: 'add_user_button' })));

  return tv.waitForAny({ quickConnect: { id: 'quick_connect_code' }, password: { id: 'password' } });
}

async function signInWithPassword(tv, app) {
  const [toggle] = tv.find({ id: 'use_credentials' });
  if (toggle) {
    tv.tap(toggle);
  }

  await tv.waitFor({ id: 'password' });
  const [username] = tv.find({ id: 'username' });
  if (username && username.enabled && username.text !== app.username) {
    await tv.fill({ id: 'username' }, app.username);
  }
  await tv.fill({ id: 'password' }, app.password);
  await tv.tapOn({ id: 'confirm' });
}

async function quickConnectCode(tv) {
  const [toggle] = tv.find({ id: 'use_quickconnect' });
  if (toggle && !tv.find({ id: 'quick_connect_code' }).length) {
    tv.tap(toggle);
  }

  const shown = await tv.waitFor({ id: 'quick_connect_code', text: /^\d{3} \d{3}$/ }, { timeout: 30000 });

  return shown.text.replace(/\s/g, '');
}

function home(tv, app) {
  return tv.waitFor({ label: app.library }, { timeout: 90000 });
}

async function openLibrary(tv, app) {
  await tv.tapOn({ label: app.library });
  await tv.waitFor({ label: app.playable }, { timeout: 60000 });
}

async function openDetails(tv, app, name) {
  await openLibrary(tv, app);
  await tv.tapOn({ label: name });
  await tv.waitFor({ label: /^(Play|Resume from .*)$/ }, { timeout: 60000 });
}

// The first Back only puts the player's controls away when they are showing.
async function leavePlayer(tv) {
  for (let attempt = 0; attempt < 4; attempt++) {
    tv.key('back');
    await sleep(2500);
    if (tv.find({ label: /^(Play|Resume from .*)$/ }).length) {
      return;
    }
  }

  throw new Error(`Back never left the player. On screen:\n${tv.summary()}`);
}

module.exports = {
  test,
  expect,
  freshStart,
  connect,
  openSignIn,
  signInWithPassword,
  quickConnectCode,
  home,
  openLibrary,
  openDetails,
  leavePlayer,
};
