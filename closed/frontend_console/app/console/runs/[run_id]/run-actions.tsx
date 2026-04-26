'use client';

import { useState } from 'react';

import { CopyButton } from '@/components/console/copy-button';
import { ErrorState } from '@/components/console/error-state';
import { PolicyHint } from '@/components/console/policy-hint';
import { StatusPill } from '@/components/console/status-pill';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { GatewayAPIError, gatewayFetchJSON } from '@/lib/gateway-client';
import { stringifySafe } from '@/lib/sanitize';
import { useOperations } from '@/lib/operations';
import { can, type EffectiveRole } from '@/lib/rbac';

type DispatchResponse = {
  runId: string;
  projectId: string;
  dispatchId: string;
  status: string;
  created: boolean;
  dpBaseUrl?: string;
};

export function RunActions({ projectId, runId, role }: { projectId: string; runId: string; role: EffectiveRole }) {
  const [error, setError] = useState<GatewayAPIError | null>(null);
  const [plan, setPlan] = useState<Record<string, unknown> | null>(null);
  const [dryRun, setDryRun] = useState<Record<string, unknown> | null>(null);
  const [dispatch, setDispatch] = useState<DispatchResponse | null>(null);
  const [bundle, setBundle] = useState<Record<string, unknown> | null>(null);
  const [idempotency, setIdempotency] = useState('');
  const { addOperation, updateOperation } = useOperations();
  const canWrite = can(role, 'run:write');
  const canRead = can(role, 'run:read');

  const invoke = async (
    action: string,
    fn: () => Promise<void>,
    options?: { poll?: { kind: 'run'; projectId: string; runId: string }; retry?: { kind: 'dispatch-run'; projectId: string; runId: string } },
  ) => {
    setError(null);
    const operationId = `${action}-${Date.now()}`;
    addOperation({
      id: operationId,
      label: action,
      status: 'pending',
      createdAt: new Date().toISOString(),
      details: runId,
      poll: options?.poll,
      retry: options?.retry,
    });
    try {
      await fn();
      updateOperation(operationId, 'succeeded', runId);
    } catch (err) {
      if (err instanceof GatewayAPIError) {
        setError(err);
        updateOperation(operationId, 'failed', err.code);
      } else {
        updateOperation(operationId, 'failed', 'unknown');
      }
    }
  };

  return (
    <div className="flex flex-col gap-4">
      <Card>
        <CardHeader>
          <CardTitle>Операции Run</CardTitle>
          <CardDescription>Планирование, dry‑run и диспетчеризация выполняются через Gateway API.</CardDescription>
        </CardHeader>
        <CardContent className="flex flex-col gap-3">
          <div className="grid gap-3 md:grid-cols-[1fr_auto]">
            <Input
              value={idempotency}
              onChange={(event) => setIdempotency(event.target.value)}
              placeholder="Idempotency-Key для dispatch (опционально)"
            />
            <Button
              variant="secondary"
              size="sm"
              onClick={() => {
                if (typeof crypto !== 'undefined' && 'randomUUID' in crypto) {
                  setIdempotency(crypto.randomUUID());
                }
              }}
            >
              Сгенерировать
            </Button>
          </div>
          <div className="flex flex-wrap gap-2">
            <Button
              variant="secondary"
              size="sm"
              onClick={() =>
                invoke('Планирование Run', async () => {
                  const response = await gatewayFetchJSON(`/api/experiments/projects/${projectId}/runs/${runId}:plan`, {
                    method: 'POST',
                  });
                  setPlan(response as Record<string, unknown>);
                })
              }
              disabled={!canWrite}
            >
              Построить план
            </Button>
            <Button
              variant="secondary"
              size="sm"
              onClick={() =>
                invoke('Dry‑run Run', async () => {
                  const response = await gatewayFetchJSON(`/api/experiments/projects/${projectId}/runs/${runId}:dry-run`, {
                    method: 'POST',
                  });
                  setDryRun(response as Record<string, unknown>);
                })
              }
              disabled={!canWrite}
            >
              Dry‑run
            </Button>
            <Button
              variant="default"
              size="sm"
              onClick={() =>
                invoke(
                  'Диспетчеризация Run',
                  async () => {
                    const response = await gatewayFetchJSON<DispatchResponse>(
                      `/api/experiments/projects/${projectId}/runs/${runId}:dispatch`,
                      {
                        method: 'POST',
                        headers: {
                          'Content-Type': 'application/json',
                          ...(idempotency.trim() ? { 'Idempotency-Key': idempotency.trim() } : {}),
                        },
                        body: JSON.stringify({ idempotencyKey: idempotency.trim() || undefined }),
                      },
                    );
                    setDispatch(response);
                  },
                  {
                    poll: { kind: 'run', projectId, runId },
                    retry: { kind: 'dispatch-run', projectId, runId },
                  },
                )
              }
              disabled={!canWrite}
            >
              Диспетчеризировать
            </Button>
            <Button
              variant="secondary"
              size="sm"
              onClick={() =>
                invoke('Reproducibility bundle', async () => {
                  const response = await gatewayFetchJSON(
                    `/api/experiments/projects/${projectId}/runs/${runId}/reproducibility-bundle`,
                    {
                      method: 'GET',
                    },
                  );
                  setBundle(response as Record<string, unknown>);
                })
              }
              disabled={!canRead}
            >
              Reproducibility bundle
            </Button>
          </div>
          <div className="text-xs text-muted-foreground">
            Отмена выполнения доступна только через DP‑контур. В консоли отражаются статусы и планирование.
          </div>
          <PolicyHint allowed={canWrite} capability="run:write" />
          <PolicyHint allowed={canRead} capability="run:read" />
        </CardContent>
      </Card>

      {error ? <ErrorState code={error.code} requestId={error.requestId} status={error.status} details={error.details} message={error.message} retryable={error.retryable} /> : null}

      {dispatch ? (
        <Card>
          <CardHeader>
            <CardTitle>Dispatch результат</CardTitle>
            <CardDescription>Идемпотентная операция. Повторный запрос возвращает тот же dispatch_id.</CardDescription>
          </CardHeader>
          <CardContent className="text-sm space-y-2">
            <div className="flex items-center gap-2">
              Dispatch ID: <span className="font-mono text-xs">{dispatch.dispatchId}</span>
              <CopyButton value={dispatch.dispatchId} />
            </div>
            <div className="flex items-center gap-2">
              Статус: <StatusPill status={dispatch.status} />
            </div>
            <div className="text-xs text-muted-foreground">Создано: {dispatch.created ? 'да' : 'нет (идемпотентно)'}</div>
          </CardContent>
        </Card>
      ) : null}

      {plan ? (
        <Card>
          <CardHeader>
            <CardTitle>План выполнения</CardTitle>
            <CardDescription>Сводка шагов и состояния.</CardDescription>
          </CardHeader>
          <CardContent>
            <pre className="text-xs whitespace-pre-wrap">{stringifySafe(plan)}</pre>
          </CardContent>
        </Card>
      ) : null}

      {dryRun ? (
        <Card>
          <CardHeader>
            <CardTitle>Dry‑run результат</CardTitle>
            <CardDescription>Проверка доступности ресурсов без фактического запуска.</CardDescription>
          </CardHeader>
          <CardContent>
            <pre className="text-xs whitespace-pre-wrap">{stringifySafe(dryRun)}</pre>
          </CardContent>
        </Card>
      ) : null}

      {bundle ? (
        <Card>
          <CardHeader>
            <CardTitle>Reproducibility bundle</CardTitle>
            <CardDescription>JSON‑снимок входов и policy snapshot.</CardDescription>
          </CardHeader>
          <CardContent>
            <pre className="text-xs whitespace-pre-wrap">{stringifySafe(bundle)}</pre>
          </CardContent>
        </Card>
      ) : null}
    </div>
  );
}
