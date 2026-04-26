import type { GatewaySession } from '@/lib/session';

export function requiresAuthGate(session: GatewaySession): boolean {
  return session.mode === 'unauthenticated';
}
