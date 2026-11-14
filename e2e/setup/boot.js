const { spawn, spawnSync } = require('node:child_process');
const net = require('node:net');
const path = require('node:path');
const fs = require('node:fs');
const http = require('node:http');

const proxy = require('./proxy');

const ROOT = path.resolve(__dirname, '..', '..');
const BUILD = path.join(__dirname, '..', '.build');
const WEB = path.join(__dirname, '..', '.client');
const SERVER_LOG = path.join(BUILD, 'server.log');

const USERNAME = 'smoke';
const PASSWORD = 'smoke-password';
const LIBRARY = 'Smoke Movies';
const MOVIES = ['Fixture Alpha', 'Fixture Beta'];
const PLAYABLE = 'Fixture Playable';
const SHOWS = 'Smoke Shows';
const SERIES = 'Fixture Show';
const SEASON = 'Season 1';
const EPISODES = ['Fixture Pilot', 'Fixture Second'];
const DRAMA = 'Fixture Drama';
const FINALE = 'Fixture Finale';

const CLIENT_IMAGE = 'jellyfin/jellyfin:10.10.0';

async function boot({ mediaSeconds = 20 } = {}) {
  const adminUrl = process.env.DATABASE_URL;
  if (!adminUrl) {
    throw new Error('DATABASE_URL is not set, so there is no server to create a scratch database on');
  }

  const binaries = build();
  const client = fetchClient();
  const media = encodeMedia(mediaSeconds);

  const scratch = 'gojellyfin_e2e_' + Math.random().toString(36).slice(2, 12).replace(/[^a-z0-9]/g, '0');
  const scratchEnv = { ADMIN_DATABASE_URL: adminUrl, SCRATCH_DATABASE: scratch };

  run(binaries.fixtures, ['create'], { env: scratchEnv });

  const databaseUrl = withDatabase(adminUrl, scratch);
  const serverEnv = clean({ DATABASE_URL: databaseUrl, MEDIA_DIRECTORIES: path.dirname(media) });

  const apiPort = await freePort();
  const posterRoot = path.join(BUILD, 'posters');
  fs.rmSync(posterRoot, { recursive: true, force: true });
  fs.mkdirSync(posterRoot, { recursive: true });

  const front = proxy.start({ webRoot: client, apiPort, posterRoot });
  const webPort = await listen(front);

  run(binaries.gojellyfin, ['migrate'], { env: serverEnv });
  run(binaries.gojellyfin, ['adduser', USERNAME], { env: serverEnv, input: PASSWORD + '\n' });
  run(binaries.fixtures, ['seed'], {
    env: {
      ...serverEnv,
      FIXTURE_MEDIA: media,
      POSTER_DIR: posterRoot,
      POSTER_URL: `http://127.0.0.1:${webPort}${proxy.POSTERS}`,
    },
  });

  const api = spawn(binaries.gojellyfin, ['server'], {
    env: {
      ...process.env,
      ...serverEnv,
      HTTP_PORT: String(apiPort),
      PUBLISHED_SERVER_URL: `http://127.0.0.1:${webPort}`,
    },
    stdio: ['ignore', 'pipe', 'pipe'],
  });

  const log = [];
  const file = fs.createWriteStream(SERVER_LOG);
  const record = (chunk) => {
    file.write(chunk);
    log.push(chunk.toString());
    if (log.length > 400) log.shift();
  };
  api.stdout.on('data', record);
  api.stderr.on('data', record);

  await waitForApi(apiPort, api, log);

  return {
    facts: {
      baseUrl: `http://127.0.0.1:${webPort}`,
      port: webPort,
      username: USERNAME,
      password: PASSWORD,
      library: LIBRARY,
      movies: MOVIES,
      playable: PLAYABLE,
      shows: SHOWS,
      series: SERIES,
      season: SEASON,
      episodes: EPISODES,
      drama: DRAMA,
      finale: FINALE,
    },
    serverLog: () => log.join(''),
    async stop() {
      await new Promise((resolve) => front.close(resolve));
      api.kill('SIGTERM');
      await once(api);
      file.end();
      run(binaries.fixtures, ['drop'], { env: scratchEnv, allowFailure: true });
    },
  };
}

function build() {
  fs.mkdirSync(BUILD, { recursive: true });
  const gojellyfin = path.join(BUILD, 'gojellyfin');
  const fixtures = path.join(BUILD, 'fixtures');

  run('go', ['build', '-o', gojellyfin, './cmd/gojellyfin'], { cwd: ROOT });
  run('go', ['build', '-o', fixtures, './e2e/fixtures'], { cwd: ROOT });

  return { gojellyfin, fixtures };
}

// VP9 and Opus in WebM because Playwright's Chromium carries no H.264 or AAC
// decoder, so that is the only pairing it will direct play.
function encodeMedia(seconds) {
  const target = path.join(BUILD, 'media', seconds === 20 ? 'playable.webm' : `playable-${seconds}s.webm`);
  if (fs.existsSync(target)) {
    return target;
  }

  fs.mkdirSync(path.dirname(target), { recursive: true });
  run('ffmpeg', [
    '-hide_banner', '-loglevel', 'error', '-y',
    '-f', 'lavfi', '-i', `testsrc2=size=640x360:rate=24:duration=${seconds}`,
    '-f', 'lavfi', '-i', `sine=frequency=440:duration=${seconds}`,
    '-c:v', 'libvpx-vp9', '-b:v', '300k', '-deadline', 'realtime', '-cpu-used', '8',
    '-c:a', 'libopus', '-b:a', '64k',
    '-shortest', target,
  ]);

  return target;
}

// jellyfin-web is published nowhere but inside the all-in-one image: no npm
// package, and its releases carry no assets.
function fetchClient() {
  if (process.env.JELLYFIN_WEB) {
    return process.env.JELLYFIN_WEB;
  }
  if (fs.existsSync(path.join(WEB, 'index.html'))) {
    return WEB;
  }

  fs.rmSync(WEB, { recursive: true, force: true });
  fs.mkdirSync(WEB, { recursive: true });

  const created = spawnSync('docker', ['create', CLIENT_IMAGE], { encoding: 'utf8' });
  if (created.status !== 0) {
    throw new Error(`failed to create a container from ${CLIENT_IMAGE}: ${created.stderr || created.error}`);
  }

  const container = created.stdout.trim();
  try {
    run('docker', ['cp', `${container}:/jellyfin/jellyfin-web/.`, WEB]);
  } finally {
    spawnSync('docker', ['rm', '-f', container], { stdio: 'ignore' });
  }

  return WEB;
}

function run(command, args, options = {}) {
  const result = spawnSync(command, args, {
    cwd: options.cwd,
    input: options.input,
    encoding: 'utf8',
    env: options.env ? { ...process.env, ...options.env } : process.env,
  });

  if (result.status !== 0 && !options.allowFailure) {
    throw new Error(
      `${command} ${args.join(' ')} exited ${result.status}\n${result.stdout || ''}${result.stderr || result.error || ''}`,
    );
  }

  return (result.stdout || '').trim();
}

function clean(overrides) {
  return {
    PUBLISHED_SERVER_URL: '',
    CORS_ORIGINS: '',
    TRANSCODER_JOBS: '',
    TRANSCODER_STALL_TIMEOUT: '',
    TEMPORAL_HOSTPORT: '',
    TEMPORAL_NAMESPACE: '',
    OTEL_EXPORTER_OTLP_ENDPOINT: '',
    TMDB_API_KEY: '',
    ...overrides,
  };
}

function withDatabase(dsn, name) {
  const url = new URL(dsn);
  url.pathname = '/' + name;

  return url.toString();
}

// :0 rather than a fixed port, so a run can never take :8081 from `make dev`.
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

function listen(server) {
  return new Promise((resolve, reject) => {
    server.on('error', reject);
    server.listen(0, '127.0.0.1', () => resolve(server.address().port));
  });
}

async function waitForApi(port, api, log) {
  const deadline = Date.now() + 60000;

  while (Date.now() < deadline) {
    if (api.exitCode !== null) {
      throw new Error(`the server exited with ${api.exitCode} before answering:\n${log.join('')}`);
    }
    if (await answers(port)) {
      return;
    }
    await sleep(200);
  }

  throw new Error(`the server never answered on ${port}:\n${log.join('')}`);
}

function answers(port) {
  return new Promise((resolve) => {
    const request = http.get(
      { host: '127.0.0.1', port, path: '/System/Info/Public', timeout: 2000 },
      (response) => {
        response.resume();
        resolve(response.statusCode === 200);
      },
    );
    request.on('error', () => resolve(false));
    request.on('timeout', () => {
      request.destroy();
      resolve(false);
    });
  });
}

function once(child) {
  return new Promise((resolve) => {
    if (child.exitCode !== null) {
      resolve();
      return;
    }
    child.on('exit', resolve);
  });
}

function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

module.exports = { boot, SERVER_LOG };
