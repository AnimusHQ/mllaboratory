import { cookies } from 'next/headers';

const PROJECT_COOKIE_KEY = 'animus_project_id';

export function getActiveProjectId(): string {
  const cookieStore = cookies();
  return cookieStore.get(PROJECT_COOKIE_KEY)?.value?.trim() ?? '';
}
