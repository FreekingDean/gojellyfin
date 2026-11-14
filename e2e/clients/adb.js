const { spawnSync } = require('node:child_process');
const fs = require('node:fs');

const KEYS = {
  back: 4,
  up: 19,
  down: 20,
  left: 21,
  right: 22,
  center: 23,
  enter: 66,
  playPause: 85,
  fastForward: 90,
  rewind: 89,
};

// A device is driven the way a person without a debugger would see it: what
// uiautomator says is on screen, a tap or a remote key, and a screenshot.
// Nothing here knows which app is running.
function device(serial = process.env.ANDROID_SERIAL) {
  const base = serial ? ['-s', serial] : [];

  function adb(args, options = {}) {
    const result = spawnSync('adb', [...base, ...args], {
      encoding: options.binary ? 'buffer' : 'utf8',
      maxBuffer: 64 * 1024 * 1024,
      timeout: options.timeout || 60000,
    });
    if (result.error) {
      throw new Error(`adb ${args.join(' ')}: ${result.error.message}`);
    }
    if (result.status !== 0 && !options.allowFailure) {
      throw new Error(`adb ${args.join(' ')} exited ${result.status}\n${result.stdout}${result.stderr}`);
    }

    return options.binary ? result.stdout : (result.stdout || '') + (options.allowFailure ? result.stderr || '' : '');
  }

  const shell = (command, options) => adb(['shell', command], options);

  // uiautomator refuses while the window never goes idle, which is every
  // frame of a playing video, so a caller that needs the tree during playback
  // has to be ready for an empty answer.
  function read() {
    const out = adb(['exec-out', 'uiautomator', 'dump', '/dev/tty'], { allowFailure: true });
    const end = out.lastIndexOf('</hierarchy>');
    if (end === -1) {
      return [];
    }

    return parse(out.slice(out.indexOf('<?xml'), end));
  }

  // A slow emulator's own apps stop answering now and then, and Android puts
  // its "isn't responding" dialog over whatever is on screen until someone
  // chooses to wait, so that choice is made before anything is looked at.
  function dump() {
    for (let attempt = 0; attempt < 5; attempt++) {
      const nodes = read();
      const wait = nodes.find((node) => node.id === 'aerr_wait');
      if (!wait) {
        return nodes;
      }
      tap(wait);
      spawnSync('sleep', ['1']);
    }

    return read();
  }

  function find(matcher) {
    return dump().filter((node) => matches(node, matcher));
  }

  async function waitFor(matcher, { timeout = 30000, gone = false } = {}) {
    const deadline = Date.now() + timeout;
    let nodes = [];

    for (;;) {
      nodes = dump();
      const found = nodes.filter((node) => matches(node, matcher));
      if (gone ? nodes.length > 0 && found.length === 0 : found.length > 0) {
        return found[0];
      }
      if (Date.now() > deadline) {
        break;
      }
      await sleep(500);
    }

    throw new Error(
      `${describe(matcher)} ${gone ? 'never left' : 'never appeared on'} the screen within ${timeout}ms. On screen:\n${summary(nodes)}`,
    );
  }

  // Whichever of several screens the app lands on, by name.
  async function waitForAny(matchers, { timeout = 30000 } = {}) {
    const deadline = Date.now() + timeout;
    let nodes = [];

    for (;;) {
      nodes = dump();
      for (const [name, matcher] of Object.entries(matchers)) {
        const found = nodes.find((node) => matches(node, matcher));
        if (found) {
          return { name, node: found };
        }
      }
      if (Date.now() > deadline) {
        break;
      }
      await sleep(500);
    }

    throw new Error(
      `none of ${Object.keys(matchers).join(', ')} appeared within ${timeout}ms. On screen:\n${summary(nodes)}`,
    );
  }

  function tap(node) {
    const [x1, y1, x2, y2] = node.bounds;
    shell(`input tap ${Math.round((x1 + x2) / 2)} ${Math.round((y1 + y2) / 2)}`);
  }

  // A view that takes focus in touch mode spends the first tap on taking it,
  // which is most of a television app, so a tap that only moved the focus
  // onto its target is finished with the remote's centre key. A view that
  // already had the focus was clicked by the tap itself. A text field is left
  // alone: there the centre key types on the on-screen keyboard.
  async function press(node) {
    const [x1, y1, x2, y2] = node.bounds;
    const x = (x1 + x2) / 2;
    const y = (y1 + y2) / 2;
    const focusedHere = (nodes) =>
      nodes.some((other) => other.focused && other.bounds[0] <= x && x <= other.bounds[2] && other.bounds[1] <= y && y <= other.bounds[3]);

    const before = focusedHere(dump());
    tap(node);
    await sleep(800);

    const nodes = dump();
    const still = nodes.some((other) => other.bounds.join() === node.bounds.join() && other.text === node.text && other.id === node.id);
    if (!before && still && focusedHere(nodes) && node.type !== 'EditText') {
      key('center');
    }
  }

  async function tapOn(matcher, options) {
    const node = await waitFor(matcher, options);
    await press(node);

    return node;
  }

  function key(name, times = 1) {
    const code = KEYS[name];
    if (code === undefined) {
      throw new Error(`no key named ${name}`);
    }
    shell(`input keyevent ${Array(times).fill(code).join(' ')}`);
  }

  // `input text` hands its argument to a shell on the device, and reads %s as
  // a space.
  function type(text) {
    const escaped = text.replace(/ /g, '%s').replace(/(["'`\\$&|;<>()*?!#~])/g, '\\$1');
    shell(`input text "${escaped}"`);
  }

  function keyboardShown() {
    return /mInputShown=true/.test(shell('dumpsys input_method', { allowFailure: true }));
  }

  // An on-screen keyboard covers the lower half of a TV and swallows the
  // remote's keys, so it is dismissed before anything under it is reached.
  async function hideKeyboard() {
    for (let attempt = 0; attempt < 3 && keyboardShown(); attempt++) {
      key('back');
      await sleep(700);
    }
  }

  async function fill(matcher, text) {
    await tapOn(matcher);
    await sleep(500);
    type(text);
    await sleep(500);
    await hideKeyboard();
  }

  function screenshot(file) {
    fs.writeFileSync(file, adb(['exec-out', 'screencap', '-p'], { binary: true }));

    return file;
  }

  function logcat(file) {
    fs.writeFileSync(file, adb(['logcat', '-d', '-v', 'time'], { allowFailure: true }));

    return file;
  }

  return {
    adb,
    shell,
    dump,
    find,
    waitFor,
    waitForAny,
    tap,
    press,
    tapOn,
    key,
    type,
    fill,
    hideKeyboard,
    screenshot,
    logcat,
    summary: () => summary(dump()),
    install: (apk) => adb(['install', '-r', '-g', apk], { timeout: 300000 }),
    reverse: (port) => adb(['reverse', `tcp:${port}`, `tcp:${port}`]),
    clear: (app) => shell(`pm clear ${app}`),
    stop: (app) => shell(`am force-stop ${app}`),
    start: (component) => shell(`am start -S -W -n ${component}`),
    clearLog: () => adb(['logcat', '-c'], { allowFailure: true }),
  };
}

function parse(xml) {
  const nodes = [];
  const tag = /<node\b([^>]*)>/g;
  let match;

  while ((match = tag.exec(xml))) {
    const attributes = {};
    const pair = /([\w-]+)="([^"]*)"/g;
    let attribute;
    while ((attribute = pair.exec(match[1]))) {
      attributes[attribute[1]] = unescape(attribute[2]);
    }

    const bounds = /\[(-?\d+),(-?\d+)\]\[(-?\d+),(-?\d+)\]/.exec(attributes.bounds || '');
    nodes.push({
      id: (attributes['resource-id'] || '').split(':id/').pop(),
      text: attributes.text || '',
      description: attributes['content-desc'] || '',
      type: (attributes.class || '').split('.').pop(),
      focused: attributes.focused === 'true',
      enabled: attributes.enabled !== 'false',
      bounds: bounds ? bounds.slice(1).map(Number) : [0, 0, 0, 0],
    });
  }

  return nodes;
}

function unescape(value) {
  return value
    .replace(/&lt;/g, '<')
    .replace(/&gt;/g, '>')
    .replace(/&quot;/g, '"')
    .replace(/&apos;/g, "'")
    .replace(/&#(\d+);/g, (_, code) => String.fromCharCode(Number(code)))
    .replace(/&amp;/g, '&');
}

// { id }, { type }, { text } and { description } all have to hold; a string is matched
// whole and a RegExp is tested. `label` is the text or the description,
// whichever the view carries.
function matches(node, matcher) {
  const holds = (value, wanted) => (wanted instanceof RegExp ? wanted.test(value) : value === wanted);

  if (matcher.id !== undefined && node.id !== matcher.id) {
    return false;
  }
  if (matcher.type !== undefined && node.type !== matcher.type) {
    return false;
  }
  if (matcher.text !== undefined && !holds(node.text, matcher.text)) {
    return false;
  }
  if (matcher.description !== undefined && !holds(node.description, matcher.description)) {
    return false;
  }
  if (matcher.label !== undefined && !holds(node.text, matcher.label) && !holds(node.description, matcher.label)) {
    return false;
  }
  if (matcher.focused !== undefined && node.focused !== matcher.focused) {
    return false;
  }

  return true;
}

function describe(matcher) {
  return Object.entries(matcher)
    .map(([key, value]) => `${key}=${value}`)
    .join(' ');
}

function summary(nodes) {
  const lines = nodes
    .filter((node) => node.id || node.text || node.description)
    .map((node) => {
      const parts = [node.type];
      if (node.id) parts.push(`#${node.id}`);
      if (node.text) parts.push(JSON.stringify(node.text));
      if (node.description) parts.push(`desc=${JSON.stringify(node.description)}`);
      if (node.focused) parts.push('(focused)');
      parts.push(`[${node.bounds.join(',')}]`);

      return '  ' + parts.join(' ');
    });

  return lines.length ? lines.join('\n') : '  (nothing uiautomator could read)';
}

function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

module.exports = { device, parse, matches, summary, sleep };
