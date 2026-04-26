'use client';

import { useMemo, useState } from 'react';

import { ErrorState } from '@/components/console/error-state';
import { CopyButton } from '@/components/console/copy-button';
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

type RunCreateResult = {
  runId: string;
  status: string;
  created: boolean;
  specHash: string;
};

type FormState = {
  idempotencyKey: string;
  pipelineSpec: string;
  datasetBindings: string;
  codeRepo: string;
  codeCommit: string;
  codePath: string;
  codeScm: string;
  envLockId: string;
  parameters: string;
};

const initialState: FormState = {
  idempotencyKey: '',
  pipelineSpec: '{\n  "steps": []\n}',
  datasetBindings: '{\n  "train": "dataset_version_id"\n}',
  codeRepo: '',
  codeCommit: '',
  codePath: '',
  codeScm: '',
  envLockId: '',
  parameters: '{\n  "seed": 42\n}',
};

const parseJSON = (value: string) => {
  if (!value.trim()) {
    return {};
  }
  return JSON.parse(value);
};

export function RunCreateForm({ projectId, role }: { projectId: string; role: EffectiveRole }) {
  const [form, setForm] = useState<FormState>(initialState);
  const [error, setError] = useState<GatewayAPIError | null>(null);
  const [result, setResult] = useState<RunCreateResult | null>(null);
  const [fieldError, setFieldError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const { addOperation, updateOperation } = useOperations();

  const ready = useMemo(() => projectId.trim().length > 0, [projectId]);
  const allowed = useMemo(() => can(role, 'run:write'), [role]);

  const updateField = (key: keyof FormState) => (event: React.ChangeEvent<HTMLInputElement | HTMLTextAreaElement>) => {
    setForm((prev) => ({ ...prev, [key]: event.target.value }));
  };

  const generateIdempotency = () => {
    if (typeof crypto !== 'undefined' && 'randomUUID' in crypto) {
      setForm((prev) => ({ ...prev, idempotencyKey: crypto.randomUUID() }));
    }
  };

  const submit = async () => {
    if (!ready) {
      return;
    }
    setError(null);
    setResult(null);
    setFieldError(null);
    setBusy(true);
    const operationId = `run-create-${Date.now()}`;
    addOperation({
      id: operationId,
      label: 'Создание RunSpec',
      status: 'pending',
      createdAt: new Date().toISOString(),
      details: projectId,
    });

    try {
      const pipelineSpec = parseJSON(form.pipelineSpec);
      const datasetBindings = parseJSON(form.datasetBindings);
      const parameters = parseJSON(form.parameters);
      const body: Record<string, unknown> = {
        pipelineSpec,
        datasetBindings,
        parameters,
        envLock: { lockId: form.envLockId.trim() },
        codeRef: {
          repoUrl: form.codeRepo.trim(),
          commitSha: form.codeCommit.trim(),
          path: form.codePath.trim() || undefined,
          scmType: form.codeScm.trim() || undefined,
        },
      };
      if (form.idempotencyKey.trim()) {
        body.idempotencyKey = form.idempotencyKey.trim();
      }

      const response = await gatewayFetchJSON<RunCreateResult>(`/api/experiments/projects/${projectId}/runs`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Idempotency-Key': form.idempotencyKey.trim() || operationId,
        },
        body: JSON.stringify(body),
      });
      setResult(response);
      updateOperation(operationId, 'succeeded', response.runId);
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
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="flex flex-col gap-6">
      {!ready ? (
        <Card>
          <CardHeader>
            <CardTitle>Project ID не задан</CardTitle>
            <CardDescription>Для создания RunSpec требуется активный project_id.</CardDescription>
          </CardHeader>
        </Card>
      ) : null}
      <Card>
        <CardHeader>
          <CardTitle>Параметры RunSpec</CardTitle>
          <CardDescription>Idempotency-Key обеспечивает неизменяемость и повторяемость операций создания.</CardDescription>
        </CardHeader>
        <CardContent className="flex flex-col gap-5">
          <div className="grid gap-4 md:grid-cols-[1fr_auto]">
            <div className="space-y-2">
              <Label htmlFor="idempotency">Idempotency-Key</Label>
              <Input
                id="idempotency"
                value={form.idempotencyKey}
                onChange={updateField('idempotencyKey')}
                placeholder="uuid или детерминированный ключ"
              />
            </div>
            <div className="flex items-end">
              <Button variant="secondary" size="sm" onClick={generateIdempotency}>
                Сгенерировать
              </Button>
            </div>
          </div>

          <div className="grid gap-4 md:grid-cols-2">
            <div className="space-y-2">
              <Label htmlFor="repo">Repo URL</Label>
              <Input id="repo" value={form.codeRepo} onChange={updateField('codeRepo')} placeholder="https://git/..." />
            </div>
            <div className="space-y-2">
              <Label htmlFor="commit">Commit SHA</Label>
              <Input id="commit" value={form.codeCommit} onChange={updateField('codeCommit')} placeholder="sha256..." />
            </div>
            <div className="space-y-2">
              <Label htmlFor="path">Path (опционально)</Label>
              <Input id="path" value={form.codePath} onChange={updateField('codePath')} placeholder="/workspace" />
            </div>
            <div className="space-y-2">
              <Label htmlFor="scm">SCM Type (опционально)</Label>
              <Input id="scm" value={form.codeScm} onChange={updateField('codeScm')} placeholder="git" />
            </div>
          </div>

          <div className="space-y-2">
            <Label htmlFor="envlock">EnvLock ID</Label>
            <Input
              id="envlock"
              value={form.envLockId}
              onChange={updateField('envLockId')}
              placeholder="env_lock_id"
            />
          </div>

          <div className="space-y-2">
            <Label htmlFor="pipeline">PipelineSpec (JSON)</Label>
            <Textarea id="pipeline" value={form.pipelineSpec} onChange={updateField('pipelineSpec')} />
          </div>

          <div className="space-y-2">
            <Label htmlFor="bindings">Dataset bindings (JSON)</Label>
            <Textarea id="bindings" value={form.datasetBindings} onChange={updateField('datasetBindings')} />
          </div>

          <div className="space-y-2">
            <Label htmlFor="params">Parameters (JSON)</Label>
            <Textarea id="params" value={form.parameters} onChange={updateField('parameters')} />
          </div>

          {fieldError ? <div className="text-sm text-rose-200">Ошибка формы: {fieldError}</div> : null}

          <div className="flex flex-wrap gap-3">
            <Button variant="default" size="sm" onClick={submit} disabled={!ready || busy || !allowed}>
              Создать RunSpec
            </Button>
            <span className="text-xs text-muted-foreground">Данные фиксируются; изменения возможны только через новую версию.</span>
          </div>
          <PolicyHint allowed={allowed} capability="run:write" />
        </CardContent>
      </Card>

      {error ? <ErrorState code={error.code} requestId={error.requestId} status={error.status} details={error.details} message={error.message} retryable={error.retryable} /> : null}

      {result ? (
        <Card>
          <CardHeader>
            <CardTitle>RunSpec создан</CardTitle>
            <CardDescription>Запись неизменяема. Для изменения создайте новую спецификацию.</CardDescription>
          </CardHeader>
          <CardContent className="space-y-3 text-sm">
            <div className="flex items-center gap-2">
              Run ID: <span className="font-mono text-xs">{result.runId}</span>
              <CopyButton value={result.runId} />
            </div>
            <div className="flex items-center gap-2">
              Spec Hash: <span className="font-mono text-xs">{result.specHash}</span>
              <CopyButton value={result.specHash} />
            </div>
            <div className="flex items-center gap-2">
              Статус: <StatusPill status={result.status} />
            </div>
            <div className="text-xs text-muted-foreground">Создано новым запросом: {result.created ? 'да' : 'нет (идемпотентно)'}</div>
          </CardContent>
        </Card>
      ) : null}
    </div>
  );
}
