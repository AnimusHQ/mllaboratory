'use client';

import { useMemo, useState } from 'react';
import { useRouter } from 'next/navigation';

import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Table, TableContainer, TableEmpty } from '@/components/ui/table';
import { formatDateTime } from '@/lib/format';
import type { components } from '@/lib/gateway-openapi';
import { useProjectContext } from '@/lib/project-context';
import { ProjectCreateButton } from './project-create-dialog';

type ProjectOnboardingProps = {
  projects: components['schemas']['Project'][];
  roles: string[];
  reason?: string;
};

export function ProjectOnboarding({ projects, roles, reason }: ProjectOnboardingProps) {
  const router = useRouter();
  const { setProjectId } = useProjectContext();
  const [manualProjectId, setManualProjectId] = useState('');

  const hasProjects = projects.length > 0;
  const reasonMessage = useMemo(() => {
    if (reason === 'project_id_required') {
      return 'Активный проект не выбран. Выберите или создайте проект для продолжения работы.';
    }
    return null;
  }, [reason]);

  const selectProject = (projectId: string) => {
    const trimmed = projectId.trim();
    if (!trimmed) {
      return;
    }
    setProjectId(trimmed);
    router.push('/console/runs');
  };

  return (
    <div className="space-y-6">
      {reasonMessage ? (
        <Card className="shadow-glow-sm">
          <CardHeader>
            <CardTitle>Требуется проектный контекст</CardTitle>
            <CardDescription>{reasonMessage}</CardDescription>
          </CardHeader>
        </Card>
      ) : null}

      <Card className="shadow-glow-sm">
        <CardHeader>
          <CardTitle>Доступные проекты</CardTitle>
          <CardDescription>Выберите проект для работы в консоли.</CardDescription>
        </CardHeader>
        <CardContent>
          <TableContainer>
            <Table>
              <thead>
                <tr className="text-left text-xs uppercase tracking-[0.18em] text-muted-foreground">
                  <th className="px-4 py-3">Имя</th>
                  <th className="px-4 py-3">Описание</th>
                  <th className="px-4 py-3">Создан</th>
                  <th className="px-4 py-3">Действие</th>
                </tr>
              </thead>
              <tbody>
                {projects.map((project) => (
                  <tr key={project.project_id} className="border-t border-border/60 text-sm">
                    <td className="px-4 py-3 font-semibold">{project.name}</td>
                    <td className="px-4 py-3 text-muted-foreground">{project.description || '—'}</td>
                    <td className="px-4 py-3 text-xs text-muted-foreground">{formatDateTime(project.created_at)}</td>
                    <td className="px-4 py-3">
                      <Button variant="secondary" size="sm" onClick={() => selectProject(project.project_id)}>
                        Выбрать
                      </Button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </Table>
            {!hasProjects ? (
              <TableEmpty>
                Проекты не найдены. Если доступ выдан администратором, используйте ручной ввод идентификатора.
              </TableEmpty>
            ) : null}
          </TableContainer>
        </CardContent>
      </Card>

      <Card className="shadow-glow-sm">
        <CardHeader>
          <CardTitle>Указать проект вручную</CardTitle>
          <CardDescription>Введите идентификатор проекта, если он предоставлен администратором.</CardDescription>
        </CardHeader>
        <CardContent className="flex flex-col gap-3 sm:flex-row sm:items-center">
          <Input
            value={manualProjectId}
            onChange={(event) => setManualProjectId(event.target.value)}
            placeholder="project_id"
          />
          <Button variant="primary" size="sm" onClick={() => selectProject(manualProjectId)} disabled={!manualProjectId.trim()}>
            Использовать
          </Button>
        </CardContent>
      </Card>

      <Card className="shadow-glow-sm">
        <CardHeader>
          <CardTitle>Создание нового проекта</CardTitle>
          <CardDescription>Доступно только администраторам.</CardDescription>
        </CardHeader>
        <CardContent>
          <ProjectCreateButton roles={roles} label="Создать проект" />
        </CardContent>
      </Card>
    </div>
  );
}
