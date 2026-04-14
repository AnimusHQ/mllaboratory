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
    <header className="flex flex-wrap items-center justify-between gap-4 border-b border-white/10 bg-[#0b1626]/85 px-6 py-4 backdrop-blur-[2px]">
      <div className="flex flex-1 flex-wrap items-center gap-3">
        <div className="text-sm font-semibold uppercase tracking-[0.18em] text-white">Animus Datalab</div>
        <Badge variant={session.mode === 'authenticated' ? 'info' : 'warning'}>
          {session.mode === 'authenticated' ? 'Сессия активна' : 'Требуется вход'}
        </Badge>
        <div className="text-xs text-white/60">Контур: Control Plane</div>
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
        <div className="text-xs text-white/60">
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
            <div className="absolute right-0 mt-2 w-64 rounded-[20px] border border-white/12 bg-[#0b1626]/95 p-3 shadow-[0_18px_36px_rgba(3,10,18,0.6)] backdrop-blur-[2px]">
              <div className="mb-2 text-[11px] uppercase tracking-[0.3em] text-white/60">Запуск</div>
              <div className="flex flex-col gap-2">
                {quickActions.map((action) => {
                  const allowed = can(effectiveRole, action.capability);
                  return (
                    <div key={action.href} className="space-y-1">
                      {allowed ? (
                        <Link
                          href={action.href}
                          className="rounded-xl border border-white/12 bg-white/5 px-3 py-2 text-sm text-white/80 hover:bg-white/10"
                          onClick={() => setShowQuick(false)}
                        >
                          {action.label}
                        </Link>
                      ) : (
                        <div className="rounded-xl border border-white/12 bg-white/5 px-3 py-2 text-sm text-white/40">
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
            <Link
              href={loginUrl}
              className="inline-flex items-center justify-center whitespace-nowrap rounded-xl bg-accent/80 px-4 py-2 text-sm font-medium text-accent-foreground transition-all hover:bg-accent/90"
            >
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
            <aside className="border-r border-white/10 bg-[#0b1626]/85 p-5">
              <div className="mb-6">
                <div className="console-kicker">Навигация</div>
                <div className="mt-2 text-sm text-white/70">Контрольная плоскость</div>
              </div>
              <nav className="flex flex-col gap-6" aria-label="Основная навигация">
                {navSections.map((section) => (
                  <div key={section.id} className="flex flex-col gap-2">
                    <div className="text-[11px] uppercase tracking-[0.3em] text-white/60">{section.label}</div>
                    <div className="flex flex-col gap-1">
                      {section.items.map((item) => {
                        const active = isActive(item.href, pathname);
                        return (
                          <Link
                            key={item.href}
                            href={item.href}
                            className={cn(
                              'rounded-xl px-3 py-2 text-sm transition',
                              active ? 'bg-white/10 text-white' : 'text-white/70 hover:bg-white/5',
                            )}
                            aria-current={active ? 'page' : undefined}
                          >
                            <div className="font-semibold">{item.label}</div>
                            {item.description ? <div className="text-xs text-white/50">{item.description}</div> : null}
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
                <div className="mb-6 rounded-[24px] border border-rose-400/40 bg-[#0b1626]/85 p-4 text-sm shadow-[0_18px_36px_rgba(3,10,18,0.6)]">
                  <div className="font-semibold text-rose-200">Сбой проверки сессии</div>
                  <div className="mt-2 text-white/70">
                    Консоль не может подтвердить текущую сессию. Повторите запрос через несколько секунд.
                  </div>
                  <div className="mt-2 text-xs text-white/60">Код: {session.error}</div>
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
