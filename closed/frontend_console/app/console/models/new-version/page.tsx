import { PageHeader, PageShell } from '@/components/ui/page-shell';
import { getActiveProjectId } from '@/lib/server-context';

import { NewModelVersionForm } from './new-version-form';

export default function NewModelVersionPage() {
  const projectId = getActiveProjectId();

  return (
    <PageShell>
      <PageHeader
        title="Новая версия модели"
        description="Фиксирует provenance: run_id, artifact_ids, dataset_version_ids."
      />
      <NewModelVersionForm projectId={projectId} />
    </PageShell>
  );
}
