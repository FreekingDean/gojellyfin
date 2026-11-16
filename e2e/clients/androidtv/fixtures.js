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

  const grid = nodes.find((node) => node.id === 'horizontal_grid');
  const focus = nodes.find((node) => node.focused);
  const inGrid = Boolean(grid && focus && focus.bounds[1] >= grid.bounds[1]);
  const where = focus ? focus.bounds.join() : '';

  return { title: title ? title.text : '', index: place ? Number(place[1]) : 0, total: place ? Number(place[2]) : 0, inGrid, where };
}

// The grid fills a column before it starts the next, and the last column can
// be shorter than the rest. Right from a row that column does not reach
// leaves the grid for the toolbar, so the walk climbs back to the top of the
// column it is in, counting the climb because up from the top row leaves the
// grid too, and crosses there.
async function walkGrid(tv, visit) {
  // The counter can trail the focus, so a move waits for the focus to land
  // somewhere new before it is read, and gives up quietly when it never does.
  const move = async (name) => {
    const from = focusedTitle(tv);
    tv.key(name);
    for (let wait = 0; wait < 12; wait++) {
      await sleep(250);
      const now = focusedTitle(tv);
      if (now.where !== from.where && now.index !== from.index) {
        break;
      }
    }
    await sleep(250);

    return focusedTitle(tv);
  };

  let focused = focusedTitle(tv);
  let top = focused.index;
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
    for (let climb = before - top; climb > 0; climb--) {
      focused = await move('up');
    }
    focused = await move('right');
    if (!focused.inGrid || focused.index <= before) {
      return null;
    }
    top = focused.index;
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
