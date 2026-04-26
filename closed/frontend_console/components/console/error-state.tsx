'use client';

import { useEffect, useMemo } from 'react';
import { usePathname, useRouter } from 'next/navigation';

import { Button } from '@/components/ui/button';
import { describeError } from '@/lib/error-messages';
import { buildProjectSelectionURL, shouldRedirectToProjectSelection } from '@/lib/project-routing';
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
  const router = useRouter();
  const pathname = usePathname();
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

  useEffect(() => {
    if (shouldRedirectToProjectSelection(code, pathname)) {
      router.push(buildProjectSelectionURL('project_id_required'));
    }
  }, [code, pathname, router]);

  const copyDiagnostics = async () => {
    try {
      await navigator.clipboard.writeText(diagnostics);
    } catch {
      // ignore clipboard errors
    }
  };

  return (
    <div className="rounded-[24px] border border-rose-400/40 bg-[#0b1626]/85 p-5 text-sm shadow-[0_18px_36px_rgba(3,10,18,0.6)]">
      <div className="font-semibold text-rose-200">{descriptor.title}</div>
      <div className="mt-2 text-white/70">{descriptor.hint}</div>
      {effectiveMessage ? <div className="mt-2 text-xs text-white/60">Сообщение: {effectiveMessage}</div> : null}
      <div className="mt-3 text-xs text-white/60">
        Код: {code ?? 'не задан'} · Request ID: {requestId ?? 'не указан'}
      </div>
      {retryable ? <div className="mt-1 text-xs text-white/60">Категория: повторяемая ошибка</div> : null}
      <div className="mt-3">
        <Button variant="secondary" size="sm" onClick={copyDiagnostics}>
          Скопировать диагностику (редактированную)
        </Button>
      </div>
    </div>
  );
}
