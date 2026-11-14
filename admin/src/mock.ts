import type { Schemas } from './api';
import type { Tone } from './ui';

export const mocked = new URLSearchParams(location.search).has('mock');

function ago(minutes: number): string {
  return new Date(Date.now() - minutes * 60000).toISOString();
}

const HOUR = 60;
const DAY = 24 * HOUR;

export const user: Schemas['UserDto'] = { Name: 'dean', Policy: { IsAdministrator: true } as Schemas['UserPolicy'] };

export const build = '12.0.0+gojellyfin.0.0.214';

export const titles: Schemas['RequestedTitle'][] = [
  {
    ItemId: '1',
    Name: 'The Brutalist',
    Kind: 'Movie',
    Requests: [{ Id: 'r1', UserId: 'u3', UserName: 'sam', Status: 'Pending', CreatedAt: ago(41) }],
    Submissions: [],
  },
  {
    ItemId: '2',
    Name: 'Shōgun',
    Kind: 'Series',
    Requests: [
      { Id: 'r2', UserId: 'u4', UserName: 'alex', Status: 'Pending', CreatedAt: ago(3 * HOUR) },
      { Id: 'r3', UserId: 'u2', UserName: 'maya', Status: 'Pending', CreatedAt: ago(5 * HOUR) },
    ],
    Submissions: [],
  },
  {
    ItemId: '3',
    Name: 'Anora',
    Kind: 'Movie',
    Requests: [{ Id: 'r4', UserId: 'u4', UserName: 'alex', Status: 'Pending', CreatedAt: ago(DAY) }],
    Submissions: [],
  },
  {
    ItemId: '4',
    Name: 'Conclave',
    Kind: 'Movie',
    Requests: [{ Id: 'r5', UserId: 'u1', UserName: 'dean', Status: 'Approved', CreatedAt: ago(DAY + 2 * HOUR) }],
    Submissions: [
      { SourceName: 'Radarr HD', AcceptedAt: ago(DAY) },
      { SourceName: 'Radarr 4K', Error: '401 Unauthorized' },
    ],
  },
  {
    ItemId: '5',
    Name: 'Slow Horses',
    Kind: 'Series',
    Requests: [{ Id: 'r6', UserId: 'u2', UserName: 'maya', Status: 'Approved', CreatedAt: ago(2 * DAY) }],
    Submissions: [{ SourceName: 'Sonarr', AcceptedAt: ago(2 * DAY) }],
  },
  {
    ItemId: '6',
    Name: 'Megalopolis',
    Kind: 'Movie',
    Requests: [{ Id: 'r7', UserId: 'u3', UserName: 'sam', Status: 'Declined', CreatedAt: ago(4 * DAY) }],
    Submissions: [],
  },
];

export const reports: Schemas['Report'][] = [
  {
    Id: 'p1',
    ItemId: '11',
    ItemName: 'Heat',
    Kind: 'Unplayable',
    Origin: 'Probe',
    Note: 'ffprobe could not read the file',
    Status: 'Open',
    Path: '/media/movies/Heat (1995)/Heat (1995) Bluray-1080p.mkv',
    CreatedAt: ago(HOUR),
    Replaceable: true,
  },
  {
    Id: 'p2',
    ItemId: '12',
    ItemName: 'Severance S2 E3',
    Kind: 'Audio',
    Origin: 'User',
    Note: 'Sound drops out after the cold open',
    Status: 'Open',
    UserName: 'maya',
    Path: '/media/tv/Severance/Season 02/Severance - S02E03 - Who Is Alive WEBDL-2160p.mkv',
    CreatedAt: ago(6 * HOUR),
    Replaceable: true,
  },
  {
    Id: 'p3',
    ItemId: '13',
    ItemName: 'Paddington 2',
    Kind: 'Unplayable',
    Origin: 'Delete',
    Note: '',
    Status: 'Open',
    UserName: 'sam',
    Path: '/media/movies/Paddington 2 (2017)/Paddington 2 (2017) WEBDL-1080p.mkv',
    CreatedAt: ago(DAY),
    Replaceable: false,
  },
  {
    Id: 'p4',
    ItemId: '14',
    ItemName: 'The Bear S3 E1',
    Kind: 'Subtitles',
    Origin: 'User',
    Note: 'English subtitles are out of sync',
    Status: 'Resolved',
    UserName: 'alex',
    Path: '/media/tv/The Bear/Season 03/The Bear - S03E01 - Tomorrow WEBDL-1080p.mkv',
    CreatedAt: ago(3 * DAY),
    Replaceable: true,
  },
];

export type Health = { name: string; tone: Tone; detail: string };

export const health: Health[] = [
  { name: 'Database', tone: 'ok', detail: 'Postgres, schema current' },
  { name: 'Background work', tone: 'ok', detail: 'Temporal, 3 queues served' },
  { name: 'Metadata', tone: 'ok', detail: 'TMDB key set' },
  { name: 'Single sign-on', tone: 'ok', detail: 'auth.example.net' },
  { name: 'Transcoding', tone: 'ok', detail: 'ffmpeg found, 1 of 4 slots busy' },
  { name: 'Radarr HD', tone: 'ok', detail: 'v5.14, answered 4 min ago' },
  { name: 'Radarr 4K', tone: 'warn', detail: '401 Unauthorized, 2 h ago' },
  { name: 'Tracing', tone: 'off', detail: 'Off. Set OTEL_EXPORTER_OTLP_ENDPOINT' },
];

export type Watching = { user: string; device: string; title: string; method: 'Direct' | 'Remux' | 'Encoding 1080p'; progress: number };

export const watching: Watching[] = [
  { user: 'maya', device: 'Android TV', title: 'Severance S2 E4', method: 'Remux', progress: 62 },
  { user: 'dean', device: 'Chrome', title: 'Dune: Part Two', method: 'Encoding 1080p', progress: 18 },
  { user: 'sam', device: 'Swiftfin', title: 'The Bear S3 E1', method: 'Direct', progress: 81 },
];

export const library = { movies: 2204, shows: 186, episodes: 9870, refreshed: ago(38), unprobed: 12, unidentified: 3 };

export type Activity = { at: string; text: string; tone?: Tone };

export const activity: Activity[] = [
  { at: ago(2), text: 'maya started playing Severance S2 E4' },
  { at: ago(41), text: 'sam requested The Brutalist' },
  { at: ago(HOUR), text: 'Probe reported Heat as unplayable', tone: 'warn' },
  { at: ago(2 * HOUR), text: 'Radarr 4K refused Conclave: 401 Unauthorized', tone: 'bad' },
  { at: ago(3 * HOUR), text: 'alex signed in with single sign-on for the first time' },
  { at: ago(5 * HOUR), text: 'dean changed the password for sam' },
  { at: ago(DAY), text: 'sam pressed Delete on Paddington 2' },
  { at: ago(DAY + HOUR), text: 'Refresh Libraries finished: 14 titles added, 2 removed' },
];

export type Downloader = {
  id: string;
  name: string;
  kind: 'Radarr' | 'Sonarr';
  version: string;
  url: string;
  variable: string;
  contacted: string;
  error?: string;
  paths: { remote: string; local: string }[];
  synced: string;
  files: number;
  requests?: { profile: string; folder: string; tag?: string };
};

export const downloaders: Downloader[] = [
  {
    id: 'radarr-hd',
    name: 'Radarr HD',
    kind: 'Radarr',
    version: '5.14',
    url: 'http://radarr.media.svc:7878',
    variable: 'SOURCE_API_KEY_RADARR',
    contacted: ago(4),
    paths: [{ remote: '/movies', local: '/media/movies' }],
    synced: ago(38),
    files: 2140,
    requests: { profile: 'HD-1080p', folder: '/movies', tag: 'requested' },
  },
  {
    id: 'radarr-4k',
    name: 'Radarr 4K',
    kind: 'Radarr',
    version: '5.14',
    url: 'http://radarr-4k.media.svc:7878',
    variable: 'SOURCE_API_KEY_RADARR_4K',
    contacted: ago(2 * HOUR),
    error: '401 Unauthorized',
    paths: [{ remote: '/movies-4k', local: '/media/movies-4k' }],
    synced: ago(2 * HOUR),
    files: 310,
    requests: { profile: 'Ultra-HD', folder: '/movies-4k' },
  },
  {
    id: 'sonarr',
    name: 'Sonarr',
    kind: 'Sonarr',
    version: '4.0',
    url: 'http://sonarr.media.svc:8989',
    variable: 'SOURCE_API_KEY_SONARR',
    contacted: ago(4),
    paths: [{ remote: '/tv', local: '/media/tv' }],
    synced: ago(36),
    files: 9870,
  },
];

export const pickers = {
  variables: ['SOURCE_API_KEY_RADARR', 'SOURCE_API_KEY_RADARR_4K', 'SOURCE_API_KEY_SONARR'],
  remoteFolders: ['/movies', '/movies-4k', '/tv'],
  localFolders: ['/media/movies', '/media/movies-4k', '/media/tv'],
  profiles: ['Any', 'HD-1080p', 'Ultra-HD'],
  tags: ['4k', 'kids', 'requested'],
};

export type Library = {
  id: 'movies' | 'shows';
  name: string;
  kind: Downloader['kind'];
  counts: string;
  synced: string;
};

export const libraries: Library[] = [
  { id: 'movies', name: 'Movies', kind: 'Radarr', counts: '2,204 movies', synced: ago(38) },
  { id: 'shows', name: 'Shows', kind: 'Sonarr', counts: '186 shows, 9,870 episodes', synced: ago(36) },
];

export type Person = {
  id: string;
  name: string;
  admin: boolean;
  disabled: boolean;
  sso: boolean;
  password: boolean;
  libraries: string[] | 'all';
  canRequest: boolean;
  needsApproval: boolean;
  quota?: number;
  used: number;
  seen?: string;
  devices: { name: string; app: string; seen: string }[];
};

export const people: Person[] = [
  {
    id: 'u1',
    name: 'dean',
    admin: true,
    disabled: false,
    sso: true,
    password: true,
    libraries: 'all',
    canRequest: true,
    needsApproval: false,
    used: 1,
    seen: ago(3),
    devices: [
      { name: 'Chrome', app: 'Jellyfin Web 10.10.0', seen: ago(3) },
      { name: 'Living Room', app: 'Android TV 0.18.4', seen: ago(2 * DAY) },
    ],
  },
  {
    id: 'u2',
    name: 'maya',
    admin: false,
    disabled: false,
    sso: true,
    password: false,
    libraries: 'all',
    canRequest: true,
    needsApproval: true,
    quota: 5,
    used: 2,
    seen: ago(2),
    devices: [{ name: 'Bedroom TV', app: 'Android TV 0.18.4', seen: ago(2) }],
  },
  {
    id: 'u3',
    name: 'sam',
    admin: false,
    disabled: false,
    sso: false,
    password: true,
    libraries: ['Movies'],
    canRequest: true,
    needsApproval: true,
    quota: 5,
    used: 2,
    seen: ago(20),
    devices: [{ name: 'iPhone', app: 'Swiftfin 1.3', seen: ago(20) }],
  },
  {
    id: 'u4',
    name: 'alex',
    admin: false,
    disabled: false,
    sso: true,
    password: false,
    libraries: 'all',
    canRequest: true,
    needsApproval: true,
    quota: 3,
    used: 2,
    seen: ago(3 * HOUR),
    devices: [{ name: 'Firefox', app: 'Jellyfin Web 10.10.0', seen: ago(3 * HOUR) }],
  },
  {
    id: 'u5',
    name: 'guest',
    admin: false,
    disabled: true,
    sso: false,
    password: true,
    libraries: ['Movies'],
    canRequest: false,
    needsApproval: true,
    used: 0,
    seen: ago(40 * DAY),
    devices: [],
  },
];

export type Job = {
  name: string;
  title: string;
  does: string;
  state: 'Idle' | 'Running';
  started?: string;
  last?: { at: string; ok: boolean; took: string; error?: string };
};

export const jobs: Job[] = [
  { name: 'RefreshLibraries', title: 'Refresh libraries', does: 'Reads every downloader and updates what each library holds', state: 'Idle', last: { at: ago(38), ok: true, took: '2 min 10 s' } },
  { name: 'ProbeFiles', title: 'Probe files', does: 'Reads the streams of every file not yet probed', state: 'Running', started: ago(6), last: { at: ago(DAY), ok: true, took: '41 min' } },
  { name: 'RefreshMetadata', title: 'Refresh metadata', does: 'Identifies titles and fetches artwork for those missing it', state: 'Idle', last: { at: ago(5 * HOUR), ok: false, took: '12 s', error: 'TMDB answered 503 three times' } },
  { name: 'SweepItems', title: 'Sweep removed titles', does: 'Removes titles no downloader reports a file for any more', state: 'Idle', last: { at: ago(DAY), ok: true, took: '3 s' } },
];

export type Queue = { name: string; holds: string; waiting: number; running: number };

export const queues: Queue[] = [
  { name: 'Refresh', holds: 'Library and downloader listings', waiting: 0, running: 0 },
  { name: 'Default', holds: 'Per-title refreshes and probes', waiting: 12, running: 4 },
  { name: 'Metadata', holds: 'Identifies against TMDB', waiting: 3, running: 1 },
];

export const temporal = { address: 'temporal-frontend.temporal.svc:7233', namespace: 'gojellyfin' };

export type Session = { user: string; device: string; app: string; address: string; seen: string; playing?: Watching };

export const sessions: Session[] = [
  { user: 'maya', device: 'Bedroom TV', app: 'Android TV 0.18.4', address: '10.0.4.21', seen: ago(0), playing: watching[0] },
  { user: 'dean', device: 'Chrome', app: 'Jellyfin Web 10.10.0', address: '10.0.4.12', seen: ago(0), playing: watching[1] },
  { user: 'sam', device: 'iPhone', app: 'Swiftfin 1.3', address: '172.58.14.9', seen: ago(1), playing: watching[2] },
  { user: 'alex', device: 'Firefox', app: 'Jellyfin Web 10.10.0', address: '10.0.4.40', seen: ago(3 * HOUR) },
];

export type Setting = { label: string; value: string; variable: string; tone?: Tone };
export type SettingGroup = { title: string; state: 'On' | 'Off' | ''; why?: string; settings: Setting[] };

export const settings: SettingGroup[] = [
  {
    title: 'Server',
    state: '',
    settings: [
      { label: 'Published address', value: 'https://jellyfin.example.net', variable: 'PUBLISHED_SERVER_URL' },
      { label: 'Server id', value: '5f0a1b2c3d4e4f6a8b9c0d1e2f3a4b5c', variable: 'SERVER_ID' },
      { label: 'Port', value: '8081', variable: 'HTTP_PORT' },
      { label: 'Media folders', value: '/media', variable: 'MEDIA_DIRECTORIES' },
    ],
  },
  {
    title: 'Single sign-on',
    state: 'On',
    settings: [
      { label: 'Provider', value: 'https://auth.example.net', variable: 'OIDC_ISSUER_URL' },
      { label: 'Client id', value: 'gojellyfin', variable: 'OIDC_CLIENT_ID' },
      { label: 'Client secret', value: 'Set', variable: 'OIDC_CLIENT_SECRET' },
      { label: 'Allowed group', value: 'media', variable: 'OIDC_ALLOWED_GROUP' },
      { label: 'Administrator group', value: 'media-admins', variable: 'OIDC_ADMIN_GROUP' },
      { label: 'Link existing local users', value: 'No', variable: 'OIDC_LINK_EXISTING_USERS' },
    ],
  },
  {
    title: 'Metadata',
    state: 'On',
    settings: [{ label: 'TMDB key', value: 'Set', variable: 'TMDB_API_KEY' }],
  },
  {
    title: 'Transcoding',
    state: 'On',
    settings: [
      { label: 'Encodes at once, per pod', value: '4', variable: 'TRANSCODER_JOBS' },
      { label: 'Stalled client timeout', value: '30 s', variable: 'TRANSCODER_STALL_TIMEOUT' },
    ],
  },
  {
    title: 'Background work',
    state: 'On',
    settings: [
      { label: 'Temporal address', value: temporal.address, variable: 'TEMPORAL_HOSTPORT' },
      { label: 'Namespace', value: temporal.namespace, variable: 'TEMPORAL_NAMESPACE' },
    ],
  },
  {
    title: 'Tracing',
    state: 'Off',
    why: 'Set OTEL_EXPORTER_OTLP_ENDPOINT to send a span per operation to a collector.',
    settings: [],
  },
];

export const branding = {
  disclaimer: 'Welcome. Ask dean if you need an account.',
  css: '.skinHeader { backdrop-filter: blur(12px); }',
};

export type ApiKey = { name: string; created: string; used?: string };

export const apiKeys: ApiKey[] = [
  { name: 'Home Assistant', created: ago(90 * DAY), used: ago(5) },
  { name: 'Jellyseerr (retired)', created: ago(200 * DAY), used: ago(60 * DAY) },
];

export type Shelf = { slug: string; name: string; shows: string; on: boolean; note?: string };

export const shelves: Shelf[] = [
  { slug: 'recently-aired', name: 'Recently Aired', shows: 'Episodes that aired lately', on: true },
  { slug: 'new-releases', name: 'New Releases', shows: 'Movies released lately', on: true },
  { slug: 'trending-movies', name: 'Trending Movies', shows: 'Movies the library lacks, which people can request', on: true },
  { slug: 'trending-shows', name: 'Trending Shows', shows: 'Shows the library lacks, which people can request', on: false },
  { slug: 'hidden-gems', name: 'Hidden Gems', shows: 'Highly rated movies each person has not watched', on: false },
  { slug: 'short-on-time', name: 'Short on Time', shows: 'Short, well rated movies each person has not watched', on: false },
  { slug: 'spooky-season', name: 'Spooky Season', shows: 'Horror movies', on: false, note: 'Only in October' },
  { slug: 'genre-spotlight', name: 'Genre Spotlight', shows: 'A different genre each week', on: false },
  { slug: 'because-you-watched', name: 'Because You Watched', shows: 'Movies like one each person finished', on: false },
];
