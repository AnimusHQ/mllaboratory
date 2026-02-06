import { PageHeader, PageShell } from '@/components/ui/page-shell';
import { deriveEffectiveRole } from '@/lib/rbac';
import { getActiveProjectId } from '@/lib/server-context';
import { getGatewaySession } from '@/lib/session';

import { NewDevEnvForm } from './new-devenv-form';

export default async function NewDevEnvPage() {
  const projectId = getActiveProjectId();
  const session = await getGatewaySession();
  const role = deriveEffectiveRole(session.mode === 'authenticated' ? session.roles : []);

  return (
    <PageShell>
      <PageHeader
        title="Новый DevEnv"
        description="Создание из Git‑репозитория с фиксированием ref/commit и TTL."
      />
      <NewDevEnvForm projectId={projectId} role={role} />
    </PageShell>
  );
}
