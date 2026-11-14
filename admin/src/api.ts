import createClient from 'openapi-fetch';

import type { components, paths } from './api.gen';
import { deviceId } from './session';

export type Schemas = components['schemas'];

let token: string | undefined;

export function setToken(value: string | undefined): void {
  token = value;
}

function authorization(): string {
  const parts = ['Client="gojellyfin admin"', 'Device="Browser"', `DeviceId="${deviceId()}"`, 'Version="1"'];
  if (token) {
    parts.push(`Token="${token}"`);
  }

  return 'MediaBrowser ' + parts.join(', ');
}

export const api = createClient<paths>({ baseUrl: location.origin });

api.use({
  onRequest({ request }) {
    request.headers.set('Authorization', authorization());

    return request;
  },
});
