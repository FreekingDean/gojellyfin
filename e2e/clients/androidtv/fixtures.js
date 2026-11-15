const fs = require('node:fs');
const jsQR = require('jsqr');
const { PNG } = require('pngjs');
const { test: base, expect } = require('@playwright/test');

const { harness } = require('../../setup/harness');
const { device, sleep } = require('../adb');
const { api } = require('../api');
const { reportFailure } = require('../failure');
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
      await reportFailure(tv, PACKAGE, testInfo);
    }
  },

  // What the television shows, published beside the browser's screenshots.
  // Nothing is compared: an emulator's frame carries a clock and whatever the
  // GPU emulation made of a gradient.
  shot: async ({ tv }, use, testInfo) => {
    await use(async (name) => {
      await sleep(1500);
      const file = tv.screenshot(testInfo.outputPath(`${name}.png`));
      await testInfo.attach(`${name}-current.png`, { path: file });

      return file;
    });
  },

  signedIn: async ({ tv, harness: app }, use) => {
    const landed = await launch(tv, {
      home: { label: app.library },
      servers: { id: 'enter_server_address' },
      users: { id: 'add_user_button' },
      login: { id: 'login_method' },
    });
    if (landed !== 'home') {
      await freshStart(tv);
      await connect(tv, app);
      await openSignIn(tv, app);
      await signInWithPassword(tv, app);
      await home(tv, app);
    }

    await use(tv);
  },
});

// The emulator's launcher draws a tutorial over whatever is in front of it
// the first few times it is seen, so one that turns up is dismissed and the
// app is brought back.
async function launch(tv, screens) {
  for (let attempt = 0; ; attempt++) {
    tv.start(ACTIVITY);
    const landed = await tv.waitForAny({ ...screens, tutorial: { id: 'tutorial_negative_action' } }, { timeout: 90000 });
    if (landed.name !== 'tutorial') {
      return landed.name;
    }
    if (attempt === 3) {
      throw new Error('the launcher tutorial would not go away');
    }
    await tv.press(landed.node);
  }
}

async function freshStart(tv) {
  tv.clear(PACKAGE);
  await launch(tv, { servers: { id: 'enter_server_address' } });
}

async function connect(tv, app) {
  await tv.tapOn({ id: 'enter_server_address' });
  await tv.fill({ id: 'address' }, app.baseUrl);
  await tv.tapOn({ id: 'confirm' });
  await tv.waitFor({ id: 'add_user_button' }, { timeout: 60000 });
}

async function openSignIn(tv) {
  await tv.tapOn({ id: 'add_user_button' });

  return tv.waitForAny({ quickConnect: { id: 'quick_connect_code' }, password: { id: 'password' } });
}

async function signInWithPassword(tv, app) {
  const [toggle] = tv.find({ id: 'use_credentials' });
  if (toggle) {
    await tv.press(toggle);
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
    await tv.press(toggle);
  }

  const shown = await tv.waitFor({ id: 'quick_connect_code', text: /^\d{3} \d{3}$/ }, { timeout: 30000 });

  return shown.text.replace(/\s/g, '');
}

// What a phone's camera would make of the screen, or null when it finds no
// code on it.
function qrOn(file) {
  const picture = PNG.sync.read(fs.readFileSync(file));
  const found = jsQR(new Uint8ClampedArray(picture.data), picture.width, picture.height);

  return found ? found.data : null;
}

function home(tv, app) {
  return tv.waitFor({ label: app.library }, { timeout: 90000 });
}

// A library opens as a grid of posters with no text on them: the app writes
// the focused title above the grid and its place below it, so the titles are
// read by walking the focus along the grid with the remote. The grid fills
// its columns top to bottom, so the walk goes down a column, then across to
// the next and back up to its top.
async function openLibrary(tv, app) {
  await tv.tapOn({ label: app.library });
  await tv.waitFor({ id: 'counter', text: /^\d+ \| \d+$/ }, { timeout: 60000 });
}

function focusedTitle(tv) {
  const nodes = tv.dump();
  const title = nodes.find((node) => node.id === 'title');
  const counter = nodes.find((node) => node.id === 'counter');
  const place = counter && /^(\d+) \| (\d+)$/.exec(counter.text);

  return { title: title ? title.text : '', index: place ? Number(place[1]) : 0, total: place ? Number(place[2]) : 0 };
}

async function walkGrid(tv, visit) {
  const move = async (name) => {
    tv.key(name);
    await sleep(700);

    return focusedTitle(tv);
  };

  let focused = focusedTitle(tv);
  for (let step = 0; step < 50; step++) {
    if (visit(focused)) {
      return focused;
    }
    if (focused.total && focused.index >= focused.total) {
      return null;
    }
    const before = focused.index;
    focused = await move('down');
    if (focused.index !== before) {
      continue;
    }
    focused = await move('right');
    for (let climb = 0; climb < 10 && focused.index > before + 1; climb++) {
      const above = await move('up');
      if (above.index === focused.index) {
        break;
      }
      focused = above;
    }
    if (focused.index === before) {
      return null;
    }
  }

  return null;
}

async function libraryTitles(tv) {
  const titles = [];
  await walkGrid(tv, ({ title }) => {
    if (title && !titles.includes(title)) {
      titles.push(title);
    }

    return false;
  });

  return titles;
}

async function openDetails(tv, app, name) {
  await openLibrary(tv, app);
  const found = await walkGrid(tv, ({ title }) => title === name);
  if (!found) {
    throw new Error(`${name} is not in ${app.library}. On screen:\n${tv.summary()}`);
  }
  tv.key('center');
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
  qrOn,
  home,
  openLibrary,
  libraryTitles,
  openDetails,
  leavePlayer,
};
