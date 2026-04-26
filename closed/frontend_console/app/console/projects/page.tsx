import { RoleBindingsTable } from '@/components/console/role-bindings-table';
import { ErrorState } from '@/components/console/error-state';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { PageHeader, PageShell, PageSection } from '@/components/ui/page-shell';
import { GatewayAPIError } from '@/lib/gateway-client';
import type { components } from '@/lib/gateway-openapi';
import { getActiveProjectId } from '@/lib/server-context';
import { gatewayServerFetchJSON } from '@/lib/server-gateway';
import { getGatewaySession } from '@/lib/session';

import { ProjectOnboarding } from './project-onboarding';
import { ProjectPageActions } from './project-page-actions';

type SearchParams = {
  reason?: string;
};

export default async function ProjectsPage({ searchParams }: { searchParams?: Promise<SearchParams> }) {
  const projectId = await getActiveProjectId();
  const params = (await searchParams) ?? {};
  const reason = params.reason?.trim();
  const session = await getGatewaySession();
  const roles = session.mode === 'authenticated' ? session.roles : [];
  let projects: components['schemas']['Project'][] = [];
  let bindings: components['schemas']['RoleBindingListResponse'] | null = null;
  let error: GatewayAPIError | null = null;

  if (projectId) {
    try {
      bindings = await gatewayServerFetchJSON<components['schemas']['RoleBindingListResponse']>(
        `/api/experiments/projects/${projectId}/role-bindings?limit=200`,
      );
    } catch (err) {
      if (err instanceof GatewayAPIError) {
        error = err;
      } else {
        error = new GatewayAPIError(500, 'gateway_unexpected');
      }
    }
  } else {
    try {
      const response = await gatewayServerFetchJSON<components['schemas']['ProjectListResponse']>(
        '/api/dataset-registry/projects?limit=200',
      );
      projects = response.projects ?? [];
    } catch (err) {
      if (err instanceof GatewayAPIError) {
        error = err;
      } else {
        error = new GatewayAPIError(500, 'gateway_unexpected');
      }
    }
  }

  return (
    <PageShell>
      <PageHeader
        title={projectId ? 'Проекты' : 'Выбор проекта'}
        description={
          projectId
            ? 'Контекст доступа, управление ролями и архивирование. Используется проектный контекст из верхней панели.'
            : 'Активный проект требуется для всех операций. Выберите существующий или создайте новый.'
        }
        actions={<ProjectPageActions roles={roles} />}
      />
      {error ? <ErrorState code={error.code} requestId={error.requestId} status={error.status} details={error.details} message={error.message} retryable={error.retryable} /> : null}
      {!projectId ? (
        <ProjectOnboarding projects={projects} roles={roles} reason={reason} />
      ) : (
        <>
          <PageSection title="Ролевые привязки" description="RBAC управляется через Gateway API. Все изменения аудируются.">
            <Card>
              <CardContent className="pt-6">
                {bindings ? <RoleBindingsTable bindings={bindings.bindings ?? []} /> : <p className="text-sm">Загрузка…</p>}
              </CardContent>
            </Card>
          </PageSection>
          <Card>
            <CardHeader>
              <CardTitle>Архивирование и жизненный цикл</CardTitle>
              <CardDescription>
                Создание и архивирование проектов выполняются через административные контуры. В этой консоли доступны только
                роль и контекст.
              </CardDescription>
            </CardHeader>
            <CardContent>
              <p className="text-sm text-muted-foreground">
                Для изменения жизненного цикла проекта используйте административные процедуры и зафиксируйте операции в аудите.
              </p>
            </CardContent>
          </Card>
        </>
      )}
    </PageShell>
  );
}
