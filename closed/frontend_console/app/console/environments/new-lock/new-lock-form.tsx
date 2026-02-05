'use client';

import { useState } from 'react';

import { CopyButton } from '@/components/console/copy-button';
import { ErrorState } from '@/components/console/error-state';
import { StatusPill } from '@/components/console/status-pill';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Textarea } from '@/components/ui/textarea';
import { GatewayAPIError, gatewayFetchJSON } from '@/lib/gateway-client';
import { useOperations } from '@/lib/operations';

type LockResponse = {
  lock: {
    lockId: string;
    envHash: string;
  };
  created: boolean;
};

export function NewLockForm({ projectId }: { projectId: string }) {
  const [envId, setEnvId] = useState('');
  const [digests, setDigests] = useState('{\n  "runtime": "sha256:..."\n}');
  const [idempotencyKey, setIdempotencyKey] = useState('');
  const [result, setResult] = useState<LockResponse | null>(null);
  const [error, setError] = useState<GatewayAPIError | null>(null);
  const [fieldError, setFieldError] = useState<string | null>(null);
  const { addOperation, updateOperation } = useOperations();

  const submit = async () => {
    if (!projectId) {
      return;
    }
    setError(null);
    setFieldError(null);
    setResult(null);
    const operationId = `env-lock-${Date.now()}`;
    addOperation({
      id: operationId,
      label: 'Создание EnvLock',
      status: 'pending',
      createdAt: new Date().toISOString(),
      details: envId,
    });
    try {
      const imageDigests = JSON.parse(digests);
      const body: Record<string, unknown> = {
        environmentDefinitionId: envId.trim(),
        imageDigests,
      };
      if (idempotencyKey.trim()) {
        body.idempotencyKey = idempotencyKey.trim();
      }
      const response = await gatewayFetchJSON<LockResponse>(`/api/experiments/projects/${projectId}/environment-locks`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Idempotency-Key': idempotencyKey.trim() || operationId,
        },
        body: JSON.stringify(body),
      });
      setResult(response);
      updateOperation(operationId, 'succeeded', response.lock.lockId);
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
    <div className="flex flex-col gap-6">
      {!projectId ? (
        <Card>
          <CardHeader>
            <CardTitle>Project ID не задан</CardTitle>
            <CardDescription>Для создания EnvLock требуется активный project_id.</CardDescription>
          </CardHeader>
        </Card>
      ) : null}

      <Card>
        <CardHeader>
          <CardTitle>Параметры блокировки</CardTitle>
          <CardDescription>Digest‑пины и idempotency‑ключ обеспечивают детерминированный контроль образов.</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="env-id">EnvironmentDefinition ID</Label>
            <Input id="env-id" value={envId} onChange={(event) => setEnvId(event.target.value)} />
          </div>
          <div className="space-y-2">
            <Label htmlFor="digests">Image digests (JSON)</Label>
            <Textarea id="digests" value={digests} onChange={(event) => setDigests(event.target.value)} />
          </div>
          <div className="space-y-2">
            <Label htmlFor="idem">Idempotency-Key</Label>
            <Input id="idem" value={idempotencyKey} onChange={(event) => setIdempotencyKey(event.target.value)} />
          </div>
          {fieldError ? <div className="text-sm text-rose-200">Ошибка формы: {fieldError}</div> : null}
          <Button variant="default" size="sm" onClick={submit} disabled={!projectId}>
            Создать EnvLock
          </Button>
        </CardContent>
      </Card>

      {error ? <ErrorState code={error.code} requestId={error.requestId} status={error.status} details={error.details} /> : null}

      {result ? (
        <Card>
          <CardHeader>
            <CardTitle>EnvLock создан</CardTitle>
            <CardDescription>Запись неизменяема и привязана к digest‑пинам.</CardDescription>
          </CardHeader>
          <CardContent className="space-y-2 text-sm">
            <div className="flex items-center gap-2">
              Lock ID: <span className="font-mono text-xs">{result.lock.lockId}</span>
              <CopyButton value={result.lock.lockId} />
            </div>
            <div className="flex items-center gap-2">
              Env Hash: <span className="font-mono text-xs">{result.lock.envHash}</span>
              <CopyButton value={result.lock.envHash} />
            </div>
            <div className="flex items-center gap-2">
              Статус: <StatusPill status={result.created ? 'created' : 'existing'} label={result.created ? 'создано' : 'идемпотентно'} />
            </div>
          </CardContent>
        </Card>
      ) : null}
    </div>
  );
}
