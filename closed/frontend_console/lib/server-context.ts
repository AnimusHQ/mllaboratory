import { cookies } from 'next/headers';

const PROJECT_COOKIE_KEY = 'animus_project_id';

export async function getActiveProjectId(): Promise<string> {
  const cookieStore = await cookies();
  return cookieStore.get(PROJECT_COOKIE_KEY)?.value?.trim() ?? '';
}
