import type { HTMLAttributes, TableHTMLAttributes } from 'react';

import { cn } from '@/lib/utils';

export function Table({ className, ...props }: TableHTMLAttributes<HTMLTableElement>) {
  return <table className={cn('console-table', className)} {...props} />;
}

export function TableContainer({ className, ...props }: HTMLAttributes<HTMLDivElement>) {
  return (
    <div
      className={cn(
        'overflow-auto rounded-[24px] border border-white/12 bg-[#0b1626]/90 shadow-[0_18px_36px_rgba(3,10,18,0.6)]',
        className,
      )}
      {...props}
    />
  );
}

export function TableEmpty({ className, ...props }: HTMLAttributes<HTMLDivElement>) {
  return (
    <div
      className={cn('flex items-center justify-center px-6 py-10 text-sm text-white/60', className)}
      {...props}
    />
  );
}
