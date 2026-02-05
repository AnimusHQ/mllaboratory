import { cookies } from 'next/headers';

import { gatewayFetchJSON, type GatewayFetchOptions } from '@/lib/gateway-client';

export async function gatewayServerFetchJSON<T>(path: string, init: GatewayFetchOptions = {}): Promise<T> {
  const cookieHeader = cookies().toString();
  const headers = new Headers(init.headers ?? {});
  if (cookieHeader) {
    headers.set('Cookie', cookieHeader);
  }
  return gatewayFetchJSON<T>(path, {
    ...init,
    headers,
  });
}
