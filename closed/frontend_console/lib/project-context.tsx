'use client';

import type { ReactNode } from 'react';
import { createContext, useCallback, useContext, useEffect, useState } from 'react';

const PROJECT_STORAGE_KEY = 'animus.console.project';
const PROJECT_COOKIE_KEY = 'animus_project_id';

export const readProjectIdFromStorage = (storedValue: string | null, cookie: string) => {
  const stored = (storedValue ?? '').trim();
  if (stored) {
    return stored;
  }
  const cookieValue = cookie
    .split(';')
    .map((entry) => entry.trim())
    .find((entry) => entry.startsWith(`${PROJECT_COOKIE_KEY}=`));
  if (!cookieValue) {
    return '';
  }
  const value = decodeURIComponent(cookieValue.split('=')[1] ?? '');
  return value.trim();
};

export const buildProjectCookie = (value: string) => {
  const trimmed = value.trim();
  if (!trimmed) {
    return `${PROJECT_COOKIE_KEY}=; path=/; max-age=0; samesite=lax`;
  }
  return `${PROJECT_COOKIE_KEY}=${encodeURIComponent(trimmed)}; path=/; max-age=31536000; samesite=lax`;
};

export const persistProjectId = (storage: Pick<Storage, 'setItem' | 'removeItem'>, value: string) => {
  const trimmed = value.trim();
  if (trimmed) {
    storage.setItem(PROJECT_STORAGE_KEY, trimmed);
  } else {
    storage.removeItem(PROJECT_STORAGE_KEY);
  }
  return buildProjectCookie(trimmed);
};

type ProjectContextValue = {
  projectId: string;
  setProjectId: (value: string) => void;
};

const ProjectContext = createContext<ProjectContextValue | undefined>(undefined);

export function ProjectProvider({ children }: { children: ReactNode }) {
  const [projectId, setProjectIdState] = useState('');

  useEffect(() => {
    const stored = window.localStorage.getItem(PROJECT_STORAGE_KEY);
    const value = readProjectIdFromStorage(stored, document.cookie);
    if (value) {
      setProjectIdState(value);
    }
  }, []);

  const setProjectId = useCallback((value: string) => {
    const trimmed = value.trim();
    setProjectIdState(trimmed);
    document.cookie = persistProjectId(window.localStorage, trimmed);
  }, []);

  return <ProjectContext.Provider value={{ projectId, setProjectId }}>{children}</ProjectContext.Provider>;
}

export function useProjectContext() {
  const context = useContext(ProjectContext);
  if (!context) {
    throw new Error('ProjectContext is not configured');
  }
  return context;
}
