import Link from 'next/link';

import { ErrorState } from '@/components/console/error-state';
import { PipelineGraph } from '@/components/console/pipeline-graph';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { PageHeader, PageSection, PageShell } from '@/components/ui/page-shell';
import { GatewayAPIError } from '@/lib/gateway-client';
import type { components } from '@/lib/gateway-openapi';
import { getActiveProjectId } from '@/lib/server-context';
import { gatewayServerFetchJSON } from '@/lib/server-gateway';
import { stringifySafe } from '@/lib/sanitize';

import { PipelineSelector } from './pipeline-selector';

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

const meta = copy['pipelines' as keyof typeof copy];

type SearchParams = {
  run_id?: string;
};

export default async function PipelinesPage({ searchParams }: { searchParams: SearchParams }) {
  const projectId = getActiveProjectId();
  const runId = searchParams.run_id?.trim() ?? '';
  let runSpec: components['schemas']['ProjectRunGetResponse'] | null = null;
  let error: GatewayAPIError | null = null;

  if (projectId && runId) {
    try {
      runSpec = await gatewayServerFetchJSON<components['schemas']['ProjectRunGetResponse']>(
        `/api/experiments/projects/${projectId}/runs/${runId}`,
      );
    } catch (err) {
      error = err instanceof GatewayAPIError ? err : new GatewayAPIError(500, 'gateway_unexpected');
    }
  }

  return (
    <PageShell>
      <PageHeader title={meta.title} description={meta.description} />
      <Card>
        <CardHeader>
          <CardTitle>DAG‑контроль</CardTitle>
          <CardDescription>Граф пайплайна строится на основе RunSpec. Для просмотра укажите run_id.</CardDescription>
        </CardHeader>
        <CardContent>
          <p className="text-sm text-muted-foreground">
            Пайплайны являются детерминированными графами исполнения. Отмена доступна через DP‑контур, консоль отображает
            текущее состояние.
          </p>
        </CardContent>
      </Card>

      <PageSection title="Выбор источника">
        <PipelineSelector />
        <div className="text-xs text-muted-foreground">
          Контекст проекта: {projectId || 'не задан'} · Run ID: {runId || 'не указан'}
        </div>
      </PageSection>

      {error ? <ErrorState code={error.code} requestId={error.requestId} status={error.status} details={error.details} message={error.message} retryable={error.retryable} /> : null}

      {runSpec ? (
        <PageSection title="DAG и шаги">
          <PipelineGraph pipelineSpec={runSpec.runSpec.pipelineSpec ?? {}} attemptsByStep={runSpec.attemptsByStep ?? {}} />
          <Card>
            <CardHeader>
              <CardTitle>Сырые данные pipelineSpec</CardTitle>
              <CardDescription>Полное описание графа, зафиксированное в RunSpec.</CardDescription>
            </CardHeader>
            <CardContent>
              <pre className="text-xs whitespace-pre-wrap">{stringifySafe(runSpec.runSpec.pipelineSpec ?? {})}</pre>
            </CardContent>
          </Card>
        </PageSection>
      ) : (
        <Card>
          <CardHeader>
            <CardTitle>Граф не загружен</CardTitle>
            <CardDescription>
              Укажите run_id и убедитесь, что ваш доступ включает RunRead. Создание новых RunSpec доступно в разделе запусков.
            </CardDescription>
          </CardHeader>
          <CardContent>
            <Link href="/console/runs/new" className="text-sm font-semibold text-primary">
              Перейти к созданию RunSpec
            </Link>
          </CardContent>
        </Card>
      )}
    </PageShell>
  );
}
