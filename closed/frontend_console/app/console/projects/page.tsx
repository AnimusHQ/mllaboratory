import { RoleBindingsTable } from '@/components/console/role-bindings-table';
import { ErrorState } from '@/components/console/error-state';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { PageHeader, PageShell } from '@/components/ui/page-shell';
import { GatewayAPIError } from '@/lib/gateway-client';
import type { components } from '@/lib/gateway-openapi';
import { getActiveProjectId } from '@/lib/server-context';
import { gatewayServerFetchJSON } from '@/lib/server-gateway';

export default async function ProjectsPage() {
  const projectId = getActiveProjectId();
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
  }

  return (
    <PageShell>
      <PageHeader
        title="Проекты"
        description="Контекст доступа, управление ролями и архивирование. Используется проектный контекст из верхней панели."
        actions={
          <Button variant="secondary" size="sm">
            Создать проект
          </Button>
        }
      />
      {!projectId ? (
        <Card>
          <CardHeader>
            <CardTitle>Контекст проекта не задан</CardTitle>
            <CardDescription>
              Укажите project_id в верхней панели для просмотра ролей и управления доступом.
            </CardDescription>
          </CardHeader>
        </Card>
      ) : null}
      {error ? (
        <ErrorState code={error.code} requestId={error.requestId} status={error.status} details={error.details} />
      ) : null}
      {projectId ? (
        <Card>
          <CardHeader>
            <CardTitle>Ролевые привязки</CardTitle>
            <CardDescription>RBAC управляется через Gateway API. Все изменения аудируются.</CardDescription>
          </CardHeader>
          <CardContent>
            {bindings ? <RoleBindingsTable bindings={bindings.bindings ?? []} /> : <p className="text-sm">Загрузка…</p>}
          </CardContent>
        </Card>
      ) : null}
    </PageShell>
  );
}
