import { PageHeader, PageShell } from '@/components/ui/page-shell';
import { getGatewaySession } from '@/lib/session';
import { deriveEffectiveRole } from '@/lib/rbac';
import { getActiveProjectId } from '@/lib/server-context';

import { RunCreateForm } from './run-create-form';

export default async function NewRunPage() {
  const projectId = await getActiveProjectId();
  const session = await getGatewaySession();
  const role = deriveEffectiveRole(session.mode === 'authenticated' ? session.roles : []);

  return (
    <PageShell>
      <PageHeader
        title="Новый RunSpec"
        description="Создайте неизменяемую спецификацию запуска. Все данные фиксируются и используются для детерминированного планирования."
      />
      <RunCreateForm projectId={projectId} role={role} />
    </PageShell>
  );
}
