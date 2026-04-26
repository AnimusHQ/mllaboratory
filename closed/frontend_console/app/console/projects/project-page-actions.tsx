'use client';

import { ProjectCreateButton } from './project-create-dialog';

export function ProjectPageActions({ roles }: { roles: string[] }) {
  return <ProjectCreateButton roles={roles} />;
}
