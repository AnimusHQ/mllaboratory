'use client';

import { useMemo } from 'react';

import { Button } from '@/components/ui/button';
import { describeError } from '@/lib/error-messages';

export type ErrorStateProps = {
  code?: string;
  requestId?: string;
  status?: number;
  details?: unknown;
};

export function ErrorState({ code, requestId, status, details }: ErrorStateProps) {
  const descriptor = useMemo(() => describeError(code), [code]);
  const diagnostics = useMemo(
    () =>
      JSON.stringify(
        {
          code: code ?? 'unknown',
          request_id: requestId ?? null,
          status: status ?? null,
          details: details ?? null,
          generated_at: new Date().toISOString(),
        },
        null,
        2,
      ),
    [code, requestId, status, details],
  );

  const copyDiagnostics = async () => {
    try {
      await navigator.clipboard.writeText(diagnostics);
    } catch {
      // ignore clipboard errors
    }
  };

  return (
    <div className="rounded-lg border border-rose-400/40 bg-card p-5 text-sm">
      <div className="font-semibold text-rose-200">{descriptor.title}</div>
      <div className="mt-2 text-muted-foreground">{descriptor.hint}</div>
      <div className="mt-3 text-xs text-muted-foreground">
        Код: {code ?? 'не задан'} · Request ID: {requestId ?? 'не указан'}
      </div>
      <div className="mt-3">
        <Button variant="secondary" size="sm" onClick={copyDiagnostics}>
          Скопировать диагностику (редактированную)
        </Button>
      </div>
    </div>
  );
}
