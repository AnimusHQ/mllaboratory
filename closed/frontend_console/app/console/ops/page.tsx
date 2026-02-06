import { ErrorState } from '@/components/console/error-state';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { PageHeader, PageSection, PageShell } from '@/components/ui/page-shell';
import { GatewayAPIError } from '@/lib/gateway-client';
import type { components } from '@/lib/gateway-openapi';
import { gatewayServerFetchJSON } from '@/lib/server-gateway';

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

const meta = copy['ops' as keyof typeof copy];

export default async function OpsPage() {
  let health: components['schemas']['HealthResponse'] | null = null;
  let ready: components['schemas']['HealthResponse'] | null = null;
  let error: GatewayAPIError | null = null;

  try {
    health = await gatewayServerFetchJSON<components['schemas']['HealthResponse']>('/healthz');
    ready = await gatewayServerFetchJSON<components['schemas']['HealthResponse']>('/readyz');
  } catch (err) {
    error = err instanceof GatewayAPIError ? err : new GatewayAPIError(500, 'gateway_unexpected');
  }

  return (
    <PageShell>
      <PageHeader title={meta.title} description={meta.description} />
      <Card>
        <CardHeader>
          <CardTitle>Операционная готовность</CardTitle>
          <CardDescription>Контроль живости, готовности и ссылок на метрики.</CardDescription>
        </CardHeader>
        <CardContent>
          <p className="text-sm text-muted-foreground">
            Контур предназначен для дежурных проверок и навигации к инструментам SRE. Все данные извлекаются через Gateway.
          </p>
        </CardContent>
      </Card>

      {error ? <ErrorState code={error.code} requestId={error.requestId} status={error.status} details={error.details} message={error.message} retryable={error.retryable} /> : null}

      <PageSection title="Health probes">
        <Card>
          <CardHeader>
            <CardTitle>Gateway probes</CardTitle>
            <CardDescription>Результаты проверок /healthz и /readyz.</CardDescription>
          </CardHeader>
          <CardContent className="grid gap-3 text-sm md:grid-cols-2">
            <div>
              <div className="text-xs text-muted-foreground">Healthz</div>
              <div>{health ? `${health.service}: ${health.status}` : '—'}</div>
            </div>
            <div>
              <div className="text-xs text-muted-foreground">Readyz</div>
              <div>{ready ? `${ready.service}: ${ready.status}` : '—'}</div>
            </div>
          </CardContent>
        </Card>
      </PageSection>

      <PageSection title="Метрики и диагностика">
        <Card>
          <CardHeader>
            <CardTitle>Ссылки</CardTitle>
            <CardDescription>Метрики и логи доступны через внутренние маршруты.</CardDescription>
          </CardHeader>
          <CardContent className="text-sm text-muted-foreground">
            <ul className="list-disc pl-5">
              <li>Прометей метрики: /metrics (доступ зависит от сети).</li>
              <li>Логи аудита и экспорта доступны через SIEM коннекторы.</li>
              <li>Информация о версии предоставляется через release‑документацию.</li>
            </ul>
          </CardContent>
        </Card>
      </PageSection>
    </PageShell>
  );
}
