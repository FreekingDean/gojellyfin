const { test: base, expect } = require('@playwright/test');

const { harness } = require('../../setup/harness');
const { device, sleep } = require('../adb');
const { api } = require('../api');
const { reportFailure } = require('../failure');
const { PACKAGE } = require('./app');

// The phone app is a native screen that asks for the server and then
// jellyfin-web in a WebView. uiautomator reads a WebView's page as views, so
// past the first screen what is matched is the page's own text and labels.
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

  phone: async ({}, use, testInfo) => {
    const phone = device();
    phone.clearLog();
    await use(phone);

    if (testInfo.status !== testInfo.expectedStatus) {
      await reportFailure(phone, PACKAGE, testInfo);
    }
  },

  // What the phone shows, published and never compared, for the reason the
  // television's screenshots are not.
  shot: async ({ phone }, use, testInfo) => {
    await use(async (name) => {
      await sleep(1500);
      const file = phone.screenshot(testInfo.outputPath(`${name}.png`));
      await testInfo.attach(`${name}-current.png`, { path: file });

      return file;
    });
  },

  signedIn: async ({ phone, harness: app }, use) => {
    const landed = await launch(phone, screens(app));
    if (landed !== 'home') {
      await freshStart(phone);
      await connect(phone, app);
      await signInWithPassword(phone, app);
      await home(phone, app);
    }

    await use(phone);
  },
});

const screens = (app) => ({
  home: { label: app.library },
  connect: { type: 'EditText' },
  login: { label: /^(Sign In|Use Quick Connect)$/i },
});

function launcherActivity(phone) {
  const resolved = phone
    .shell(`cmd package resolve-activity --brief -c android.intent.category.LAUNCHER ${PACKAGE}`)
    .trim()
    .split('\n')
    .pop()
    .trim();
  if (!resolved.includes('/')) {
    throw new Error(`${PACKAGE} has no launcher activity: ${resolved}`);
  }

  return resolved;
}

async function launch(phone, wanted) {
  phone.start(launcherActivity(phone));
  const landed = await phone.waitForAny(wanted, { timeout: 90000 });

  return landed.name;
}

async function freshStart(phone) {
  phone.clear(PACKAGE);
  await launch(phone, { connect: { type: 'EditText' } });
}

// The connect screen is drawn before its field takes typing, so the address
// is typed until the field holds it.
async function connect(phone, app) {
  for (let attempt = 0; ; attempt++) {
    await sleep(1500);
    await phone.fill({ type: 'EditText' }, app.baseUrl);
    if (phone.find({ type: 'EditText', text: app.baseUrl }).length) {
      break;
    }
    if (attempt === 3) {
      throw new Error(`the server's address never reached its field. On screen:\n${phone.summary()}`);
    }
  }
  await phone.tapOn({ label: /^connect$/i });
  await phone.waitForAny({ login: screens(app).login, users: { label: app.username } }, { timeout: 90000 });
}

// jellyfin-web's login page lists the server's users as cards; the card
// opens the password form for that user.
async function signInWithPassword(phone, app) {
  const [card] = phone.find({ label: new RegExp(`^${app.username}$`) });
  if (card) {
    await phone.press(card);
  }
  await phone.waitFor({ type: 'EditText' });
  const fields = phone.find({ type: 'EditText' });
  if (fields.length > 1) {
    await phone.fill({ type: 'EditText' }, app.username);
  }
  const password = phone.find({ type: 'EditText' }).pop();
  phone.tap(password);
  await sleep(500);
  phone.type(app.password);
  await sleep(500);
  await phone.hideKeyboard();
  await phone.tapOn({ label: /^Sign In$/i });
}

async function quickConnectCode(phone) {
  await phone.tapOn({ label: /^(Use )?Quick Connect$/i });
  const shown = await phone.waitFor({ label: /\b\d{6}\b/ }, { timeout: 30000 });

  return /\b(\d{6})\b/.exec(shown.text || shown.description)[1];
}

function home(phone, app) {
  return phone.waitFor({ label: app.library }, { timeout: 90000 });
}

// A library's cards carry their titles as text under the poster, so the
// titles are read straight off the screen once the first one has drawn.
async function openLibrary(phone, app) {
  await phone.tapOn({ label: app.library });
  await phone.waitFor({ label: app.movies[0] }, { timeout: 60000 });
}

function onScreen(phone, names) {
  const nodes = phone.dump();

  return names.filter((name) => nodes.some((node) => node.text === name || node.description === name));
}

async function openDetails(phone, app, name) {
  await openLibrary(phone, app);
  await phone.tapOn({ label: name }, { timeout: 30000 });
  await phone.waitFor({ label: /^(Play|Resume)$/i }, { timeout: 60000 });
}

// uiautomator cannot read the screen while the video plays, and Back does
// not stop jellyfin-web's player in the app, so the player is stopped with
// the remote's stop key, which the page takes as its own Stop.
async function leavePlayer(phone) {
  for (let attempt = 0; attempt < 3; attempt++) {
    phone.key('stop');
    await sleep(2500);
    if (phone.find({ label: /^(Play|Resume)$/i }).length) {
      return;
    }
    phone.key('back');
    await sleep(2500);
  }

  throw new Error(`the player never stopped. On screen:\n${phone.summary()}`);
}

module.exports = {
  test,
  expect,
  freshStart,
  connect,
  signInWithPassword,
  quickConnectCode,
  home,
  openLibrary,
  onScreen,
  openDetails,
  leavePlayer,
};
