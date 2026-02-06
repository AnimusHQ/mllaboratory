import { PageHeader, PageShell } from '@/components/ui/page-shell';
import { deriveEffectiveRole } from '@/lib/rbac';
import { getActiveProjectId } from '@/lib/server-context';
import { getGatewaySession } from '@/lib/session';

import { NewModelVersionForm } from './new-version-form';

export default async function NewModelVersionPage() {
  const projectId = getActiveProjectId();
  const session = await getGatewaySession();
  const role = deriveEffectiveRole(session.mode === 'authenticated' ? session.roles : []);

  return (
    <PageShell>
      <PageHeader
        title="Новая версия модели"
        description="Фиксирует provenance: run_id, artifact_ids, dataset_version_ids."
      />
      <NewModelVersionForm projectId={projectId} role={role} />
    </PageShell>
  );
}
