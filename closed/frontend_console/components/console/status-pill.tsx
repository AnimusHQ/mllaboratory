'use client';

import { cn } from '@/lib/utils';

const statusStyles: Record<string, string> = {
  pending: 'border-amber-300/50 text-amber-200',
  running: 'border-sky-300/50 text-sky-200',
  succeeded: 'border-emerald-300/50 text-emerald-200',
  failed: 'border-rose-300/50 text-rose-200',
  canceled: 'border-slate-300/50 text-slate-200',
  draft: 'border-slate-300/50 text-slate-200',
  validated: 'border-sky-300/50 text-sky-200',
  approved: 'border-emerald-300/50 text-emerald-200',
  deprecated: 'border-amber-300/50 text-amber-200',
};

export function StatusPill({ status, label }: { status?: string | null; label?: string }) {
  const normalized = status?.toLowerCase() ?? 'unknown';
  return (
    <span className={cn('console-pill', statusStyles[normalized] ?? 'border-white/15 text-white/70')}>
      {label ?? status ?? 'не задано'}
    </span>
  );
}
