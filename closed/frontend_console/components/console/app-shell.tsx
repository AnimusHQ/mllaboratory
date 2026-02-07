'use client';

import Link from 'next/link';
import { usePathname, useRouter, useSearchParams } from 'next/navigation';
import type { ReactNode } from 'react';
import { useEffect, useMemo, useState } from 'react';

import { Breadcrumbs } from '@/components/console/breadcrumbs';
import { OperationsPanel } from '@/components/console/operations-panel';
import { PolicyHint } from '@/components/console/policy-hint';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { requiresAuthGate } from '@/lib/auth/auth-gate';
import { getGatewayLoginUrl } from '@/lib/auth/login-url';
import { OperationsProvider } from '@/lib/operations';
import { cn } from '@/lib/utils';
import { ProjectProvider, useProjectContext } from '@/lib/project-context';
import { can, deriveEffectiveRole, roleLabel, type Capability } from '@/lib/rbac';
import type { GatewaySession } from '@/lib/session';

export type NavItem = {
  label: string;
  href: string;
  description?: string;
};

export type NavSection = {
  id: string;
  label: string;
  items: NavItem[];
};

const navSections: NavSection[] = [
  {
    id: 'primary',
    label: 'Контуры управления',
    items: [
      { label: 'Проекты', href: '/console/projects', description: 'Контекст, роли, архив' },
      { label: 'Наборы данных', href: '/console/datasets', description: 'Версии и качество' },
      { label: 'Артефакты', href: '/console/artifacts', description: 'Хранилище и загрузки' },
      { label: 'Запуски', href: '/console/runs', description: 'Очереди, ретраи, состояния' },
      { label: 'Пайплайны', href: '/console/pipelines', description: 'DAG, узлы, исполнение' },
      { label: 'Среды', href: '/console/environments', description: 'Шаблоны и блокировки' },
      { label: 'DevEnv (IDE)', href: '/console/devenvs', description: 'Сессии и TTL' },
      { label: 'Модели', href: '/console/models', description: 'Версии и экспорт' },
      { label: 'Lineage', href: '/console/lineage', description: 'Графы происхождения' },
      { label: 'Аудит / SIEM', href: '/console/audit', description: 'Доставки и DLQ' },
      { label: 'Ops', href: '/console/ops', description: 'Готовность и метрики' },
    ],
  },
];

const quickActions: Array<{ label: string; href: string; capability: Capability }> = [
  { label: 'Новый Run', href: '/console/runs/new', capability: 'run:write' },
  { label: 'Новый PipelineRun', href: '/console/pipelines/new', capability: 'run:write' },
  { label: 'Новый EnvLock', href: '/console/environments/new-lock', capability: 'env:write' },
  { label: 'Новый DevEnv', href: '/console/devenvs/new', capability: 'devenv:write' },
  { label: 'Новая версия модели', href: '/console/models/new-version', capability: 'model:write' },
];

const isActive = (href: string, pathname: string | null) => {
  if (!pathname) {
    return false;
  }
  return pathname === href || pathname.startsWith(`${href}/`);
};

function TopBar({ session }: { session: GatewaySession }) {
  const router = useRouter();
  const { projectId, setProjectId } = useProjectContext();
  const [showQuick, setShowQuick] = useState(false);
  const effectiveRole = useMemo(() => deriveEffectiveRole(session.mode === 'authenticated' ? session.roles : []), [session]);
  const [goMode, setGoMode] = useState(false);

  useEffect(() => {
    const handler = (event: KeyboardEvent) => {
      const target = event.target as HTMLElement | null;
      const tag = target?.tagName?.toLowerCase();
      if (tag === 'input' || tag === 'textarea' || target?.getAttribute('contenteditable') === 'true') {
        return;
      }
      if (event.key === '/') {
        event.preventDefault();
        const input = document.getElementById('console-search') as HTMLInputElement | null;
        input?.focus();
        return;
      }
      if (event.key.toLowerCase() === 'g') {
        setGoMode(true);
        return;
      }
      if (goMode) {
        switch (event.key.toLowerCase()) {
          case 'p':
            router.push('/console/projects');
            break;
          case 'd':
            router.push('/console/datasets');
            break;
          case 'r':
            router.push('/console/runs');
            break;
          case 'm':
            router.push('/console/models');
            break;
          case 'e':
            router.push('/console/environments');
            break;
        }
        setGoMode(false);
      }
    };
    document.addEventListener('keydown', handler);
    return () => document.removeEventListener('keydown', handler);
  }, [router, goMode]);

  return (
    <header className="flex flex-wrap items-center justify-between gap-4 border-b border-border/70 bg-card/40 px-6 py-4">
      <div className="flex flex-1 flex-wrap items-center gap-3">
        <div className="text-sm font-semibold tracking-[0.18em] uppercase">Animus Datalab</div>
        <Badge variant={session.mode === 'authenticated' ? 'info' : 'warning'}>
          {session.mode === 'authenticated' ? 'Сессия активна' : 'Требуется вход'}
        </Badge>
        <div className="text-xs text-muted-foreground">Контур: Control Plane</div>
      </div>
      <div className="flex flex-1 flex-wrap items-center justify-end gap-3">
        <Input
          id="console-search"
          placeholder="Поиск по идентификатору, имени, хэшу"
          className="max-w-xs"
        />
        <div className="flex items-center gap-2">
          <span className="text-xs text-muted-foreground">Проект</span>
          <Input
            value={projectId}
            onChange={(event) => setProjectId(event.target.value)}
            placeholder="project_id"
            className="h-8 w-40 text-xs"
          />
        </div>
        <div className="text-xs text-muted-foreground">
          {session.mode === 'authenticated' ? session.subject : 'Не аутентифицирован'}
        </div>
        <Badge variant={effectiveRole === 'admin' ? 'success' : effectiveRole === 'editor' ? 'info' : 'neutral'}>
          {roleLabel(effectiveRole)}
        </Badge>
        <div className="relative">
          <Button variant="secondary" size="sm" onClick={() => setShowQuick((prev) => !prev)}>
            Быстрые действия
          </Button>
          {showQuick ? (
            <div className="absolute right-0 mt-2 w-64 rounded-lg border border-border/70 bg-card p-3 shadow-glow-sm">
              <div className="mb-2 text-xs uppercase tracking-[0.2em] text-muted-foreground">Запуск</div>
              <div className="flex flex-col gap-2">
                {quickActions.map((action) => {
                  const allowed = can(effectiveRole, action.capability);
                  return (
                    <div key={action.href} className="space-y-1">
                      {allowed ? (
                        <Link
                          href={action.href}
                          className="rounded-md border border-border/60 px-3 py-2 text-sm hover:bg-muted/40"
                          onClick={() => setShowQuick(false)}
                        >
                          {action.label}
                        </Link>
                      ) : (
                        <div className="rounded-md border border-border/60 px-3 py-2 text-sm text-muted-foreground">
                          {action.label}
                        </div>
                      )}
                      <PolicyHint allowed={allowed} capability={action.capability} />
                    </div>
                  );
                })}
              </div>
            </div>
          ) : null}
        </div>
      </div>
    </header>
  );
}

function AuthRequired({ loginUrl }: { loginUrl: string }) {
  return (
    <div className="min-h-screen bg-background text-foreground">
      <div className="flex min-h-screen items-center justify-center px-6">
        <div className="console-surface w-full max-w-lg p-8">
          <div className="console-section-title">Доступ к консоли</div>
          <h1 className="mt-3 text-2xl font-semibold">Авторизация требуется</h1>
          <p className="mt-3 text-sm text-muted-foreground">
            Для работы в контрольной плоскости требуется активная сессия Gateway. Выполните вход и вернитесь к
            запрошенному разделу.
          </p>
          <div className="mt-6">
            <Link href={loginUrl} className="inline-flex items-center rounded-md bg-primary px-4 py-2 text-sm font-semibold text-primary-foreground">
              Войти
            </Link>
          </div>
          <div className="mt-4 text-xs text-muted-foreground">
            Если вход не открывается, проверьте адрес Gateway и сетевой доступ.
          </div>
        </div>
      </div>
    </div>
  );
}

export function AppShell({ session, children }: { session: GatewaySession; children: ReactNode }) {
  const pathname = usePathname();
  const searchParams = useSearchParams();
  const returnTo = useMemo(() => {
    if (!pathname) {
      return '/console';
    }
    const query = searchParams?.toString();
    return query ? `${pathname}?${query}` : pathname;
  }, [pathname, searchParams]);
  const loginUrl = useMemo(() => getGatewayLoginUrl(returnTo), [returnTo]);

  if (requiresAuthGate(session)) {
    return <AuthRequired loginUrl={loginUrl} />;
  }

  return (
    <ProjectProvider>
      <OperationsProvider>
        <div className="min-h-screen bg-background text-foreground">
          <TopBar session={session} />
          <div className="grid min-h-[calc(100vh-72px)] grid-cols-[260px_1fr]">
            <aside className="border-r border-border/70 bg-card/40 p-5">
              <div className="mb-6">
                <div className="console-kicker">Навигация</div>
                <div className="mt-2 text-sm text-muted-foreground">Контрольная плоскость</div>
              </div>
              <nav className="flex flex-col gap-6" aria-label="Основная навигация">
                {navSections.map((section) => (
                  <div key={section.id} className="flex flex-col gap-2">
                    <div className="text-xs uppercase tracking-[0.2em] text-muted-foreground">{section.label}</div>
                    <div className="flex flex-col gap-1">
                      {section.items.map((item) => {
                        const active = isActive(item.href, pathname);
                        return (
                          <Link
                            key={item.href}
                            href={item.href}
                            className={cn(
                              'rounded-md px-3 py-2 text-sm transition',
                              active ? 'bg-muted/50 text-foreground' : 'text-muted-foreground hover:bg-muted/40',
                            )}
                            aria-current={active ? 'page' : undefined}
                          >
                            <div className="font-semibold">{item.label}</div>
                            {item.description ? <div className="text-xs text-muted-foreground">{item.description}</div> : null}
                          </Link>
                        );
                      })}
                    </div>
                  </div>
                ))}
              </nav>
            </aside>
            <main className="px-8 py-6">
              {session.mode === 'error' ? (
                <div className="mb-6 rounded-lg border border-rose-400/40 bg-card p-4 text-sm">
                  <div className="font-semibold text-rose-200">Сбой проверки сессии</div>
                  <div className="mt-2 text-muted-foreground">
                    Консоль не может подтвердить текущую сессию. Повторите запрос через несколько секунд.
                  </div>
                  <div className="mt-2 text-xs text-muted-foreground">Код: {session.error}</div>
                </div>
              ) : null}
              <div className="mb-4">
                <Breadcrumbs />
              </div>
              <div className="mb-6">
                <OperationsPanel />
              </div>
              {children}
            </main>
          </div>
        </div>
      </OperationsProvider>
    </ProjectProvider>
  );
}
