import Link from 'next/link';

import { DevEnvTable } from '@/components/console/devenv-table';
import { ErrorState } from '@/components/console/error-state';
import { Pagination } from '@/components/console/pagination';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { PageHeader, PageSection, PageShell } from '@/components/ui/page-shell';
import { GatewayAPIError } from '@/lib/gateway-client';
import type { components } from '@/lib/gateway-openapi';
import { getActiveProjectId } from '@/lib/server-context';
import { getGatewaySession } from '@/lib/session';
import { gatewayServerFetchJSON } from '@/lib/server-gateway';
import { deriveEffectiveRole } from '@/lib/rbac';

import { DevEnvFilters } from './devenv-filters';
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

const meta = copy['devenvs' as keyof typeof copy];

type SearchParams = {
  q?: string;
  state?: string;
  page?: string;
};

export default async function DevEnvsPage({ searchParams }: { searchParams: SearchParams }) {
  const projectId = getActiveProjectId();
  const session = await getGatewaySession();
  const role = deriveEffectiveRole(session.mode === 'authenticated' ? session.roles : []);
  let environments: components['schemas']['DevEnvironment'][] = [];
  let error: GatewayAPIError | null = null;
  const query = searchParams.q?.toLowerCase().trim() ?? '';
  const stateFilter = searchParams.state?.toLowerCase().trim() ?? '';
  const pageRaw = Number(searchParams.page ?? '1');
  const page = Number.isFinite(pageRaw) && pageRaw > 0 ? pageRaw : 1;

  if (projectId) {
    try {
      const response = await gatewayServerFetchJSON<components['schemas']['DevEnvironmentListResponse']>(
        `/api/experiments/projects/${projectId}/devenvs?limit=200`,
      );
      environments = response.environments ?? [];
    } catch (err) {
      error = err instanceof GatewayAPIError ? err : new GatewayAPIError(500, 'gateway_unexpected');
    }
  }

  const filtered = environments.filter((env) => {
    if (stateFilter && env.state?.toLowerCase() !== stateFilter) {
      return false;
    }
    if (!query) {
      return true;
    }
    return [env.devEnvId, env.repoUrl, env.templateRef, env.refValue].join(' ').toLowerCase().includes(query);
  });

  const pageSize = 20;
  const totalPages = Math.max(1, Math.ceil(filtered.length / pageSize));
  const safePage = Math.min(page, totalPages);
  const slice = filtered.slice((safePage - 1) * pageSize, safePage * pageSize);

  return (
    <PageShell>
      <PageHeader
        title={meta.title}
        description={meta.description}
        actions={
          <Link href="/console/devenvs/new" className="text-sm font-semibold text-primary">
            Новый DevEnv
          </Link>
        }
      />
      <Card>
        <CardHeader>
          <CardTitle>Сессии IDE</CardTitle>
          <CardDescription>Доступ осуществляется только через CP‑прокси. Внутренние сервисы не доступны напрямую.</CardDescription>
        </CardHeader>
        <CardContent>
          <p className="text-sm text-muted-foreground">
            Для открытия VS Code используется прокси‑маршрут DevEnv. Все открытия и остановки фиксируются в аудите.
          </p>
        </CardContent>
      </Card>

      {!projectId ? (
        <Card>
          <CardHeader>
            <CardTitle>Контекст проекта не задан</CardTitle>
            <CardDescription>Укажите project_id в верхней панели для управления DevEnv.</CardDescription>
          </CardHeader>
        </Card>
      ) : null}

      {error ? <ErrorState code={error.code} requestId={error.requestId} status={error.status} details={error.details} /> : null}

      <PageSection title="Фильтры и поиск">
        <DevEnvFilters />
      </PageSection>

      <PageSection title="DevEnvs">
        <DevEnvTable environments={slice} projectId={projectId} role={role} />
        <div className="mt-4">
          <Pagination page={safePage} totalPages={totalPages} />
        </div>
      </PageSection>
    </PageShell>
  );
}
