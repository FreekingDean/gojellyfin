const CREDENTIALS = 'jellyfin_credentials';
const DEVICE = 'gojellyfin_admin_device';

type StoredServer = {
  Id: string;
  AccessToken?: string;
  UserId?: string;
  ManualAddress?: string;
  LastConnectionMode?: number;
  DateLastAccessed?: number;
};

type Credentials = { Servers?: StoredServer[] };

function read(): Credentials {
  try {
    return (JSON.parse(localStorage.getItem(CREDENTIALS) ?? '') as Credentials) ?? {};
  } catch {
    return {};
  }
}

function entry(credentials: Credentials, serverId: string): StoredServer | undefined {
  const servers = credentials.Servers ?? [];

  return serverId ? servers.find((server) => server.Id === serverId) : servers.find((server) => server.AccessToken);
}

export function storedToken(serverId: string): string | undefined {
  return entry(read(), serverId)?.AccessToken;
}

export function storeToken(serverId: string, token: string, userId: string): void {
  const credentials = read();
  const servers = credentials.Servers ?? [];
  let server = entry(credentials, serverId);
  if (!server) {
    server = { Id: serverId, ManualAddress: location.origin, LastConnectionMode: 2 };
    servers.push(server);
  }
  server.AccessToken = token;
  server.UserId = userId;
  server.DateLastAccessed = Date.now();
  credentials.Servers = servers;
  localStorage.setItem(CREDENTIALS, JSON.stringify(credentials));
}

export function forgetToken(serverId: string): void {
  const credentials = read();
  const server = entry(credentials, serverId);
  if (!server) {
    return;
  }
  delete server.AccessToken;
  delete server.UserId;
  localStorage.setItem(CREDENTIALS, JSON.stringify(credentials));
}

export function deviceId(): string {
  let id = localStorage.getItem(DEVICE);
  if (!id) {
    id = 'admin-' + crypto.randomUUID();
    localStorage.setItem(DEVICE, id);
  }

  return id;
}
