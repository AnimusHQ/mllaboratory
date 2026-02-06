'use client';

import { useState } from 'react';
import Link from 'next/link';

import { CopyButton } from '@/components/console/copy-button';
import { ErrorState } from '@/components/console/error-state';
import { PolicyHint } from '@/components/console/policy-hint';
import { StatusPill } from '@/components/console/status-pill';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Table, TableContainer, TableEmpty } from '@/components/ui/table';
import { GatewayAPIError, gatewayFetchJSON } from '@/lib/gateway-client';
import type { components } from '@/lib/gateway-openapi';
import { formatDateTime } from '@/lib/format';
import { useOperations } from '@/lib/operations';
import { can, type EffectiveRole } from '@/lib/rbac';

type ModelVersion = components['schemas']['ModelVersion'];

export function ModelVersionsTable({
  projectId,
  modelId,
  versions,
  role,
}: {
  projectId: string;
  modelId: string;
  versions: ModelVersion[];
  role: EffectiveRole;
}) {
  const [rows, setRows] = useState<ModelVersion[]>(versions);
  const [error, setError] = useState<GatewayAPIError | null>(null);
  const [targets, setTargets] = useState<Record<string, string>>({});
  const { addOperation, updateOperation } = useOperations();

  const updateRow = (updated: ModelVersion) => {
    setRows((prev) => prev.map((row) => (row.modelVersionId === updated.modelVersionId ? updated : row)));
  };

  const transition = async (action: 'validate' | 'approve' | 'deprecate', version: ModelVersion) => {
    if (!projectId) {
      return;
    }
    setError(null);
    const operationId = `model-${action}-${version.modelVersionId}-${Date.now()}`;
    addOperation({
      id: operationId,
      label: `Model ${action}`,
      status: 'pending',
      createdAt: new Date().toISOString(),
      details: version.modelVersionId,
    });
    try {
      const response = await gatewayFetchJSON<components['schemas']['ModelVersionTransitionResponse']>(
        `/projects/${projectId}/model-versions/${version.modelVersionId}:${action}`,
        { method: 'POST' },
      );
      updateRow(response.modelVersion);
      updateOperation(operationId, 'succeeded', response.modelVersion.status);
    } catch (err) {
      if (err instanceof GatewayAPIError) {
        setError(err);
        updateOperation(operationId, 'failed', err.code);
      } else {
        updateOperation(operationId, 'failed', 'unknown');
      }
    }
  };

  const exportVersion = async (version: ModelVersion) => {
    if (!projectId) {
      return;
    }
    const target = targets[version.modelVersionId] ?? '';
    setError(null);
    const operationId = `model-export-${version.modelVersionId}-${Date.now()}`;
    addOperation({
      id: operationId,
      label: 'Model export',
      status: 'pending',
      createdAt: new Date().toISOString(),
      details: version.modelVersionId,
    });
    try {
      const response = await gatewayFetchJSON<components['schemas']['ModelExportResponse']>(
        `/projects/${projectId}/model-versions/${version.modelVersionId}:export`,
        {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ target: target.trim() || undefined }),
        },
      );
      updateOperation(operationId, 'succeeded', response.export.exportId);
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
              <th>Model Version</th>
              <th>Status</th>
              <th>Run / Artifacts</th>
              <th>Создано</th>
              <th>Действия</th>
            </tr>
          </thead>
          <tbody>
            {rows.map((version) => (
              <tr key={version.modelVersionId}>
                <td className="font-mono text-xs">
                  <div className="flex items-center gap-2">
                    {version.modelVersionId}
                    <CopyButton value={version.modelVersionId} />
                  </div>
                  <div className="text-xs text-muted-foreground">v{version.version}</div>
                </td>
                <td>
                  <StatusPill status={version.status} />
                </td>
                <td className="text-xs text-muted-foreground">
                  <div className="flex items-center gap-2">
                    Run: {version.runId}
                    <CopyButton value={version.runId} />
                  </div>
                  <div>Артефакты: {version.artifactIds?.length ?? 0}</div>
                  <Link
                    href={`/console/lineage?model_version_id=${version.modelVersionId}`}
                    className="text-xs font-semibold text-primary"
                  >
                    Lineage
                  </Link>
                </td>
                <td className="text-xs text-muted-foreground">{formatDateTime(version.createdAt)}</td>
                <td>
                  <div className="flex flex-col gap-2">
                    <div className="flex flex-wrap gap-2">
                      <Button
                        variant="secondary"
                        size="sm"
                        onClick={() => transition('validate', version)}
                        disabled={!can(role, 'model:write')}
                      >
                        Validate
                      </Button>
                      <Button
                        variant="secondary"
                        size="sm"
                        onClick={() => transition('approve', version)}
                        disabled={!can(role, 'model:approve')}
                      >
                        Approve
                      </Button>
                      <Button
                        variant="secondary"
                        size="sm"
                        onClick={() => transition('deprecate', version)}
                        disabled={!can(role, 'model:approve')}
                      >
                        Deprecate
                      </Button>
                    </div>
                    <div className="flex flex-wrap items-center gap-2">
                      <Input
                        value={targets[version.modelVersionId] ?? ''}
                        onChange={(event) =>
                          setTargets((prev) => ({ ...prev, [version.modelVersionId]: event.target.value }))
                        }
                        placeholder="target для экспорта"
                        className="h-8 w-44 text-xs"
                      />
                      <Button
                        variant="default"
                        size="sm"
                        onClick={() => exportVersion(version)}
                        disabled={!can(role, 'model:export')}
                      >
                        Экспорт
                      </Button>
                    </div>
                    <div className="space-y-1">
                      <PolicyHint allowed={can(role, 'model:write')} capability="model:write" />
                      <PolicyHint allowed={can(role, 'model:approve')} capability="model:approve" />
                      <PolicyHint allowed={can(role, 'model:export')} capability="model:export" />
                    </div>
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </Table>
        {rows.length === 0 ? <TableEmpty>Версии модели не найдены.</TableEmpty> : null}
      </TableContainer>
    </div>
  );
}
