'use client';

import { useState } from 'react';

import { CopyButton } from '@/components/console/copy-button';
import { ErrorState } from '@/components/console/error-state';
import { PolicyHint } from '@/components/console/policy-hint';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Textarea } from '@/components/ui/textarea';
import { GatewayAPIError, gatewayFetchJSON } from '@/lib/gateway-client';
import { useOperations } from '@/lib/operations';
import { can, type EffectiveRole } from '@/lib/rbac';

export function DatasetCreateForm({ role }: { role: EffectiveRole }) {
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [metadata, setMetadata] = useState('');
  const [error, setError] = useState<GatewayAPIError | null>(null);
  const [fieldError, setFieldError] = useState<string | null>(null);
  const [result, setResult] = useState<{ dataset_id: string } | null>(null);
  const { addOperation, updateOperation } = useOperations();
  const allowed = can(role, 'dataset:write');

  const submit = async () => {
    setError(null);
    setFieldError(null);
    setResult(null);
    const operationId = `dataset-create-${Date.now()}`;
    addOperation({
      id: operationId,
      label: 'Создание датасета',
      status: 'pending',
      createdAt: new Date().toISOString(),
      details: name,
    });
    try {
      const body: Record<string, unknown> = {
        name: name.trim(),
        description: description.trim() || undefined,
      };
      if (metadata.trim()) {
        body.metadata = JSON.parse(metadata);
      }
      const response = await gatewayFetchJSON<{ dataset_id: string }>(`/api/dataset-registry/datasets`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
      });
      setResult(response);
      updateOperation(operationId, 'succeeded', response.dataset_id);
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
          <CardTitle>Создать набор данных</CardTitle>
          <CardDescription>Регистрация нового датасета без загрузки содержимого.</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="name">Название</Label>
            <Input id="name" value={name} onChange={(event) => setName(event.target.value)} />
          </div>
          <div className="space-y-2">
            <Label htmlFor="desc">Описание</Label>
            <Input id="desc" value={description} onChange={(event) => setDescription(event.target.value)} />
          </div>
          <div className="space-y-2">
            <Label htmlFor="meta">Metadata (JSON)</Label>
            <Textarea id="meta" value={metadata} onChange={(event) => setMetadata(event.target.value)} />
          </div>
          {fieldError ? <div className="text-sm text-rose-200">Ошибка формы: {fieldError}</div> : null}
          <Button variant="default" size="sm" onClick={submit} disabled={!allowed}>
            Зарегистрировать
          </Button>
          <PolicyHint allowed={allowed} capability="dataset:write" />
        </CardContent>
      </Card>

      {error ? <ErrorState code={error.code} requestId={error.requestId} status={error.status} details={error.details} message={error.message} retryable={error.retryable} /> : null}

      {result ? (
        <Card>
          <CardHeader>
            <CardTitle>Dataset создан</CardTitle>
            <CardDescription>Используйте dataset_id для загрузки версий.</CardDescription>
          </CardHeader>
          <CardContent className="text-sm">
            <div className="flex items-center gap-2">
              Dataset ID: <span className="font-mono text-xs">{result.dataset_id}</span>
              <CopyButton value={result.dataset_id} />
            </div>
          </CardContent>
        </Card>
      ) : null}
    </div>
  );
}
