'use client';

import { useState } from 'react';

import { CopyButton } from '@/components/console/copy-button';
import { ErrorState } from '@/components/console/error-state';
import { StatusPill } from '@/components/console/status-pill';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { GatewayAPIError, gatewayFetchJSON } from '@/lib/gateway-client';
import { useOperations } from '@/lib/operations';

type ModelVersionCreateResult = {
  modelVersion: {
    modelVersionId: string;
    status: string;
    version: string;
  };
  created: boolean;
};

const parseList = (value: string) =>
  value
    .split(',')
    .map((item) => item.trim())
    .filter(Boolean);

export function NewModelVersionForm({ projectId }: { projectId: string }) {
  const [modelId, setModelId] = useState('');
  const [version, setVersion] = useState('');
  const [runId, setRunId] = useState('');
  const [artifactIds, setArtifactIds] = useState('');
  const [datasetIds, setDatasetIds] = useState('');
  const [idempotencyKey, setIdempotencyKey] = useState('');
  const [result, setResult] = useState<ModelVersionCreateResult | null>(null);
  const [error, setError] = useState<GatewayAPIError | null>(null);
  const { addOperation, updateOperation } = useOperations();

  const submit = async () => {
    if (!projectId) {
      return;
    }
    setError(null);
    setResult(null);
    const operationId = `model-version-${Date.now()}`;
    addOperation({
      id: operationId,
      label: 'Создание версии модели',
      status: 'pending',
      createdAt: new Date().toISOString(),
      details: modelId,
    });
    try {
      const body: Record<string, unknown> = {
        version: version.trim(),
        runId: runId.trim(),
        artifactIds: parseList(artifactIds),
        datasetVersionIds: parseList(datasetIds),
      };
      if (idempotencyKey.trim()) {
        body.idempotencyKey = idempotencyKey.trim();
      }
      const response = await gatewayFetchJSON<ModelVersionCreateResult>(
        `/projects/${projectId}/models/${modelId}/versions`,
        {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
            'Idempotency-Key': idempotencyKey.trim() || operationId,
          },
          body: JSON.stringify(body),
        },
      );
      setResult(response);
      updateOperation(operationId, 'succeeded', response.modelVersion.modelVersionId);
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
    <div className="flex flex-col gap-6">
      {!projectId ? (
        <Card>
          <CardHeader>
            <CardTitle>Project ID не задан</CardTitle>
            <CardDescription>Для создания версии требуется активный project_id.</CardDescription>
          </CardHeader>
        </Card>
      ) : null}
      <Card>
        <CardHeader>
          <CardTitle>Параметры версии</CardTitle>
          <CardDescription>Заполните provenance‑поля. Идентификаторы разделяются запятыми.</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="model-id">Model ID</Label>
            <Input id="model-id" value={modelId} onChange={(event) => setModelId(event.target.value)} />
          </div>
          <div className="space-y-2">
            <Label htmlFor="version">Version</Label>
            <Input id="version" value={version} onChange={(event) => setVersion(event.target.value)} />
          </div>
          <div className="space-y-2">
            <Label htmlFor="run-id">Run ID</Label>
            <Input id="run-id" value={runId} onChange={(event) => setRunId(event.target.value)} />
          </div>
          <div className="space-y-2">
            <Label htmlFor="artifact-ids">Artifact IDs</Label>
            <Input
              id="artifact-ids"
              value={artifactIds}
              onChange={(event) => setArtifactIds(event.target.value)}
              placeholder="artifact_id1, artifact_id2"
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="dataset-ids">Dataset Version IDs</Label>
            <Input
              id="dataset-ids"
              value={datasetIds}
              onChange={(event) => setDatasetIds(event.target.value)}
              placeholder="dataset_version_id1, dataset_version_id2"
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="idem">Idempotency-Key</Label>
            <Input id="idem" value={idempotencyKey} onChange={(event) => setIdempotencyKey(event.target.value)} />
          </div>
          <Button variant="default" size="sm" onClick={submit} disabled={!projectId}>
            Создать версию
          </Button>
        </CardContent>
      </Card>

      {error ? <ErrorState code={error.code} requestId={error.requestId} status={error.status} details={error.details} /> : null}

      {result ? (
        <Card>
          <CardHeader>
            <CardTitle>Версия создана</CardTitle>
            <CardDescription>Дальнейшие переходы выполняются через validate/approve/deprecate.</CardDescription>
          </CardHeader>
          <CardContent className="space-y-2 text-sm">
            <div className="flex items-center gap-2">
              Version ID: <span className="font-mono text-xs">{result.modelVersion.modelVersionId}</span>
              <CopyButton value={result.modelVersion.modelVersionId} />
            </div>
            <div className="flex items-center gap-2">
              Статус: <StatusPill status={result.modelVersion.status} />
            </div>
            <div className="text-xs text-muted-foreground">Создано: {result.created ? 'да' : 'нет (идемпотентно)'}</div>
          </CardContent>
        </Card>
      ) : null}
    </div>
  );
}
