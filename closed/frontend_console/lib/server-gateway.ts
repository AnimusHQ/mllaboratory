import { cookies } from 'next/headers';

import { gatewayFetchJSON, type GatewayFetchOptions } from '@/lib/gateway-client';
import { getActiveProjectId } from '@/lib/server-context';

export async function gatewayServerFetchJSON<T>(path: string, init: GatewayFetchOptions = {}): Promise<T> {
  const cookieHeader = cookies().toString();
  const headers = new Headers(init.headers ?? {});
  if (cookieHeader) {
    headers.set('Cookie', cookieHeader);
  }
  if (!headers.has('X-Project-Id') && !headers.has('X-Project-ID')) {
    const projectId = await getActiveProjectId();
    if (projectId) {
      headers.set('X-Project-Id', projectId);
    }
  }
  return gatewayFetchJSON<T>(path, {
    ...init,
    headers,
  });
}
