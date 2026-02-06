import { ArtifactsTable } from '@/components/console/artifacts-table';
import { ErrorState } from '@/components/console/error-state';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { PageHeader, PageShell } from '@/components/ui/page-shell';
import { GatewayAPIError } from '@/lib/gateway-client';
import type { components } from '@/lib/gateway-openapi';
import { deriveEffectiveRole } from '@/lib/rbac';
import { getGatewaySession } from '@/lib/session';
import { gatewayServerFetchJSON } from '@/lib/server-gateway';

export default async function ArtifactsPage({
  searchParams,
}: {
  searchParams?: { run_id?: string };
}) {
  const session = await getGatewaySession();
  const role = deriveEffectiveRole(session.mode === 'authenticated' ? session.roles : []);
  const runId = searchParams?.run_id?.trim() ?? '';
  let data: components['schemas']['ExperimentRunArtifactListResponse'] | null = null;
  let error: GatewayAPIError | null = null;

  if (runId) {
    try {
      data = await gatewayServerFetchJSON<components['schemas']['ExperimentRunArtifactListResponse']>(
        `/api/experiments/experiment-runs/${runId}/artifacts?limit=200`,
      );
    } catch (err) {
      if (err instanceof GatewayAPIError) {
        error = err;
      } else {
        error = new GatewayAPIError(500, 'gateway_unexpected');
      }
    }
  }

  return (
    <PageShell>
      <PageHeader
        title="Артефакты"
        description="Список артефактов запуска, фильтрация и скачивание. Все операции фиксируются аудитом."
        actions={
          <Button variant="secondary" size="sm">
            Загрузить артефакт
          </Button>
        }
      />
      <Card>
        <CardHeader>
          <CardTitle>Поиск по Run ID</CardTitle>
          <CardDescription>Укажите идентификатор запуска для просмотра артефактов.</CardDescription>
        </CardHeader>
        <CardContent>
          <form className="flex flex-wrap items-center gap-3" method="GET">
            <Input name="run_id" defaultValue={runId} placeholder="run_id" className="max-w-xs" />
            <Button type="submit" size="sm">
              Загрузить
            </Button>
          </form>
        </CardContent>
      </Card>
      {error ? (
        <ErrorState code={error.code} requestId={error.requestId} status={error.status} details={error.details} message={error.message} retryable={error.retryable} />
      ) : null}
      {data ? (
        <Card>
          <CardHeader>
            <CardTitle>Артефакты запуска</CardTitle>
            <CardDescription>Сортировка по времени создания (desc).</CardDescription>
          </CardHeader>
          <CardContent>
            <ArtifactsTable artifacts={data.artifacts ?? []} role={role} />
          </CardContent>
        </Card>
      ) : null}
    </PageShell>
  );
}
