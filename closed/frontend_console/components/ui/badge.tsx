import type { HTMLAttributes } from 'react';

import { cn } from '@/lib/utils';

export type BadgeVariant = 'neutral' | 'success' | 'warning' | 'error' | 'info';

const variantStyles: Record<BadgeVariant, string> = {
  neutral: 'border-white/15 text-white/70',
  success: 'border-emerald-400/40 text-emerald-200',
  warning: 'border-amber-400/40 text-amber-200',
  error: 'border-rose-400/40 text-rose-200',
  info: 'border-sky-400/40 text-sky-200',
};

export function Badge({ className, variant = 'neutral', ...props }: HTMLAttributes<HTMLSpanElement> & { variant?: BadgeVariant }) {
  return (
    <span
      className={cn(
        'inline-flex items-center rounded-full border bg-white/5 px-2.5 py-1 text-[11px] font-semibold uppercase tracking-[0.2em]',
        variantStyles[variant],
        className,
      )}
      {...props}
    />
  );
}
