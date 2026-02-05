import { DatasetsTable } from '@/components/console/datasets-table';
import { ErrorState } from '@/components/console/error-state';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { PageHeader, PageShell } from '@/components/ui/page-shell';
import { GatewayAPIError } from '@/lib/gateway-client';
import type { components } from '@/lib/gateway-openapi';
import { gatewayServerFetchJSON } from '@/lib/server-gateway';

export default async function DatasetsPage() {
  let data: components['schemas']['DatasetListResponse'] | null = null;
  let error: GatewayAPIError | null = null;

  try {
    data = await gatewayServerFetchJSON<components['schemas']['DatasetListResponse']>(
      '/api/dataset-registry/datasets?limit=200',
    );
  } catch (err) {
    if (err instanceof GatewayAPIError) {
      error = err;
    } else {
      error = new GatewayAPIError(500, 'gateway_unexpected');
    }
  }

  return (
    <PageShell>
      <PageHeader
        title="Наборы данных"
        description="Реестр датасетов, версии и контроль качества. Операции протоколируются аудитом."
        actions={
          <Button variant="secondary" size="sm">
            Зарегистрировать набор
          </Button>
        }
      />
      {error ? (
        <ErrorState code={error.code} requestId={error.requestId} status={error.status} details={error.details} />
      ) : null}
      <Card>
        <CardHeader>
          <CardTitle>Список наборов данных</CardTitle>
          <CardDescription>Данные получены через Gateway API. Сортировка по времени создания.</CardDescription>
        </CardHeader>
        <CardContent>
          {data ? <DatasetsTable datasets={data.datasets ?? []} /> : <p className="text-sm">Загрузка…</p>}
        </CardContent>
      </Card>
    </PageShell>
  );
}
