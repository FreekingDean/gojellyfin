const { spawn, spawnSync } = require('node:child_process');
const net = require('node:net');
const path = require('node:path');

const VERSION = require('@playwright/test/package.json').version;
const IMAGE = `mcr.microsoft.com/playwright:v${VERSION}-noble`;
const MODULES = path.join(__dirname, '..', 'node_modules');
const NAME = `gojellyfin-e2e-browser-${process.pid}`;

// The browser runs in the Playwright image matching the installed version, so
// text renders with the same fonts on a laptop as in CI, which is what lets
// screenshots be compared at all. The server inside it is this checkout's own
// node_modules, mounted rather than downloaded, so the two cannot drift. The tests themselves run here; exposeNetwork tunnels the browser's
// requests for 127.0.0.1 back to this machine, so the harness needs no change.
async function main() {
  const port = await freePort();

  const started = spawnSync(
    'docker',
    [
      'run', '-d', '--rm', '--init', '--ipc=host', '--name', NAME,
      '-p', `127.0.0.1:${port}:3000`,
      '-v', `${MODULES}:/e2e/node_modules:ro`,
      IMAGE,
      'node', '/e2e/node_modules/@playwright/test/cli.js', 'run-server', '--port', '3000', '--host', '0.0.0.0',
    ],
    { encoding: 'utf8' },
  );
  if (started.status !== 0) {
    throw new Error(`could not start ${IMAGE}: ${started.stderr || started.error}`);
  }

  try {
    await waitForServer();
    const status = await test(process.argv.slice(2), `ws://127.0.0.1:${port}/`);
    process.exitCode = status;
  } finally {
    spawnSync('docker', ['rm', '-f', NAME], { stdio: 'ignore' });
  }
}

function test(args, endpoint) {
  return new Promise((resolve) => {
    const child = spawn('npx', ['playwright', 'test', ...args], {
      stdio: 'inherit',
      env: { ...process.env, E2E_BROWSER_WS: endpoint },
    });
    child.on('exit', (code) => resolve(code ?? 1));
  });
}

function freePort() {
  return new Promise((resolve, reject) => {
    const probe = net.createServer();
    probe.on('error', reject);
    probe.listen(0, '127.0.0.1', () => {
      const { port } = probe.address();
      probe.close(() => resolve(port));
    });
  });
}

async function waitForServer() {
  const deadline = Date.now() + 180000;

  while (Date.now() < deadline) {
    const logs = spawnSync('docker', ['logs', NAME], { encoding: 'utf8' });
    if (/Listening on/.test(logs.stdout + logs.stderr)) {
      return;
    }
    if (logs.status !== 0) {
      throw new Error(`the browser server exited:\n${logs.stdout}${logs.stderr}`);
    }
    await new Promise((resolve) => setTimeout(resolve, 500));
  }

  throw new Error('the browser server never listened');
}

main().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
