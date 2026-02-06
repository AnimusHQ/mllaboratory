import Link from 'next/link';

import { CopyButton } from '@/components/console/copy-button';
import { ErrorState } from '@/components/console/error-state';
import { PolicyHint } from '@/components/console/policy-hint';
import { Pagination } from '@/components/console/pagination';
import { StatusPill } from '@/components/console/status-pill';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Table, TableContainer, TableEmpty } from '@/components/ui/table';
import { PageHeader, PageSection, PageShell } from '@/components/ui/page-shell';
import { GatewayAPIError } from '@/lib/gateway-client';
import type { components } from '@/lib/gateway-openapi';
import { formatDateTime } from '@/lib/format';
import { can, deriveEffectiveRole } from '@/lib/rbac';
import { getActiveProjectId } from '@/lib/server-context';
import { getGatewaySession } from '@/lib/session';
import { gatewayServerFetchJSON } from '@/lib/server-gateway';

import { RunFilters } from './run-filters';

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

const meta = copy['runs' as keyof typeof copy];

type SearchParams = {
  status?: string;
  q?: string;
  page?: string;
};

export default async function RunsPage({ searchParams }: { searchParams?: Promise<SearchParams> }) {
  const params = (await searchParams) ?? {};
  const session = await getGatewaySession();
  const role = deriveEffectiveRole(session.mode === 'authenticated' ? session.roles : []);
  const canWrite = can(role, 'run:write');
  let runs: components['schemas']['ExperimentRun'][] = [];
  let error: GatewayAPIError | null = null;

  const statusFilter = params.status?.trim();
  const query = params.q?.trim().toLowerCase() ?? '';
  const pageRaw = Number(params.page ?? '1');
  const page = Number.isFinite(pageRaw) && pageRaw > 0 ? pageRaw : 1;

  try {
    const params = new URLSearchParams({ limit: '200' });
    if (statusFilter) {
      params.set('status', statusFilter);
    }
    const response = await gatewayServerFetchJSON<components['schemas']['ExperimentRunListResponse']>(
      `/api/experiments/experiment-runs?${params.toString()}`,
    );
    runs = response.runs ?? [];
  } catch (err) {
    error = err instanceof GatewayAPIError ? err : new GatewayAPIError(500, 'gateway_unexpected');
  }

  const filtered = runs.filter((run) => {
    if (!query) {
      return true;
    }
    const haystack = [
      run.run_id,
      run.experiment_id,
      run.dataset_version_id,
      run.git_repo,
      run.git_commit,
      run.git_ref,
      run.status,
    ]
      .filter(Boolean)
      .join(' ')
      .toLowerCase();
    return haystack.includes(query);
  });

  const sorted = [...filtered].sort((a, b) => {
    const aTime = a.started_at ? new Date(a.started_at).getTime() : 0;
    const bTime = b.started_at ? new Date(b.started_at).getTime() : 0;
    if (aTime !== bTime) {
      return bTime - aTime;
    }
    return a.run_id.localeCompare(b.run_id);
  });

  const pageSize = 20;
  const totalPages = Math.max(1, Math.ceil(sorted.length / pageSize));
  const safePage = Math.min(page, totalPages);
  const slice = sorted.slice((safePage - 1) * pageSize, safePage * pageSize);
  const projectId = await getActiveProjectId();

  return (
    <PageShell>
      <PageHeader
        title={meta.title}
        description={meta.description}
        actions={
          <div className="flex flex-col items-end gap-1">
            {canWrite ? (
              <Button asChild variant="secondary" size="sm">
                <Link href="/console/runs/new">Новый RunSpec</Link>
              </Button>
            ) : (
              <Button variant="secondary" size="sm" disabled>
                Новый RunSpec
              </Button>
            )}
            <PolicyHint allowed={canWrite} capability="run:write" />
          </div>
        }
      />
      <Card>
        <CardHeader>
          <CardTitle>Сводка очереди</CardTitle>
          <CardDescription>Список запусков собирается через Gateway. Создание RunSpec требует активного project_id.</CardDescription>
        </CardHeader>
        <CardContent>
          <div className="flex flex-wrap items-center gap-3 text-xs text-muted-foreground">
            <div>Всего: {filtered.length}</div>
            <div>Страница: {safePage}</div>
            <div>Контекст проекта: {projectId || 'не задан'}</div>
          </div>
        </CardContent>
      </Card>

      <PageSection title="Фильтры и поиск">
        <RunFilters />
      </PageSection>

      {error ? <ErrorState code={error.code} requestId={error.requestId} status={error.status} details={error.details} message={error.message} retryable={error.retryable} /> : null}

      <PageSection title="Реестр запусков" description="Неизменяемые записи Run. Таблица детерминированно отсортирована.">
        <TableContainer>
          <Table>
            <thead>
              <tr>
                <th>Run ID</th>
                <th>Experiment ID</th>
                <th>Статус</th>
                <th>Dataset Version</th>
                <th>Git</th>
                <th>Время</th>
                <th>Действия</th>
              </tr>
            </thead>
            <tbody>
              {slice.map((run) => (
                <tr key={run.run_id}>
                  <td className="font-mono text-xs">
                    <div className="flex items-center gap-2">
                      {run.run_id}
                      <CopyButton value={run.run_id} />
                    </div>
                  </td>
                  <td className="font-mono text-xs">
                    <div className="flex items-center gap-2">
                      {run.experiment_id}
                      <CopyButton value={run.experiment_id} />
                    </div>
                  </td>
                  <td>
                    <StatusPill status={run.status} />
                  </td>
                  <td className="font-mono text-xs">
                    {run.dataset_version_id ? (
                      <div className="flex items-center gap-2">
                        {run.dataset_version_id}
                        <CopyButton value={run.dataset_version_id} />
                      </div>
                    ) : (
                      '—'
                    )}
                  </td>
                  <td className="text-xs text-muted-foreground">
                    {run.git_repo ? <div>{run.git_repo}</div> : <div>—</div>}
                    {run.git_commit ? <div className="font-mono">{run.git_commit}</div> : null}
                    {run.git_ref ? <div className="font-mono">{run.git_ref}</div> : null}
                  </td>
                  <td className="text-xs text-muted-foreground">
                    <div>Старт: {formatDateTime(run.started_at)}</div>
                    <div>Финиш: {formatDateTime(run.ended_at)}</div>
                  </td>
                  <td>
                    <Button asChild variant="secondary" size="sm">
                      <Link href={`/console/runs/${run.run_id}`}>Открыть</Link>
                    </Button>
                  </td>
                </tr>
              ))}
            </tbody>
          </Table>
          {slice.length === 0 ? <TableEmpty>Запуски не найдены.</TableEmpty> : null}
        </TableContainer>
        <div className="mt-4">
          <Pagination page={safePage} totalPages={totalPages} />
        </div>
      </PageSection>
    </PageShell>
  );
}
