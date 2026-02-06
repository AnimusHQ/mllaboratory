import { DatasetCreateForm } from '@/app/console/datasets/dataset-create-form';
import { DatasetUploadForm } from '@/app/console/datasets/dataset-upload-form';
import { DatasetVersionsTable } from '@/components/console/dataset-versions-table';
import { DatasetsTable } from '@/components/console/datasets-table';
import { ErrorState } from '@/components/console/error-state';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { PageHeader, PageSection, PageShell } from '@/components/ui/page-shell';
import { GatewayAPIError } from '@/lib/gateway-client';
import type { components } from '@/lib/gateway-openapi';
import { deriveEffectiveRole } from '@/lib/rbac';
import { getGatewaySession } from '@/lib/session';
import { gatewayServerFetchJSON } from '@/lib/server-gateway';

const copy = {
  title: 'Наборы данных',
  description: 'Реестр датасетов, версии и контроль качества. Операции протоколируются аудитом.',
};

type SearchParams = {
  dataset_id?: string;
};

export default async function DatasetsPage({ searchParams }: { searchParams?: Promise<SearchParams> }) {
  const params = (await searchParams) ?? {};
  let datasets: components['schemas']['Dataset'][] = [];
  let versions: components['schemas']['DatasetVersion'][] = [];
  let error: GatewayAPIError | null = null;
  const datasetId = params.dataset_id?.trim() ?? '';
  const session = await getGatewaySession();
  const role = deriveEffectiveRole(session.mode === 'authenticated' ? session.roles : []);

  try {
    const data = await gatewayServerFetchJSON<components['schemas']['DatasetListResponse']>(
      '/api/dataset-registry/datasets?limit=200',
    );
    datasets = data.datasets ?? [];
    if (datasetId) {
      const versionResponse = await gatewayServerFetchJSON<components['schemas']['DatasetVersionListResponse']>(
        `/api/dataset-registry/datasets/${datasetId}/versions?limit=200`,
      );
      versions = versionResponse.versions ?? [];
    }
  } catch (err) {
    if (err instanceof GatewayAPIError) {
      error = err;
    } else {
      error = new GatewayAPIError(500, 'gateway_unexpected');
    }
  }

  return (
    <PageShell>
      <PageHeader title={copy.title} description={copy.description} />

      {error ? <ErrorState code={error.code} requestId={error.requestId} status={error.status} details={error.details} message={error.message} retryable={error.retryable} /> : null}

      <PageSection title="Регистрация набора">
        <DatasetCreateForm role={role} />
      </PageSection>

      <PageSection title="Список наборов данных">
        <Card>
          <CardHeader>
            <CardTitle>Datasets</CardTitle>
            <CardDescription>Используйте фильтры и выберите dataset_id для работы с версиями.</CardDescription>
          </CardHeader>
          <CardContent>{datasets ? <DatasetsTable datasets={datasets} /> : <p className="text-sm">Загрузка…</p>}</CardContent>
        </Card>
      </PageSection>

      <PageSection title="Версии набора" description="Загрузка и управление версионными объектами.">
        {datasetId ? (
          <>
            <DatasetUploadForm datasetId={datasetId} role={role} />
            <DatasetVersionsTable versions={versions} role={role} />
          </>
        ) : (
          <Card>
            <CardHeader>
              <CardTitle>Dataset ID не выбран</CardTitle>
              <CardDescription>Выберите dataset_id в таблице или задайте его вручную.</CardDescription>
            </CardHeader>
          </Card>
        )}
      </PageSection>
    </PageShell>
  );
}
