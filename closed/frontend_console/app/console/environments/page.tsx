import Link from 'next/link';

import { CopyButton } from '@/components/console/copy-button';
import { ErrorState } from '@/components/console/error-state';
import { StatusPill } from '@/components/console/status-pill';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Table, TableContainer, TableEmpty } from '@/components/ui/table';
import { PageHeader, PageSection, PageShell } from '@/components/ui/page-shell';
import { GatewayAPIError } from '@/lib/gateway-client';
import type { components } from '@/lib/gateway-openapi';
import { formatDateTime } from '@/lib/format';
import { getActiveProjectId } from '@/lib/server-context';
import { gatewayServerFetchJSON } from '@/lib/server-gateway';

import { EnvironmentFilters } from './environment-filters';

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

const meta = copy['environments' as keyof typeof copy];

type SearchParams = {
  q?: string;
};

export default async function EnvironmentsPage({ searchParams }: { searchParams: SearchParams }) {
  const projectId = getActiveProjectId();
  const query = searchParams.q?.toLowerCase().trim() ?? '';
  let definitions: components['schemas']['EnvironmentDefinition'][] = [];
  let locks: components['schemas']['EnvironmentLock'][] = [];
  let verifications: components['schemas']['ImageVerificationRecord'][] = [];
  let error: GatewayAPIError | null = null;

  if (projectId) {
    try {
      const defs = await gatewayServerFetchJSON<components['schemas']['EnvironmentDefinitionListResponse']>(
        `/api/experiments/projects/${projectId}/environment-definitions?limit=200`,
      );
      definitions = defs.environments ?? [];
      const locksResponse = await gatewayServerFetchJSON<components['schemas']['EnvironmentLockListResponse']>(
        `/api/experiments/projects/${projectId}/environment-locks?limit=200`,
      );
      locks = locksResponse.locks ?? [];
      const verificationResponse = await gatewayServerFetchJSON<components['schemas']['ImageVerificationListResponse']>(
        `/api/experiments/projects/${projectId}/registry/verifications?limit=200`,
      );
      verifications = verificationResponse.items ?? [];
    } catch (err) {
      error = err instanceof GatewayAPIError ? err : new GatewayAPIError(500, 'gateway_unexpected');
    }
  }

  const filteredDefinitions = definitions.filter((env) => {
    if (!query) {
      return true;
    }
    return [env.environmentDefinitionId, env.name, env.status].join(' ').toLowerCase().includes(query);
  });

  const filteredLocks = locks.filter((lock) => {
    if (!query) {
      return true;
    }
    const images = lock.images?.map((image) => `${image.name} ${image.ref} ${image.digest}`).join(' ') ?? '';
    return [lock.lockId, lock.environmentDefinitionId, lock.envHash, images].join(' ').toLowerCase().includes(query);
  });

  const findVerification = (digest: string) =>
    verifications.find((record) => record.imageDigestRef.includes(digest));

  return (
    <PageShell>
      <PageHeader
        title={meta.title}
        description={meta.description}
        actions={
          <Link href="/console/environments/new-lock" className="text-sm font-semibold text-primary">
            Новый EnvLock
          </Link>
        }
      />
      <Card>
        <CardHeader>
          <CardTitle>Контроль среды исполнения</CardTitle>
          <CardDescription>Шаблоны и блокировки являются неизменяемыми. Верификация образов проводится до создания EnvLock.</CardDescription>
        </CardHeader>
        <CardContent>
          <div className="flex flex-wrap items-center gap-3 text-xs text-muted-foreground">
            <div>Контекст проекта: {projectId || 'не задан'}</div>
            <div>Definitions: {filteredDefinitions.length}</div>
            <div>Locks: {filteredLocks.length}</div>
          </div>
        </CardContent>
      </Card>

      <PageSection title="Фильтрация">
        <EnvironmentFilters />
      </PageSection>

      {error ? <ErrorState code={error.code} requestId={error.requestId} status={error.status} details={error.details} /> : null}

      <PageSection title="Environment Definitions" description="Версии шаблонов окружений.">
        <TableContainer>
          <Table>
            <thead>
              <tr>
                <th>ID</th>
                <th>Название</th>
                <th>Версия</th>
                <th>Статус</th>
                <th>Создано</th>
                <th>Образы</th>
              </tr>
            </thead>
            <tbody>
              {filteredDefinitions.map((env) => (
                <tr key={env.environmentDefinitionId}>
                  <td className="font-mono text-xs">
                    <div className="flex items-center gap-2">
                      {env.environmentDefinitionId}
                      <CopyButton value={env.environmentDefinitionId} />
                    </div>
                  </td>
                  <td>
                    <div className="text-sm font-semibold">{env.name}</div>
                    {env.description ? <div className="text-xs text-muted-foreground">{env.description}</div> : null}
                  </td>
                  <td className="text-xs">{env.version}</td>
                  <td>
                    <StatusPill status={env.status} />
                  </td>
                  <td className="text-xs text-muted-foreground">{formatDateTime(env.createdAt)}</td>
                  <td className="text-xs text-muted-foreground">
                    {env.baseImages?.map((image) => (
                      <div key={`${env.environmentDefinitionId}-${image.name}`}>{image.name}: {image.ref}</div>
                    )) ?? '—'}
                  </td>
                </tr>
              ))}
            </tbody>
          </Table>
          {filteredDefinitions.length === 0 ? <TableEmpty>Определения не найдены.</TableEmpty> : null}
        </TableContainer>
      </PageSection>

      <PageSection title="Environment Locks" description="Блокировки окружений с зафиксированными digest и проверками.">
        <TableContainer>
          <Table>
            <thead>
              <tr>
                <th>Lock ID</th>
                <th>Definition</th>
                <th>Env Hash</th>
                <th>Образы</th>
                <th>Registry verify</th>
              </tr>
            </thead>
            <tbody>
              {filteredLocks.map((lock) => (
                <tr key={lock.lockId}>
                  <td className="font-mono text-xs">
                    <div className="flex items-center gap-2">
                      {lock.lockId}
                      <CopyButton value={lock.lockId} />
                    </div>
                  </td>
                  <td className="text-xs">{lock.environmentDefinitionId}</td>
                  <td className="font-mono text-xs">{lock.envHash}</td>
                  <td className="text-xs text-muted-foreground">
                    {lock.images?.map((image) => (
                      <div key={`${lock.lockId}-${image.name}`} className="mb-1">
                        <div>{image.name}</div>
                        <div className="font-mono">{image.ref}</div>
                        <div className="font-mono">{image.digest}</div>
                      </div>
                    )) ?? '—'}
                  </td>
                  <td className="text-xs">
                    {lock.images?.map((image) => {
                      const verification = findVerification(image.digest);
                      if (!verification) {
                        return (
                          <div key={`${lock.lockId}-${image.digest}`} className="text-muted-foreground">
                            нет данных
                          </div>
                        );
                      }
                      return (
                        <div key={`${lock.lockId}-${image.digest}`} className="mb-2">
                          <StatusPill status={verification.status} />
                          <div className="text-xs text-muted-foreground">
                            policy: {verification.policyMode} · provider: {verification.provider}
                          </div>
                        </div>
                      );
                    }) ?? '—'}
                  </td>
                </tr>
              ))}
            </tbody>
          </Table>
          {filteredLocks.length === 0 ? <TableEmpty>Блокировки не найдены.</TableEmpty> : null}
        </TableContainer>
      </PageSection>
    </PageShell>
  );
}
