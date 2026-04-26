import Link from 'next/link';

import { CopyButton } from '@/components/console/copy-button';
import { ErrorState } from '@/components/console/error-state';
import { StatusPill } from '@/components/console/status-pill';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { PageHeader, PageSection, PageShell } from '@/components/ui/page-shell';
import { Table, TableContainer, TableEmpty } from '@/components/ui/table';
import { GatewayAPIError } from '@/lib/gateway-client';
import type { components } from '@/lib/gateway-openapi';
import { formatDateTime } from '@/lib/format';
import { stringifySafe } from '@/lib/sanitize';
import { getActiveProjectId } from '@/lib/server-context';
import { getGatewaySession } from '@/lib/session';
import { deriveEffectiveRole } from '@/lib/rbac';
import { gatewayServerFetchJSON } from '@/lib/server-gateway';

import { RunActions } from './run-actions';

type Params = {
  run_id: string;
};

export default async function RunDetailPage({ params }: { params?: Promise<Params> }) {
  const routeParams = (await params) ?? { run_id: '' };
  const projectId = await getActiveProjectId();
  const runId = routeParams.run_id;
  const session = await getGatewaySession();
  const role = deriveEffectiveRole(session.mode === 'authenticated' ? session.roles : []);
  let runSpec: components['schemas']['ProjectRunGetResponse'] | null = null;
  let events: components['schemas']['ExperimentRunEventListResponse'] | null = null;
  let error: GatewayAPIError | null = null;

  if (projectId) {
    try {
      runSpec = await gatewayServerFetchJSON<components['schemas']['ProjectRunGetResponse']>(
        `/api/experiments/projects/${projectId}/runs/${runId}`,
      );
    } catch (err) {
      error = err instanceof GatewayAPIError ? err : new GatewayAPIError(500, 'gateway_unexpected');
    }

    if (!error) {
      try {
        events = await gatewayServerFetchJSON<components['schemas']['ExperimentRunEventListResponse']>(
          `/api/experiments/experiment-runs/${runId}/events?limit=50`,
        );
      } catch (err) {
        if (err instanceof GatewayAPIError) {
          error = err;
        }
      }
    }
  }

  return (
    <PageShell>
      <PageHeader
        title={`Run ${runId}`}
        description="Неизменяемая спецификация запуска. Все действия контролируются RBAC и аудируются."
        actions={
          <div className="flex flex-wrap gap-2">
            <Link href="/console/runs/new" className="text-sm font-semibold text-primary">
              Создать новый RunSpec
            </Link>
            <CopyButton value={runId} />
          </div>
        }
      />

      {!projectId ? (
        <Card>
          <CardHeader>
            <CardTitle>Контекст проекта не задан</CardTitle>
            <CardDescription>Для операций планирования и dispatch требуется project_id.</CardDescription>
          </CardHeader>
        </Card>
      ) : null}

      {error ? <ErrorState code={error.code} requestId={error.requestId} status={error.status} details={error.details} message={error.message} retryable={error.retryable} /> : null}

      {runSpec ? (
        <>
          <Card>
            <CardHeader>
              <CardTitle>Сводка RunSpec</CardTitle>
              <CardDescription>Статус и хэш спецификации. Сущность НЕИЗМЕНЯЕМО.</CardDescription>
            </CardHeader>
            <CardContent className="grid gap-3 text-sm md:grid-cols-2">
              <div>
                <div className="text-xs text-muted-foreground">Статус</div>
                <StatusPill status={runSpec.status} />
              </div>
              <div>
                <div className="text-xs text-muted-foreground">Состояние</div>
                <div>{runSpec.state}</div>
              </div>
              <div>
                <div className="text-xs text-muted-foreground">Spec Hash</div>
                <div className="flex items-center gap-2 font-mono text-xs">
                  {runSpec.specHash}
                  <CopyButton value={runSpec.specHash} />
                </div>
              </div>
              <div>
                <div className="text-xs text-muted-foreground">Создано</div>
                <div>{formatDateTime(runSpec.createdAt)}</div>
              </div>
              <div>
                <div className="text-xs text-muted-foreground">План существует</div>
                <div>{runSpec.planExists ? 'да' : 'нет'}</div>
              </div>
              <div>
                <div className="text-xs text-muted-foreground">Policy snapshot hash</div>
                <div className="font-mono text-xs">
                  {runSpec.runSpec.policySnapshot?.snapshotSha256 ? (
                    <div className="flex items-center gap-2">
                      {runSpec.runSpec.policySnapshot.snapshotSha256}
                      <CopyButton value={runSpec.runSpec.policySnapshot.snapshotSha256} />
                    </div>
                  ) : (
                    '—'
                  )}
                </div>
              </div>
            </CardContent>
          </Card>

          <RunActions projectId={projectId} runId={runId} role={role} />

          <PageSection title="Inputs" description="Исходные данные и параметры неизменяемого запуска.">
            <Card>
              <CardHeader>
                <CardTitle>CodeRef и EnvLock</CardTitle>
                <CardDescription>Репозиторий, коммит и блокировка окружения фиксируются при создании.</CardDescription>
              </CardHeader>
              <CardContent className="grid gap-4 text-sm md:grid-cols-2">
                <div>
                  <div className="text-xs text-muted-foreground">Repo URL</div>
                  <div className="font-mono text-xs">{runSpec.runSpec.codeRef.repoUrl}</div>
                </div>
                <div>
                  <div className="text-xs text-muted-foreground">Commit</div>
                  <div className="flex items-center gap-2 font-mono text-xs">
                    {runSpec.runSpec.codeRef.commitSha}
                    <CopyButton value={runSpec.runSpec.codeRef.commitSha} />
                  </div>
                </div>
                <div>
                  <div className="text-xs text-muted-foreground">EnvLock ID</div>
                  <div className="font-mono text-xs">{runSpec.runSpec.envLock.lockId}</div>
                </div>
                <div>
                  <div className="text-xs text-muted-foreground">Env Hash</div>
                  <div className="font-mono text-xs">{runSpec.runSpec.envLock.envHash}</div>
                </div>
              </CardContent>
            </Card>

            <Card>
              <CardHeader>
                <CardTitle>PipelineSpec</CardTitle>
                <CardDescription>Спецификация шагов и графа исполнения.</CardDescription>
              </CardHeader>
              <CardContent>
                <pre className="text-xs whitespace-pre-wrap">{stringifySafe(runSpec.runSpec.pipelineSpec)}</pre>
              </CardContent>
            </Card>

            <Card>
              <CardHeader>
                <CardTitle>Dataset bindings</CardTitle>
                <CardDescription>Привязка версий наборов данных.</CardDescription>
              </CardHeader>
              <CardContent>
                <pre className="text-xs whitespace-pre-wrap">{stringifySafe(runSpec.runSpec.datasetBindings)}</pre>
              </CardContent>
            </Card>

            <Card>
              <CardHeader>
                <CardTitle>Parameters</CardTitle>
                <CardDescription>Параметры конфигурации запуска.</CardDescription>
              </CardHeader>
              <CardContent>
                <pre className="text-xs whitespace-pre-wrap">{stringifySafe(runSpec.runSpec.parameters)}</pre>
              </CardContent>
            </Card>
          </PageSection>

          <PageSection title="Outputs" description="Артефакты, метрики и результаты исполнения.">
            <Card>
              <CardHeader>
                <CardTitle>Артефакты</CardTitle>
                <CardDescription>Список артефактов доступен через отдельный раздел.</CardDescription>
              </CardHeader>
              <CardContent>
                <Link href={`/console/artifacts?run_id=${runId}`} className="text-sm font-semibold text-primary">
                  Перейти к артефактам Run
                </Link>
              </CardContent>
            </Card>
          </PageSection>

          <PageSection title="Audit" description="События исполнения, агрегированные через Gateway.">
            <TableContainer>
              <Table>
                <thead>
                  <tr>
                    <th>Время</th>
                    <th>Уровень</th>
                    <th>Сообщение</th>
                    <th>Актор</th>
                  </tr>
                </thead>
                <tbody>
                  {(events?.events ?? []).map((event) => (
                    <tr key={event.event_id}>
                      <td className="text-xs text-muted-foreground">{formatDateTime(event.occurred_at)}</td>
                      <td className="text-xs">{event.level}</td>
                      <td className="text-xs">{event.message}</td>
                      <td className="text-xs text-muted-foreground">{event.actor}</td>
                    </tr>
                  ))}
                </tbody>
              </Table>
              {(events?.events?.length ?? 0) === 0 ? <TableEmpty>События не зафиксированы.</TableEmpty> : null}
            </TableContainer>
          </PageSection>

          <PageSection title="Diagnostics" description="Контрольные суммы и дополнительные сведения.">
            <Card>
              <CardHeader>
                <CardTitle>Policy snapshot</CardTitle>
                <CardDescription>Зафиксированные политики в момент создания RunSpec.</CardDescription>
              </CardHeader>
              <CardContent>
                <pre className="text-xs whitespace-pre-wrap">{stringifySafe(runSpec.runSpec.policySnapshot)}</pre>
              </CardContent>
            </Card>
          </PageSection>
        </>
      ) : null}
    </PageShell>
  );
}
