const fs = require('node:fs');

const { boot } = require('./boot');
const { HANDOFF } = require('./harness');

module.exports = async function globalSetup() {
  const app = await boot();

  fs.mkdirSync(require('node:path').dirname(HANDOFF), { recursive: true });
  fs.writeFileSync(HANDOFF, JSON.stringify(app.facts));

  return async () => {
    await app.stop();
  };
};
