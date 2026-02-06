import { AuditDeliveryTable } from '@/components/console/audit-delivery-table';
import { ErrorState } from '@/components/console/error-state';
import { Pagination } from '@/components/console/pagination';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Table, TableContainer, TableEmpty } from '@/components/ui/table';
import { PageHeader, PageSection, PageShell } from '@/components/ui/page-shell';
import { GatewayAPIError } from '@/lib/gateway-client';
import type { components } from '@/lib/gateway-openapi';
import { formatDateTime } from '@/lib/format';
import { gatewayServerFetchJSON } from '@/lib/server-gateway';
import { getGatewaySession } from '@/lib/session';
import { deriveEffectiveRole } from '@/lib/rbac';

import { AuditFilters } from './audit-filters';
import { DeliveryFilters } from './delivery-filters';

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

const meta = copy['audit' as keyof typeof copy];

type SearchParams = {
  q?: string;
  delivery_status?: string;
  delivery_id?: string;
  events_page?: string;
  deliveries_page?: string;
  attempts_page?: string;
};

export default async function AuditPage({ searchParams }: { searchParams: SearchParams }) {
  const session = await getGatewaySession();
  const role = deriveEffectiveRole(session.mode === 'authenticated' ? session.roles : []);
  const query = searchParams.q?.toLowerCase().trim() ?? '';
  const deliveryStatus = searchParams.delivery_status?.trim() ?? '';
  const deliveryId = searchParams.delivery_id?.trim() ?? '';
  const eventsPage = Number(searchParams.events_page ?? '1') || 1;
  const deliveriesPage = Number(searchParams.deliveries_page ?? '1') || 1;
  const attemptsPage = Number(searchParams.attempts_page ?? '1') || 1;
  let events: components['schemas']['AuditEvent'][] = [];
  let deliveries: components['schemas']['ExportDelivery'][] = [];
  let attempts: components['schemas']['ExportDeliveryAttempt'][] = [];
  let error: GatewayAPIError | null = null;

  try {
    const eventResponse = await gatewayServerFetchJSON<components['schemas']['AuditEventListResponse']>(
      `/api/audit/events?limit=200`,
    );
    events = eventResponse.events ?? [];
    const params = new URLSearchParams({ limit: '200' });
    if (deliveryStatus) {
      params.set('status', deliveryStatus);
    }
    const deliveryResponse = await gatewayServerFetchJSON<components['schemas']['ExportDeliveryListResponse']>(
      `/api/audit/admin/audit/exports/deliveries?${params.toString()}`,
    );
    deliveries = deliveryResponse.deliveries ?? [];
    if (deliveryId) {
      const attemptsResponse = await gatewayServerFetchJSON<components['schemas']['ExportDeliveryAttemptListResponse']>(
        `/api/audit/admin/audit/exports/deliveries/${deliveryId}/attempts?limit=200`,
      );
      attempts = attemptsResponse.attempts ?? [];
    }
  } catch (err) {
    error = err instanceof GatewayAPIError ? err : new GatewayAPIError(500, 'gateway_unexpected');
  }

  const filteredEvents = events.filter((event) => {
    if (!query) {
      return true;
    }
    return [event.actor, event.action, event.resource_type, event.resource_id]
      .filter(Boolean)
      .join(' ')
      .toLowerCase()
      .includes(query);
  });

  const sortedEvents = [...filteredEvents].sort((a, b) => {
    const aTime = a.occurred_at ? new Date(a.occurred_at).getTime() : 0;
    const bTime = b.occurred_at ? new Date(b.occurred_at).getTime() : 0;
    if (aTime !== bTime) {
      return bTime - aTime;
    }
    return a.event_id - b.event_id;
  });
  const sortedDeliveries = [...deliveries].sort((a, b) => {
    const aTime = a.created_at ? new Date(a.created_at).getTime() : 0;
    const bTime = b.created_at ? new Date(b.created_at).getTime() : 0;
    return bTime - aTime;
  });
  const sortedAttempts = [...attempts].sort((a, b) => {
    const aTime = a.attempted_at ? new Date(a.attempted_at).getTime() : 0;
    const bTime = b.attempted_at ? new Date(b.attempted_at).getTime() : 0;
    return bTime - aTime;
  });

  const eventPageSize = 20;
  const deliveryPageSize = 20;
  const attemptPageSize = 20;
  const eventTotalPages = Math.max(1, Math.ceil(sortedEvents.length / eventPageSize));
  const deliveryTotalPages = Math.max(1, Math.ceil(sortedDeliveries.length / deliveryPageSize));
  const attemptTotalPages = Math.max(1, Math.ceil(sortedAttempts.length / attemptPageSize));
  const safeEventsPage = Math.min(eventsPage, eventTotalPages);
  const safeDeliveriesPage = Math.min(deliveriesPage, deliveryTotalPages);
  const safeAttemptsPage = Math.min(attemptsPage, attemptTotalPages);
  const eventsSlice = sortedEvents.slice((safeEventsPage - 1) * eventPageSize, safeEventsPage * eventPageSize);
  const deliveriesSlice = sortedDeliveries.slice((safeDeliveriesPage - 1) * deliveryPageSize, safeDeliveriesPage * deliveryPageSize);
  const attemptsSlice = sortedAttempts.slice((safeAttemptsPage - 1) * attemptPageSize, safeAttemptsPage * attemptPageSize);

  return (
    <PageShell>
      <PageHeader title={meta.title} description={meta.description} />
      <Card>
        <CardHeader>
          <CardTitle>Аудит и SIEM экспорт</CardTitle>
          <CardDescription>События и доставки формируются в append‑only журнале.</CardDescription>
        </CardHeader>
        <CardContent>
          <p className="text-sm text-muted-foreground">Доступ к SIEM‑операциям требует административных ролей.</p>
        </CardContent>
      </Card>

      {error ? <ErrorState code={error.code} requestId={error.requestId} status={error.status} details={error.details} message={error.message} retryable={error.retryable} /> : null}

      <PageSection title="Аудит‑события">
        <AuditFilters />
        <TableContainer>
          <Table>
            <thead>
              <tr>
                <th>ID</th>
                <th>Время</th>
                <th>Actor</th>
                <th>Action</th>
                <th>Resource</th>
              </tr>
            </thead>
            <tbody>
              {eventsSlice.map((event) => (
                <tr key={event.event_id}>
                  <td className="font-mono text-xs">{event.event_id}</td>
                  <td className="text-xs text-muted-foreground">{formatDateTime(event.occurred_at)}</td>
                  <td className="text-xs">{event.actor}</td>
                  <td className="text-xs">{event.action}</td>
                  <td className="text-xs text-muted-foreground">
                    {event.resource_type}:{event.resource_id}
                  </td>
                </tr>
              ))}
            </tbody>
          </Table>
          {eventsSlice.length === 0 ? <TableEmpty>События не найдены.</TableEmpty> : null}
        </TableContainer>
        <Pagination page={safeEventsPage} totalPages={eventTotalPages} param="events_page" />
      </PageSection>

      <PageSection title="SIEM доставки">
        <DeliveryFilters />
        <AuditDeliveryTable deliveries={deliveriesSlice} role={role} />
        <Pagination page={safeDeliveriesPage} totalPages={deliveryTotalPages} param="deliveries_page" />
      </PageSection>

      <PageSection title="Попытки доставки" description="Укажите delivery_id через query ?delivery_id=...">
        <TableContainer>
          <Table>
            <thead>
              <tr>
                <th>Attempt ID</th>
                <th>Delivery</th>
                <th>Outcome</th>
                <th>Status code</th>
                <th>Latency</th>
                <th>Время</th>
              </tr>
            </thead>
            <tbody>
              {attemptsSlice.map((attempt) => (
                <tr key={attempt.attempt_id}>
                  <td className="font-mono text-xs">{attempt.attempt_id}</td>
                  <td className="text-xs">{attempt.delivery_id}</td>
                  <td className="text-xs">{attempt.outcome}</td>
                  <td className="text-xs">{attempt.status_code ?? '—'}</td>
                  <td className="text-xs">{attempt.latency_ms ?? '—'}</td>
                  <td className="text-xs text-muted-foreground">{formatDateTime(attempt.attempted_at)}</td>
                </tr>
              ))}
            </tbody>
          </Table>
          {attemptsSlice.length === 0 ? <TableEmpty>Попытки не найдены.</TableEmpty> : null}
        </TableContainer>
        <Pagination page={safeAttemptsPage} totalPages={attemptTotalPages} param="attempts_page" />
      </PageSection>
    </PageShell>
  );
}
