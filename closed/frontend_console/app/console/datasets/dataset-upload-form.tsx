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

export function DatasetUploadForm({ datasetId, role }: { datasetId: string; role: EffectiveRole }) {
  const [file, setFile] = useState<File | null>(null);
  const [metadata, setMetadata] = useState('');
  const [qualityRuleId, setQualityRuleId] = useState('');
  const [error, setError] = useState<GatewayAPIError | null>(null);
  const [fieldError, setFieldError] = useState<string | null>(null);
  const [result, setResult] = useState<{ version_id: string } | null>(null);
  const { addOperation, updateOperation } = useOperations();
  const allowed = can(role, 'dataset:write');

  const submit = async () => {
    if (!datasetId || !file) {
      setFieldError('Файл и dataset_id обязательны');
      return;
    }
    setError(null);
    setFieldError(null);
    setResult(null);
    const operationId = `dataset-upload-${Date.now()}`;
    addOperation({
      id: operationId,
      label: 'Загрузка версии датасета',
      status: 'pending',
      createdAt: new Date().toISOString(),
      details: datasetId,
    });
    try {
      const form = new FormData();
      form.append('file', file);
      if (metadata.trim()) {
        form.append('metadata', metadata.trim());
      }
      if (qualityRuleId.trim()) {
        form.append('quality_rule_id', qualityRuleId.trim());
      }
      const response = await gatewayFetchJSON<{ version_id: string }>(
        `/api/dataset-registry/datasets/${datasetId}/versions/upload`,
        {
          method: 'POST',
          body: form,
        },
      );
      setResult(response);
      updateOperation(operationId, 'succeeded', response.version_id);
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
          <CardTitle>Загрузка версии</CardTitle>
          <CardDescription>Данные отправляются в Dataset Registry через Gateway.</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="file">Файл</Label>
            <Input id="file" type="file" onChange={(event) => setFile(event.target.files?.[0] ?? null)} />
          </div>
          <div className="space-y-2">
            <Label htmlFor="meta">Metadata (JSON строка)</Label>
            <Textarea id="meta" value={metadata} onChange={(event) => setMetadata(event.target.value)} />
          </div>
          <div className="space-y-2">
            <Label htmlFor="quality">Quality rule ID (опционально)</Label>
            <Input id="quality" value={qualityRuleId} onChange={(event) => setQualityRuleId(event.target.value)} />
          </div>
          {fieldError ? <div className="text-sm text-rose-200">Ошибка формы: {fieldError}</div> : null}
          <Button variant="default" size="sm" onClick={submit} disabled={!allowed}>
            Загрузить версию
          </Button>
          <PolicyHint allowed={allowed} capability="dataset:write" />
        </CardContent>
      </Card>

      {error ? <ErrorState code={error.code} requestId={error.requestId} status={error.status} details={error.details} message={error.message} retryable={error.retryable} /> : null}

      {result ? (
        <Card>
          <CardHeader>
            <CardTitle>Версия создана</CardTitle>
            <CardDescription>DatasetVersion зарегистрирован и доступен для загрузки.</CardDescription>
          </CardHeader>
          <CardContent className="text-sm">
            <div className="flex items-center gap-2">
              Version ID: <span className="font-mono text-xs">{result.version_id}</span>
              <CopyButton value={result.version_id} />
            </div>
          </CardContent>
        </Card>
      ) : null}
    </div>
  );
}
