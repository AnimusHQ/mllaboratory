'use client';

import { useState } from 'react';

import { CopyButton } from '@/components/console/copy-button';
import { ErrorState } from '@/components/console/error-state';
import { PolicyHint } from '@/components/console/policy-hint';
import { StatusPill } from '@/components/console/status-pill';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Textarea } from '@/components/ui/textarea';
import { GatewayAPIError, gatewayFetchJSON } from '@/lib/gateway-client';
import { useOperations } from '@/lib/operations';
import { can, type EffectiveRole } from '@/lib/rbac';

type ModelCreateResult = {
  model: {
    modelId: string;
    name: string;
    status: string;
  };
  created: boolean;
};

export function ModelCreateForm({ projectId, role }: { projectId: string; role: EffectiveRole }) {
  const [name, setName] = useState('');
  const [metadata, setMetadata] = useState('{\n  "domain": "nlp"\n}');
  const [idempotencyKey, setIdempotencyKey] = useState('');
  const [error, setError] = useState<GatewayAPIError | null>(null);
  const [fieldError, setFieldError] = useState<string | null>(null);
  const [result, setResult] = useState<ModelCreateResult | null>(null);
  const { addOperation, updateOperation } = useOperations();
  const allowed = can(role, 'model:write');

  const submit = async () => {
    if (!projectId) {
      return;
    }
    setError(null);
    setFieldError(null);
    setResult(null);
    const operationId = `model-create-${Date.now()}`;
    addOperation({
      id: operationId,
      label: 'Создание модели',
      status: 'pending',
      createdAt: new Date().toISOString(),
      details: name,
    });
    try {
      const body: Record<string, unknown> = {
        name: name.trim(),
        metadata: metadata.trim() ? JSON.parse(metadata) : undefined,
      };
      if (idempotencyKey.trim()) {
        body.idempotencyKey = idempotencyKey.trim();
      }
      const response = await gatewayFetchJSON<ModelCreateResult>(`/projects/${projectId}/models`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Idempotency-Key': idempotencyKey.trim() || operationId,
        },
        body: JSON.stringify(body),
      });
      setResult(response);
      updateOperation(operationId, 'succeeded', response.model.modelId);
    } catch (err) {
      if (err instanceof GatewayAPIError) {
        setError(err);
        updateOperation(operationId, 'failed', err.code);
      } else if (err instanceof Error) {
        setFieldError(err.message);
        updateOperation(operationId, 'failed', err.message);
      } else {
        setFieldError('Неизвестная ошибка');
        updateOperation(operationId, 'failed', 'unknown');
      }
    }
  };

  return (
    <div className="flex flex-col gap-4">
      <Card>
        <CardHeader>
          <CardTitle>Создать модель</CardTitle>
          <CardDescription>Создание логической модели (контейнер версий).</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="name">Название</Label>
            <Input id="name" value={name} onChange={(event) => setName(event.target.value)} />
          </div>
          <div className="space-y-2">
            <Label htmlFor="meta">Metadata (JSON)</Label>
            <Textarea id="meta" value={metadata} onChange={(event) => setMetadata(event.target.value)} />
          </div>
          <div className="space-y-2">
            <Label htmlFor="idem">Idempotency-Key</Label>
            <Input id="idem" value={idempotencyKey} onChange={(event) => setIdempotencyKey(event.target.value)} />
          </div>
          {fieldError ? <div className="text-sm text-rose-200">Ошибка формы: {fieldError}</div> : null}
          <Button variant="default" size="sm" onClick={submit} disabled={!projectId || !allowed}>
            Создать модель
          </Button>
          <PolicyHint allowed={allowed} capability="model:write" />
        </CardContent>
      </Card>

      {error ? <ErrorState code={error.code} requestId={error.requestId} status={error.status} details={error.details} message={error.message} retryable={error.retryable} /> : null}

      {result ? (
        <Card>
          <CardHeader>
            <CardTitle>Модель создана</CardTitle>
            <CardDescription>Созданный контейнер используется для версий.</CardDescription>
          </CardHeader>
          <CardContent className="space-y-2 text-sm">
            <div className="flex items-center gap-2">
              Model ID: <span className="font-mono text-xs">{result.model.modelId}</span>
              <CopyButton value={result.model.modelId} />
            </div>
            <div className="flex items-center gap-2">
              Статус: <StatusPill status={result.model.status} />
            </div>
            <div className="text-xs text-muted-foreground">Создано: {result.created ? 'да' : 'нет (идемпотентно)'}</div>
          </CardContent>
        </Card>
      ) : null}
    </div>
  );
}
