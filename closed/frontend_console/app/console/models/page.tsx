import Link from 'next/link';

import { CopyButton } from '@/components/console/copy-button';
import { ErrorState } from '@/components/console/error-state';
import { Pagination } from '@/components/console/pagination';
import { ModelVersionsTable } from '@/components/console/model-versions-table';
import { StatusPill } from '@/components/console/status-pill';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Table, TableContainer, TableEmpty } from '@/components/ui/table';
import { PageHeader, PageSection, PageShell } from '@/components/ui/page-shell';
import { GatewayAPIError } from '@/lib/gateway-client';
import type { components } from '@/lib/gateway-openapi';
import { formatDateTime } from '@/lib/format';
import { getActiveProjectId } from '@/lib/server-context';
import { getGatewaySession } from '@/lib/session';
import { gatewayServerFetchJSON } from '@/lib/server-gateway';
import { deriveEffectiveRole } from '@/lib/rbac';

import { ModelCreateForm } from './model-create-form';
import { ModelFilters } from './model-filters';
import { ModelSelector } from './model-selector';

const copy = {
  datasets: {
    title: 'Наборы данных',
    description: 'Регистрация, версии, загрузки и контроль качества. Все обращения идут через Gateway API.',
  },
  runs: {
    title: 'Запуски',
    description: 'Создание, управление состояниями, ретраи и пакеты воспроизводимости.',
  },
  pipelines: {
    title: 'Пайплайны',
    description: 'DAG‑исполнение, узлы и контролируемые отмены.',
  },
  environments: {
    title: 'Среды исполнения',
    description: 'Шаблоны, блокировки окружений и верификация образов.',
  },
  devenvs: {
    title: 'DevEnv (IDE)',
    description: 'Сессии IDE через прокси, контроль TTL и остановка окружений.',
  },
  models: {
    title: 'Регистр моделей',
    description: 'Жизненный цикл версий, экспорт и provenance.',
  },
  lineage: {
    title: 'Lineage',
    description: 'Графы происхождения запусков и версий моделей.',
  },
  audit: {
    title: 'Аудит / SIEM',
    description: 'События аудита, доставки, попытки и DLQ.',
  },
  ops: {
    title: 'Ops',
    description: 'Операционная готовность, контроль метрик и health‑состояния.',
  },
} as const;

const meta = copy['models' as keyof typeof copy];

type SearchParams = {
  q?: string;
  status?: string;
  model_id?: string;
  page?: string;
};

export default async function ModelsPage({ searchParams }: { searchParams?: Promise<SearchParams> }) {
  const params = (await searchParams) ?? {};
  const projectId = await getActiveProjectId();
  const session = await getGatewaySession();
  const role = deriveEffectiveRole(session.mode === 'authenticated' ? session.roles : []);
  const query = params.q?.toLowerCase().trim() ?? '';
  const statusFilter = params.status?.toLowerCase().trim() ?? '';
  const modelId = params.model_id?.trim() ?? '';
  const pageRaw = Number(params.page ?? '1');
  const page = Number.isFinite(pageRaw) && pageRaw > 0 ? pageRaw : 1;
  let models: components['schemas']['Model'][] = [];
  let versions: components['schemas']['ModelVersion'][] = [];
  let error: GatewayAPIError | null = null;

  if (projectId) {
    try {
      const response = await gatewayServerFetchJSON<components['schemas']['ModelListResponse']>(
        `/projects/${projectId}/models?limit=200`,
      );
      models = response.models ?? [];
      if (modelId) {
        const versionResponse = await gatewayServerFetchJSON<components['schemas']['ModelVersionListResponse']>(
          `/projects/${projectId}/models/${modelId}/versions?limit=200`,
        );
        versions = versionResponse.modelVersions ?? [];
      }
    } catch (err) {
      error = err instanceof GatewayAPIError ? err : new GatewayAPIError(500, 'gateway_unexpected');
    }
  }

  const filtered = models.filter((model) => {
    if (statusFilter && model.status?.toLowerCase() !== statusFilter) {
      return false;
    }
    if (!query) {
      return true;
    }
    return [model.modelId, model.name, model.status].join(' ').toLowerCase().includes(query);
  });

  const sorted = [...filtered].sort((a, b) => {
    const aTime = a.createdAt ? new Date(a.createdAt).getTime() : 0;
    const bTime = b.createdAt ? new Date(b.createdAt).getTime() : 0;
    return bTime - aTime;
  });

  const pageSize = 20;
  const totalPages = Math.max(1, Math.ceil(sorted.length / pageSize));
  const safePage = Math.min(page, totalPages);
  const slice = sorted.slice((safePage - 1) * pageSize, safePage * pageSize);

  return (
    <PageShell>
      <PageHeader
        title={meta.title}
        description={meta.description}
        actions={
          <Link href="/console/models/new-version" className="text-sm font-semibold text-primary">
            Новая версия модели
          </Link>
        }
      />
      <Card>
        <CardHeader>
          <CardTitle>Регистр моделей</CardTitle>
          <CardDescription>Логические модели и версии с полным provenance‑следом.</CardDescription>
        </CardHeader>
        <CardContent>
          <div className="text-xs text-muted-foreground">Контекст проекта: {projectId || 'не задан'}</div>
        </CardContent>
      </Card>

      {error ? <ErrorState code={error.code} requestId={error.requestId} status={error.status} details={error.details} message={error.message} retryable={error.retryable} /> : null}

      <PageSection title="Создание модели">
        <ModelCreateForm projectId={projectId} role={role} />
      </PageSection>

      <PageSection title="Список моделей" description="Поиск по имени и статусу.">
        <ModelFilters />
        <TableContainer>
          <Table>
            <thead>
              <tr>
                <th>ID</th>
                <th>Имя</th>
                <th>Статус</th>
                <th>Версия</th>
                <th>Создано</th>
              </tr>
            </thead>
            <tbody>
              {slice.map((model) => (
                <tr key={model.modelId}>
                  <td className="font-mono text-xs">
                    <div className="flex items-center gap-2">
                      {model.modelId}
                      <CopyButton value={model.modelId} />
                    </div>
                  </td>
                  <td>
                    <div className="text-sm font-semibold">{model.name}</div>
                    <div className="text-xs text-muted-foreground">{model.metadata ? 'metadata задана' : 'без metadata'}</div>
                  </td>
                  <td>
                    <StatusPill status={model.status} />
                  </td>
                  <td className="text-xs">{model.version}</td>
                  <td className="text-xs text-muted-foreground">{formatDateTime(model.createdAt)}</td>
                </tr>
              ))}
            </tbody>
          </Table>
          {slice.length === 0 ? <TableEmpty>Модели не найдены.</TableEmpty> : null}
        </TableContainer>
        <div className="mt-4">
          <Pagination page={safePage} totalPages={totalPages} />
        </div>
      </PageSection>

      <PageSection title="Версии модели" description="Выберите model_id для просмотра списка версий.">
        <ModelSelector />
        {modelId ? (
          <ModelVersionsTable projectId={projectId} modelId={modelId} versions={versions} role={role} />
        ) : (
          <Card>
            <CardHeader>
              <CardTitle>Model ID не выбран</CardTitle>
              <CardDescription>Укажите model_id для работы с версиями и экспортами.</CardDescription>
            </CardHeader>
          </Card>
        )}
      </PageSection>
    </PageShell>
  );
}
