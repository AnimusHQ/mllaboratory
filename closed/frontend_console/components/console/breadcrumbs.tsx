'use client';

import Link from 'next/link';
import { usePathname } from 'next/navigation';

const labelMap: Record<string, string> = {
  console: 'Консоль',
  projects: 'Проекты',
  datasets: 'Наборы данных',
  artifacts: 'Артефакты',
  runs: 'Запуски',
  pipelines: 'Пайплайны',
  environments: 'Среды',
  devenvs: 'DevEnv',
  models: 'Модели',
  lineage: 'Lineage',
  audit: 'Аудит / SIEM',
  ops: 'Ops',
  'new-lock': 'Новый EnvLock',
  'new-version': 'Новая версия модели',
  new: 'Новый',
};

export function Breadcrumbs() {
  const pathname = usePathname();
  if (!pathname?.startsWith('/console')) {
    return null;
  }

  const segments = pathname.split('/').filter(Boolean);
  const crumbs = segments.map((segment, index) => {
    const href = `/${segments.slice(0, index + 1).join('/')}`;
    const label = labelMap[segment] ?? segment;
    return { href, label, isLast: index === segments.length - 1 };
  });

  return (
    <nav aria-label="Breadcrumb" className="flex flex-wrap items-center gap-2 text-xs text-white/60">
      {crumbs.map((crumb, index) => (
        <span key={`${crumb.href}-${index}`} className="flex items-center gap-2">
          {crumb.isLast ? (
            <span className="text-white">{crumb.label}</span>
          ) : (
            <Link href={crumb.href} className="hover:text-white">
              {crumb.label}
            </Link>
          )}
          {crumb.isLast ? null : <span className="text-white/40">/</span>}
        </span>
      ))}
    </nav>
  );
}
