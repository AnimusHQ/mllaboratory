import { ErrorState } from '@/components/console/error-state';
import { LineageGraph } from '@/components/console/lineage-graph';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { PageHeader, PageSection, PageShell } from '@/components/ui/page-shell';
import { GatewayAPIError } from '@/lib/gateway-client';
import type { components } from '@/lib/gateway-openapi';
import { gatewayServerFetchJSON } from '@/lib/server-gateway';

import { LineageFilters } from './lineage-filters';

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

const meta = copy['lineage' as keyof typeof copy];

type SearchParams = {
  run_id?: string;
  model_version_id?: string;
};

export default async function LineagePage({ searchParams }: { searchParams: SearchParams }) {
  const runId = searchParams.run_id?.trim() ?? '';
  const modelVersionId = searchParams.model_version_id?.trim() ?? '';
  let graph: components['schemas']['SubgraphResponse'] | null = null;
  let error: GatewayAPIError | null = null;

  if (runId || modelVersionId) {
    try {
      const params = new URLSearchParams({ depth: '3', max_edges: '200' });
      if (runId) {
        graph = await gatewayServerFetchJSON<components['schemas']['SubgraphResponse']>(
          `/api/lineage/runs/${runId}?${params.toString()}`,
        );
      } else if (modelVersionId) {
        graph = await gatewayServerFetchJSON<components['schemas']['SubgraphResponse']>(
          `/api/lineage/model-versions/${modelVersionId}?${params.toString()}`,
        );
      }
    } catch (err) {
      error = err instanceof GatewayAPIError ? err : new GatewayAPIError(500, 'gateway_unexpected');
    }
  }

  return (
    <PageShell>
      <PageHeader title={meta.title} description={meta.description} />
      <Card>
        <CardHeader>
          <CardTitle>Lineage‑граф</CardTitle>
          <CardDescription>Материализованные связи входов и выходов. Графы агрегируются через Gateway.</CardDescription>
        </CardHeader>
        <CardContent>
          <p className="text-sm text-muted-foreground">Используйте run_id или model_version_id для загрузки подграфа.</p>
        </CardContent>
      </Card>

      <PageSection title="Запрос lineage">
        <LineageFilters />
      </PageSection>

      {error ? <ErrorState code={error.code} requestId={error.requestId} status={error.status} details={error.details} message={error.message} retryable={error.retryable} /> : null}

      {graph ? (
        <PageSection title="Граф">
          <LineageGraph nodes={graph.nodes ?? []} edges={graph.edges ?? []} />
        </PageSection>
      ) : (
        <Card>
          <CardHeader>
            <CardTitle>Граф не загружен</CardTitle>
            <CardDescription>Укажите идентификатор run_id или model_version_id.</CardDescription>
          </CardHeader>
        </Card>
      )}
    </PageShell>
  );
}
