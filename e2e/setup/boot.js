const { spawn, spawnSync } = require('node:child_process');
const net = require('node:net');
const path = require('node:path');
const fs = require('node:fs');
const http = require('node:http');

const proxy = require('./proxy');
const radarr = require('./radarr');
const tmdb = require('./tmdb');

const ROOT = path.resolve(__dirname, '..', '..');
const BUILD = path.join(__dirname, '..', '.build');
const WEB = path.join(__dirname, '..', '.client');
const SERVER_LOG = path.join(BUILD, 'server.log');

const USERNAME = 'smoke';
const PASSWORD = 'smoke-password';
const LIBRARY = 'Smoke Movies';
const MOVIES = ['Fixture Alpha', 'Fixture Beta'];
const PLAYABLE = 'Fixture Playable';
const TRANSCODED = 'Fixture Transcoded';
const SHOWS = 'Smoke Shows';
const SERIES = 'Fixture Show';
const SEASON = 'Season 1';
const EPISODES = ['Fixture Pilot', 'Fixture Second'];
const DRAMA = 'Fixture Drama';
const FINALE = 'Fixture Finale';
const RESUMED = ['Fixture Alpha', 'Fixture Beta', 'Fixture Pilot'];
const SSO_USERNAME = 'sso-viewer';
const SSO_CLIENT_ID = 'gojellyfin';
const SSO_CLIENT_SECRET = 'not-a-real-secret';

// What TMDB calls trending and the library does not hold. The seed paints a
// poster for each under the size segment the server asks for.
const POSTER_SIZE = 'w780';
const WANTED_MOVIE = {
  id: 900001,
  name: 'Wanted Film',
  overview: 'A film the library does not hold yet.',
  tagline: 'Not here yet.',
  released: '2024-05-17',
  poster: '/wanted-film.png',
};
const WANTED_SHOW = {
  id: 900002,
  name: 'Wanted Show',
  overview: 'A show the library does not hold yet.',
  tagline: 'Coming, perhaps.',
  released: '2023-09-08',
  poster: '/wanted-show.png',
};

const CLIENT_IMAGE = 'jellyfin/jellyfin:12.4';

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

  const jellyfinData = path.join(BUILD, 'jellyfin-data');
  fs.rmSync(jellyfinData, { recursive: true, force: true });
  fs.mkdirSync(jellyfinData, { recursive: true });
  const jellyfin = JSON.parse(run(binaries.fixtures, ['jellyfin', jellyfinData]));

  const reserved = await reservePort();
  const apiPort = reserved.port;
  const posterRoot = path.join(BUILD, 'posters');
  fs.rmSync(posterRoot, { recursive: true, force: true });
  fs.mkdirSync(path.join(posterRoot, POSTER_SIZE), { recursive: true });

  const front = proxy.start({ webRoot: client, apiPort, posterRoot });
  const webPort = await listen(front);

  const catalogue = tmdb.start({
    imageBase: `http://127.0.0.1:${webPort}${proxy.POSTERS}/`,
    movies: [WANTED_MOVIE],
    shows: [WANTED_SHOW],
  });
  const cataloguePort = await listen(catalogue);

  const downloader = radarr.start();
  const downloaderUrl = `http://127.0.0.1:${await listen(downloader)}`;

  const serverEnv = clean({
    DATABASE_URL: databaseUrl,
    MEDIA_DIRECTORIES: path.dirname(media.playable),
    TMDB_API_KEY: 'e2e',
    TMDB_BASE_URL: `http://127.0.0.1:${cataloguePort}/3`,
    SOURCE_API_KEY_FIXTURES: 'e2e',
  });

  run(binaries.gojellyfin, ['migrate'], { env: serverEnv });
  run(binaries.gojellyfin, ['adduser', USERNAME], { env: serverEnv, input: PASSWORD + '\n' });
  run(binaries.fixtures, ['seed'], {
    env: {
      ...serverEnv,
      FIXTURE_MEDIA: media.playable,
      FIXTURE_TRANSCODE: media.transcoded,
      FIXTURE_DOWNLOADER: downloaderUrl,
      POSTER_DIR: posterRoot,
      UNOWNED_POSTER_DIR: path.join(posterRoot, POSTER_SIZE),
      POSTER_URL: `http://127.0.0.1:${webPort}${proxy.POSTERS}`,
    },
  });

  const baseUrl = `http://127.0.0.1:${webPort}`;
  const provider = await startProvider(binaries.fixtures, baseUrl);

  await reserved.release();
  const api = spawn(binaries.gojellyfin, ['server'], {
    env: {
      ...process.env,
      ...serverEnv,
      HTTP_PORT: String(apiPort),
      PUBLISHED_SERVER_URL: baseUrl,
      TRUSTED_PROXIES: '127.0.0.0/8,::1/128',
      OIDC_ISSUER_URL: provider.url,
      OIDC_CLIENT_ID: SSO_CLIENT_ID,
      OIDC_CLIENT_SECRET: SSO_CLIENT_SECRET,
      SERVER_ID: jellyfin.serverId,
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
      baseUrl,
      port: webPort,
      issuer: provider.url,
      ssoUsername: SSO_USERNAME,
      username: USERNAME,
      password: PASSWORD,
      library: LIBRARY,
      movies: MOVIES,
      playable: PLAYABLE,
      transcoded: TRANSCODED,
      shows: SHOWS,
      series: SERIES,
      season: SEASON,
      episodes: EPISODES,
      drama: DRAMA,
      finale: FINALE,
      wantedMovie: WANTED_MOVIE,
      wantedShow: WANTED_SHOW,
      resumed: RESUMED,
      jellyfin: {
        ...jellyfin,
        migrate: { command: binaries.migrate, args: ['--from', jellyfinData], env: serverEnv },
      },
      downloaderUrl,
      downloaderVersion: radarr.VERSION,
      downloaderRootFolder: radarr.ROOT_FOLDER,
      mediaDirectory: path.dirname(media.playable),
    },
    serverLog: () => log.join(''),
    async stop() {
      await Promise.all([front, catalogue, downloader].map(shut));
      await terminate(api);
      await terminate(provider.child);
      file.end();
      run(binaries.fixtures, ['drop'], { env: scratchEnv, allowFailure: true });
    },
  };
}

function build() {
  fs.mkdirSync(BUILD, { recursive: true });
  const gojellyfin = path.join(BUILD, 'gojellyfin');
  const fixtures = path.join(BUILD, 'fixtures');
  const migrate = path.join(BUILD, 'jellyfinmigrate');

  // The binary embeds the admin app, so it is built first or /admin answers 503.
  const admin = path.join(ROOT, 'admin');
  run('npm', ['ci'], { cwd: admin });
  run('npm', ['run', 'build'], { cwd: admin });

  run('go', ['build', '-o', gojellyfin, './cmd/gojellyfin'], { cwd: ROOT });
  run('go', ['build', '-o', fixtures, './e2e/fixtures'], { cwd: ROOT });
  run('go', ['build', '-o', migrate, './scripts/jellyfinmigrate'], { cwd: ROOT });

  return { gojellyfin, fixtures, migrate };
}

// The playable title is VP9 and Opus in WebM, which every Chromium build
// direct plays. The transcoded one is MPEG-4 Part 2, which no browser profile
// names, at 1440 lines, so the server has to encode it down to 1080p H.264.
function encodeMedia(seconds) {
  return { playable: encodePlayable(seconds), transcoded: encodeTranscoded() };
}

function encodeTranscoded() {
  const target = path.join(BUILD, 'media', 'transcoded.mkv');
  if (fs.existsSync(target)) {
    return target;
  }

  fs.mkdirSync(path.dirname(target), { recursive: true });
  run('ffmpeg', [
    '-hide_banner', '-loglevel', 'error', '-y',
    '-f', 'lavfi', '-i', 'testsrc2=size=1920x1440:rate=24:duration=20',
    '-f', 'lavfi', '-i', 'sine=frequency=440:duration=20',
    '-c:v', 'mpeg4', '-q:v', '8',
    '-c:a', 'aac', '-b:a', '64k',
    '-shortest', target,
  ]);

  return target;
}

function encodePlayable(seconds) {
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

// The provider is the same mock the Go tests sign in against, served from the
// fixtures binary because a browser has to be redirected to it. It prints its
// address once it is listening.
function startProvider(fixtures, baseUrl) {
  const child = spawn(fixtures, ['oidc'], {
    env: { ...process.env, OIDC_REDIRECT_URI: baseUrl + '/sso/oidc/callback', OIDC_USERNAME: SSO_USERNAME },
    stdio: ['ignore', 'pipe', 'pipe'],
  });

  return new Promise((resolve, reject) => {
    let output = '';
    child.stdout.on('data', (chunk) => {
      output += chunk.toString();
      const line = output.split('\n')[0];
      if (output.includes('\n')) {
        resolve({ child, url: line.trim() });
      }
    });
    child.stderr.on('data', (chunk) => {
      output += chunk.toString();
    });
    child.on('exit', (code) => reject(new Error(`the mock provider exited with ${code}:\n${output}`)));
  });
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
    TMDB_BASE_URL: '',
    SERVER_ID: '',
    ...overrides,
  };
}

function withDatabase(dsn, name) {
  const url = new URL(dsn);
  url.pathname = '/' + name;

  return url.toString();
}

// :0 rather than a fixed port, so a run can never take :8081 from `make dev`.
// The port stays held until the server is about to bind it, because every
// listen(0) the harness makes in between could otherwise be handed it.
function reservePort() {
  return new Promise((resolve, reject) => {
    const probe = net.createServer();
    probe.on('error', reject);
    probe.listen(0, '127.0.0.1', () => {
      const { port } = probe.address();
      resolve({ port, release: () => new Promise((done) => probe.close(done)) });
    });
  });
}

const connections = new WeakMap();

function listen(server) {
  const open = new Set();
  connections.set(server, open);
  server.on('connection', (socket) => {
    open.add(socket);
    socket.on('close', () => open.delete(socket));
  });
  return new Promise((resolve, reject) => {
    server.on('error', reject);
    server.listen(0, '127.0.0.1', () => resolve(server.address().port));
  });
}

// A client still connected, such as a WebView holding the websocket, keeps
// close from ever calling back, so whatever is still open is cut.
function shut(server) {
  return new Promise((resolve) => {
    server.close(resolve);
    for (const socket of connections.get(server) ?? []) socket.destroy();
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

async function terminate(child) {
  child.kill('SIGTERM');
  const killing = setTimeout(() => child.kill('SIGKILL'), 15000);
  await once(child);
  clearTimeout(killing);
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
