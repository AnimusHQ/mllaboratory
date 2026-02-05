'use client';

import type { ReactNode } from 'react';
import { createContext, useCallback, useContext, useEffect, useState } from 'react';

const PROJECT_STORAGE_KEY = 'animus.console.project';
const PROJECT_COOKIE_KEY = 'animus_project_id';

type ProjectContextValue = {
  projectId: string;
  setProjectId: (value: string) => void;
};

const ProjectContext = createContext<ProjectContextValue | undefined>(undefined);

export function ProjectProvider({ children }: { children: ReactNode }) {
  const [projectId, setProjectIdState] = useState('');

  useEffect(() => {
    const stored = window.localStorage.getItem(PROJECT_STORAGE_KEY) ?? '';
    if (stored) {
      setProjectIdState(stored);
      return;
    }
    const cookieValue = document.cookie
      .split(';')
      .map((entry) => entry.trim())
      .find((entry) => entry.startsWith(`${PROJECT_COOKIE_KEY}=`));
    if (cookieValue) {
      const value = decodeURIComponent(cookieValue.split('=')[1] ?? '');
      if (value) {\n        setProjectIdState(value);\n      }\n    }
  }, []);

  const setProjectId = useCallback((value: string) => {
    const trimmed = value.trim();
    setProjectIdState(trimmed);
    if (trimmed) {
      window.localStorage.setItem(PROJECT_STORAGE_KEY, trimmed);
      document.cookie = `${PROJECT_COOKIE_KEY}=${encodeURIComponent(trimmed)}; path=/; max-age=31536000; samesite=lax`;
    } else {
      window.localStorage.removeItem(PROJECT_STORAGE_KEY);
      document.cookie = `${PROJECT_COOKIE_KEY}=; path=/; max-age=0; samesite=lax`;
    }
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
