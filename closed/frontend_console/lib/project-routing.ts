export const PROJECT_SELECTION_PATH = '/console/projects';

export const buildProjectSelectionURL = (reason?: string) => {
  if (!reason) {
    return PROJECT_SELECTION_PATH;
  }
  const params = new URLSearchParams({ reason });
  return `${PROJECT_SELECTION_PATH}?${params.toString()}`;
};

export const shouldRedirectToProjectSelection = (code?: string, pathname?: string | null) => {
  if (!code) {
    return false;
  }
  const normalized = code.toLowerCase();
  if (normalized !== 'project_id_required') {
    return false;
  }
  if (!pathname) {
    return true;
  }
  return !pathname.startsWith(PROJECT_SELECTION_PATH);
};
