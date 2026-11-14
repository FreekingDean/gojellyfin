export type Config = {
  ServerId: string;
  SingleSignOn: boolean;
};

export function readConfig(): Config {
  const content = document.querySelector('meta[name="gojellyfin-admin"]')?.getAttribute('content') ?? '';
  try {
    return JSON.parse(content) as Config;
  } catch {
    return { ServerId: '', SingleSignOn: false };
  }
}
