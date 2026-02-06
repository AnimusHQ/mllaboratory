import { cookies } from 'next/headers';

import type { components } from '@/lib/gateway-openapi';
import { GatewayAPIError, gatewayFetchJSON } from '@/lib/gateway-client';

export type GatewaySession =
  | {
      mode: 'authenticated';
      subject: string;
      email?: string;
      roles: string[];
    }
  | { mode: 'unauthenticated' }
  | { mode: 'error'; error: string };

export async function getGatewaySession(): Promise<GatewaySession> {
  const cookieStore = await cookies();
  const cookieHeader = cookieStore.toString();

  try {
    const session = await gatewayFetchJSON<components['schemas']['SessionResponse']>('/auth/session', {
      method: 'GET',
      headers: cookieHeader ? { Cookie: cookieHeader } : undefined,
      retry: false,
    });
    return {
      mode: 'authenticated',
      subject: session.subject,
      email: session.email,
      roles: session.roles ?? [],
    };
  } catch (err) {
    if (err instanceof GatewayAPIError) {
      if (err.status === 401) {
        return { mode: 'unauthenticated' };
      }
    }
    return { mode: 'error', error: 'gateway_session_unavailable' };
  }
}
