import type { HTMLAttributes, ReactNode } from 'react';

import { cn } from '@/lib/utils';

export function PageShell({ className, ...props }: HTMLAttributes<HTMLDivElement>) {
  return <div className={cn('flex flex-col gap-6', className)} {...props} />;
}

export function PageHeader({ title, description, actions }: { title: string; description?: string; actions?: ReactNode }) {
  return (
    <div className="flex flex-wrap items-start justify-between gap-4">
      <div className="max-w-3xl">
        <div className="console-kicker">Операционный контур</div>
        <h1 className="text-2xl font-semibold leading-tight">{title}</h1>
        {description ? <p className="mt-2 text-sm text-white/70">{description}</p> : null}
      </div>
      {actions ? <div className="flex flex-wrap gap-2">{actions}</div> : null}
    </div>
  );
}

export function PageSection({ title, description, children }: { title: string; description?: string; children: ReactNode }) {
  return (
    <section className="flex flex-col gap-4">
      <div>
        <div className="console-section-title">{title}</div>
        {description ? <p className="mt-2 text-sm text-white/70">{description}</p> : null}
      </div>
      {children}
    </section>
  );
}
