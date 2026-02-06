'use client';

import { useMemo } from 'react';

import { Button } from '@/components/ui/button';
import { describeError } from '@/lib/error-messages';
import { sanitizePayload } from '@/lib/sanitize';

export type ErrorStateProps = {
  code?: string;
  message?: string;
  requestId?: string;
  status?: number;
  details?: unknown;
  retryable?: boolean;
};

export function ErrorState({ code, message, requestId, status, details, retryable }: ErrorStateProps) {
  const descriptor = useMemo(() => describeError(code), [code]);
  const effectiveMessage = useMemo(() => {
    if (message) {
      return message;
    }
    if (details && typeof details === 'object') {
      const record = details as Record<string, unknown>;
      if (typeof record.message === 'string') {
        return record.message;
      }
      if (typeof record.detail === 'string') {
        return record.detail;
      }
    }
    return undefined;
  }, [message, details]);
  const sanitizedDetails = useMemo(() => sanitizePayload(details), [details]);
  const diagnostics = useMemo(
    () =>
      JSON.stringify(
        {
          code: code ?? 'unknown',
          message: effectiveMessage ?? null,
          request_id: requestId ?? null,
          status: status ?? null,
          details: sanitizedDetails ?? null,
          retryable: retryable ?? null,
          generated_at: new Date().toISOString(),
        },
        null,
        2,
      ),
    [code, effectiveMessage, requestId, status, sanitizedDetails, retryable],
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
      {effectiveMessage ? <div className="mt-2 text-xs text-muted-foreground">Сообщение: {effectiveMessage}</div> : null}
      <div className="mt-3 text-xs text-muted-foreground">
        Код: {code ?? 'не задан'} · Request ID: {requestId ?? 'не указан'}
      </div>
      {retryable ? <div className="mt-1 text-xs text-muted-foreground">Категория: повторяемая ошибка</div> : null}
      <div className="mt-3">
        <Button variant="secondary" size="sm" onClick={copyDiagnostics}>
          Скопировать диагностику (редактированную)
        </Button>
      </div>
    </div>
  );
}
