'use client';

import { useMemo, useState } from 'react';

import { CopyButton } from '@/components/console/copy-button';
import { ErrorState } from '@/components/console/error-state';
import { PolicyHint } from '@/components/console/policy-hint';
import { StatusPill } from '@/components/console/status-pill';
import { Button } from '@/components/ui/button';
import { Table, TableContainer, TableEmpty } from '@/components/ui/table';
import { GatewayAPIError, gatewayFetchJSON } from '@/lib/gateway-client';
import type { components } from '@/lib/gateway-openapi';
import { formatDateTime, formatDurationSeconds } from '@/lib/format';
import { useOperations } from '@/lib/operations';
import { can, type EffectiveRole } from '@/lib/rbac';
import { buildDevEnvProxyUrl } from '@/lib/devenv';

type DevEnv = components['schemas']['DevEnvironment'];

export function DevEnvTable({ environments, projectId, role }: { environments: DevEnv[]; projectId: string; role: EffectiveRole }) {
  const [rows, setRows] = useState<DevEnv[]>(environments);
  const [error, setError] = useState<GatewayAPIError | null>(null);
  const { addOperation, updateOperation } = useOperations();

  const canWrite = useMemo(() => can(role, 'devenv:write'), [role]);
  const canRead = useMemo(() => can(role, 'devenv:read'), [role]);

  const openSession = async (env: DevEnv) => {
    if (!projectId || !canWrite) {
      return;
    }
    setError(null);
    const operationId = `devenv-session-${env.devEnvId}-${Date.now()}`;
    addOperation({
      id: operationId,
      label: 'Открытие IDE‑сессии',
      status: 'pending',
      createdAt: new Date().toISOString(),
      details: env.devEnvId,
    });
    try {
      const response = await gatewayFetchJSON<components['schemas']['DevEnvironmentAccessResponse']>(
        `/api/experiments/projects/${projectId}/devenvs/${env.devEnvId}:open-ide-session`,
        {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ ttlSeconds: 3600 }),
        },
      );
      const url = buildDevEnvProxyUrl(response.proxyPath);
      window.open(url, '_blank', 'noopener');
      updateOperation(operationId, 'succeeded', response.sessionId);
    } catch (err) {
      if (err instanceof GatewayAPIError) {
        setError(err);
        updateOperation(operationId, 'failed', err.code);
      } else {
        updateOperation(operationId, 'failed', 'unknown');
      }
    }
  };

  const stopEnv = async (env: DevEnv) => {
    if (!projectId || !canWrite) {
      return;
    }
    setError(null);
    const operationId = `devenv-stop-${env.devEnvId}-${Date.now()}`;
    addOperation({
      id: operationId,
      label: 'Остановка DevEnv',
      status: 'pending',
      createdAt: new Date().toISOString(),
      details: env.devEnvId,
    });
    try {
      const response = await gatewayFetchJSON<components['schemas']['DevEnvironmentResponse']>(
        `/api/experiments/projects/${projectId}/devenvs/${env.devEnvId}:stop`,
        { method: 'POST' },
      );
      setRows((prev) => prev.map((item) => (item.devEnvId === env.devEnvId ? response.environment : item)));
      updateOperation(operationId, 'succeeded', env.devEnvId);
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
      {error ? <ErrorState code={error.code} requestId={error.requestId} status={error.status} details={error.details} message={error.message} retryable={error.retryable} /> : null}
      <TableContainer>
        <Table>
          <thead>
            <tr>
              <th>ID</th>
              <th>Состояние</th>
              <th>Repo / Ref</th>
              <th>Template</th>
              <th>TTL</th>
              <th>Доступ</th>
            </tr>
          </thead>
          <tbody>
            {rows.map((env) => (
              <tr key={env.devEnvId}>
                <td className="font-mono text-xs">
                  <div className="flex items-center gap-2">
                    {env.devEnvId}
                    <CopyButton value={env.devEnvId} />
                  </div>
                </td>
                <td>
                  <StatusPill status={env.state} />
                </td>
                <td className="text-xs text-muted-foreground">
                  <div>{env.repoUrl}</div>
                  <div className="font-mono">
                    {env.refType}:{env.refValue}
                  </div>
                  {env.commitPin ? <div className="font-mono">pin: {env.commitPin}</div> : null}
                </td>
                <td className="text-xs text-muted-foreground">
                  <div>{env.templateRef}</div>
                  <div>v{env.templateVersion}</div>
                </td>
                <td className="text-xs text-muted-foreground">
                  <div>TTL: {formatDurationSeconds(env.ttlSeconds)}</div>
                  <div>Истекает: {formatDateTime(env.expiresAt)}</div>
                  <div>Доступ: {formatDateTime(env.lastAccessAt)}</div>
                </td>
                <td>
                  <div className="flex flex-wrap gap-2">
                    <Button
                      variant="secondary"
                      size="sm"
                      onClick={() => openSession(env)}
                      disabled={!canWrite || !canRead}
                    >
                      Открыть IDE
                    </Button>
                    <Button
                      variant="secondary"
                      size="sm"
                      onClick={() => stopEnv(env)}
                      disabled={!canWrite}
                    >
                      Остановить
                    </Button>
                  </div>
                  <PolicyHint allowed={canWrite} capability="devenv:write" />
                </td>
              </tr>
            ))}
          </tbody>
        </Table>
        {rows.length === 0 ? <TableEmpty>DevEnv не найдены.</TableEmpty> : null}
      </TableContainer>
    </div>
  );
}
