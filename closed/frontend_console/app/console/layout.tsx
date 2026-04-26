import type { ReactNode } from 'react';

import { AppShell } from '@/components/console/app-shell';
import { getGatewaySession } from '@/lib/session';

export default async function ConsoleLayout({ children }: { children: ReactNode }) {
  const session = await getGatewaySession();
  return <AppShell session={session}>{children}</AppShell>;
}
