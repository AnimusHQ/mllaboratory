import { PageHeader, PageShell } from '@/components/ui/page-shell';
import { deriveEffectiveRole } from '@/lib/rbac';
import { getActiveProjectId } from '@/lib/server-context';
import { getGatewaySession } from '@/lib/session';

import { NewLockForm } from './new-lock-form';

export default async function NewLockPage() {
  const projectId = await getActiveProjectId();
  const session = await getGatewaySession();
  const role = deriveEffectiveRole(session.mode === 'authenticated' ? session.roles : []);

  return (
    <PageShell>
      <PageHeader
        title="Новый Environment Lock"
        description="Создаёт неизменяемую блокировку окружения. Верификация образов выполняется до фиксации."
      />
      <NewLockForm projectId={projectId} role={role} />
    </PageShell>
  );
}
