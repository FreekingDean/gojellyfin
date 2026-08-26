const fs = require('node:fs');

// puppeteer-core drives an installed browser, so nothing downloads one.
const CANDIDATES = [
  process.env.CHROME_PATH,
  process.env.PUPPETEER_EXECUTABLE_PATH,
  '/usr/bin/google-chrome',
  '/usr/bin/google-chrome-stable',
  '/usr/bin/chromium',
  '/usr/bin/chromium-browser',
  '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',
  '/Applications/Chromium.app/Contents/MacOS/Chromium',
];

function executablePath() {
  for (const candidate of CANDIDATES) {
    if (candidate && fs.existsSync(candidate)) {
      return candidate;
    }
  }

  throw new Error(
    'no Chrome found: install one or set CHROME_PATH. Looked in ' + CANDIDATES.filter(Boolean).join(', '),
  );
}

module.exports = { executablePath };
