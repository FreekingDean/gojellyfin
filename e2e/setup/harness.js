const fs = require('node:fs');
const path = require('node:path');

const HANDOFF = path.join(__dirname, '..', '.build', 'harness.json');

let cached;

function harness() {
  if (!cached) {
    cached = JSON.parse(fs.readFileSync(HANDOFF, 'utf8'));
  }

  return cached;
}

module.exports = { HANDOFF, harness };
