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
import { GatewayAPIError, gatewayFetchJSON } from '@/lib/gateway-client';
import { useOperations } from '@/lib/operations';
import { can, type EffectiveRole } from '@/lib/rbac';

type DevEnvResponse = {
  environment: {
    devEnvId: string;
    state: string;
    expiresAt: string;
  };
  created: boolean;
};

export function NewDevEnvForm({ projectId, role }: { projectId: string; role: EffectiveRole }) {
  const [templateRef, setTemplateRef] = useState('');
  const [repoUrl, setRepoUrl] = useState('');
  const [refType, setRefType] = useState<'branch' | 'tag' | 'commit'>('branch');
  const [refValue, setRefValue] = useState('');
  const [commitPin, setCommitPin] = useState('');
  const [ttlSeconds, setTtlSeconds] = useState('3600');
  const [idempotencyKey, setIdempotencyKey] = useState('');
  const [result, setResult] = useState<DevEnvResponse | null>(null);
  const [error, setError] = useState<GatewayAPIError | null>(null);
  const { addOperation, updateOperation } = useOperations();
  const allowed = can(role, 'devenv:write');

  const submit = async () => {
    if (!projectId) {
      return;
    }
    setError(null);
    setResult(null);
    const operationId = `devenv-create-${Date.now()}`;
    addOperation({
      id: operationId,
      label: 'Создание DevEnv',
      status: 'pending',
      createdAt: new Date().toISOString(),
      details: repoUrl,
    });
    try {
      const body: Record<string, unknown> = {
        templateRef: templateRef.trim(),
        repoUrl: repoUrl.trim(),
        refType,
        refValue: refValue.trim(),
        commitPin: commitPin.trim() || undefined,
        ttlSeconds: Number(ttlSeconds),
      };
      if (idempotencyKey.trim()) {
        body.idempotencyKey = idempotencyKey.trim();
      }
      const response = await gatewayFetchJSON<DevEnvResponse>(`/api/experiments/projects/${projectId}/devenvs`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Idempotency-Key': idempotencyKey.trim() || operationId,
        },
        body: JSON.stringify(body),
      });
      setResult(response);
      const finalStatus = response.environment.state === 'active' ? 'succeeded' : 'pending';
      updateOperation(operationId, finalStatus, response.environment.devEnvId, {
        poll: { kind: 'devenv', projectId, devEnvId: response.environment.devEnvId },
      });
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
            <CardDescription>Для создания DevEnv требуется активный project_id.</CardDescription>
          </CardHeader>
        </Card>
      ) : null}
      <Card>
        <CardHeader>
          <CardTitle>Параметры DevEnv</CardTitle>
          <CardDescription>Repo URL и ref фиксируются. Дополнительно можно закрепить commit_pin.</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="template">Template ref</Label>
            <Input id="template" value={templateRef} onChange={(event) => setTemplateRef(event.target.value)} />
          </div>
          <div className="space-y-2">
            <Label htmlFor="repo">Repo URL</Label>
            <Input id="repo" value={repoUrl} onChange={(event) => setRepoUrl(event.target.value)} />
          </div>
          <div className="grid gap-3 md:grid-cols-2">
            <div className="space-y-2">
              <Label htmlFor="ref-type">Ref type</Label>
              <Input id="ref-type" value={refType} onChange={(event) => setRefType(event.target.value as any)} />
            </div>
            <div className="space-y-2">
              <Label htmlFor="ref-value">Ref value</Label>
              <Input id="ref-value" value={refValue} onChange={(event) => setRefValue(event.target.value)} />
            </div>
          </div>
          <div className="grid gap-3 md:grid-cols-2">
            <div className="space-y-2">
              <Label htmlFor="commit-pin">Commit pin (опционально)</Label>
              <Input id="commit-pin" value={commitPin} onChange={(event) => setCommitPin(event.target.value)} />
            </div>
            <div className="space-y-2">
              <Label htmlFor="ttl">TTL (сек)</Label>
              <Input id="ttl" value={ttlSeconds} onChange={(event) => setTtlSeconds(event.target.value)} />
            </div>
          </div>
          <div className="space-y-2">
            <Label htmlFor="idem">Idempotency-Key</Label>
            <Input id="idem" value={idempotencyKey} onChange={(event) => setIdempotencyKey(event.target.value)} />
          </div>
          <Button variant="default" size="sm" onClick={submit} disabled={!projectId || !allowed}>
            Создать DevEnv
          </Button>
          <PolicyHint allowed={allowed} capability="devenv:write" />
        </CardContent>
      </Card>

      {error ? <ErrorState code={error.code} requestId={error.requestId} status={error.status} details={error.details} message={error.message} retryable={error.retryable} /> : null}

      {result ? (
        <Card>
          <CardHeader>
            <CardTitle>DevEnv создан</CardTitle>
            <CardDescription>Сессии IDE открываются через прокси.</CardDescription>
          </CardHeader>
          <CardContent className="space-y-2 text-sm">
            <div className="flex items-center gap-2">
              DevEnv ID: <span className="font-mono text-xs">{result.environment.devEnvId}</span>
              <CopyButton value={result.environment.devEnvId} />
            </div>
            <div className="flex items-center gap-2">
              Статус: <StatusPill status={result.environment.state} />
            </div>
            <div className="text-xs text-muted-foreground">Экспирация: {result.environment.expiresAt}</div>
          </CardContent>
        </Card>
      ) : null}
    </div>
  );
}
