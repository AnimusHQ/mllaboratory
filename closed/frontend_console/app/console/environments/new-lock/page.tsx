import { PageHeader, PageShell } from '@/components/ui/page-shell';
import { getActiveProjectId } from '@/lib/server-context';

import { NewLockForm } from './new-lock-form';

export default function NewLockPage() {
  const projectId = getActiveProjectId();

  return (
    <PageShell>
      <PageHeader
        title="Новый Environment Lock"
        description="Создаёт неизменяемую блокировку окружения. Верификация образов выполняется до фиксации."
      />
      <NewLockForm projectId={projectId} />
    </PageShell>
  );
}
