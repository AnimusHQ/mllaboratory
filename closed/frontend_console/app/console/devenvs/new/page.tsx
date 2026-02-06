import { PageHeader, PageShell } from '@/components/ui/page-shell';
import { getActiveProjectId } from '@/lib/server-context';

import { NewDevEnvForm } from './new-devenv-form';

export default function NewDevEnvPage() {
  const projectId = getActiveProjectId();

  return (
    <PageShell>
      <PageHeader
        title="Новый DevEnv"
        description="Создание из Git‑репозитория с фиксированием ref/commit и TTL."
      />
      <NewDevEnvForm projectId={projectId} />
    </PageShell>
  );
}
