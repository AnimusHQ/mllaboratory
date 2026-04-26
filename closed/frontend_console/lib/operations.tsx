'use client';

import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState } from 'react';

import { gatewayFetchJSON } from '@/lib/gateway-client';

export type OperationStatus = 'pending' | 'succeeded' | 'failed';

export type OperationPoll =
  | {
      kind: 'run';
      projectId: string;
      runId: string;
      intervalMs?: number;
    }
  | {
      kind: 'devenv';
      projectId: string;
      devEnvId: string;
      intervalMs?: number;
    };

export type OperationRetry =
  | {
      kind: 'dispatch-run';
      projectId: string;
      runId: string;
    }
  | {
      kind: 'open-devenv-session';
      projectId: string;
      devEnvId: string;
    };

export type ConsoleOperation = {
  id: string;
  label: string;
  status: OperationStatus;
  createdAt: string;
  updatedAt?: string;
  details?: string;
  poll?: OperationPoll;
  retry?: OperationRetry;
};

type OperationsContextValue = {
  operations: ConsoleOperation[];
  addOperation: (operation: ConsoleOperation) => void;
  updateOperation: (id: string, status: OperationStatus, details?: string, patch?: Partial<ConsoleOperation>) => void;
  clearCompleted: () => void;
  retryOperation: (operation: ConsoleOperation) => Promise<void>;
};

const STORAGE_KEY = 'animus_console_operations_v1';

const OperationsContext = createContext<OperationsContextValue | null>(null);

const loadOperations = (): ConsoleOperation[] => {
  if (typeof window === 'undefined') {
    return [];
  }
  try {
    const raw = window.localStorage.getItem(STORAGE_KEY);
    if (!raw) {
      return [];
    }
    const parsed = JSON.parse(raw) as ConsoleOperation[];
    return Array.isArray(parsed) ? parsed : [];
  } catch {
    return [];
  }
};

const persistOperations = (operations: ConsoleOperation[]) => {
  if (typeof window === 'undefined') {
    return;
  }
  try {
    window.localStorage.setItem(STORAGE_KEY, JSON.stringify(operations));
  } catch {
    // ignore persistence failures
  }
};

export function OperationsProvider({ children }: { children: React.ReactNode }) {
  const [operations, setOperations] = useState<ConsoleOperation[]>([]);
  const pollingRef = useRef(false);

  useEffect(() => {
    setOperations(loadOperations());
  }, []);

  useEffect(() => {
    persistOperations(operations);
  }, [operations]);

  const addOperation = useCallback((operation: ConsoleOperation) => {
    setOperations((prev) => {
      const next = [operation, ...prev].slice(0, 50);
      return next;
    });
  }, []);

  const updateOperation = useCallback(
    (id: string, status: OperationStatus, details?: string, patch?: Partial<ConsoleOperation>) => {
    setOperations((prev) =>
      prev.map((operation) =>
        operation.id === id
          ? {
              ...operation,
              status,
              details: details ?? operation.details,
              updatedAt: new Date().toISOString(),
              ...(patch ?? {}),
            }
          : operation,
      ),
    );
    },
    [],
  );

  const clearCompleted = useCallback(() => {
    setOperations((prev) => prev.filter((op) => op.status === 'pending'));
  }, []);

  const pollOperation = useCallback(
    async (operation: ConsoleOperation) => {
      if (!operation.poll) {
        return;
      }
      try {
        switch (operation.poll.kind) {
          case 'run': {
            const response = await gatewayFetchJSON<{ status?: string }>(
              `/api/experiments/projects/${operation.poll.projectId}/runs/${operation.poll.runId}`,
              { method: 'GET', retry: true },
            );
            const status = response.status?.toLowerCase();
            if (status === 'succeeded') {
              updateOperation(operation.id, 'succeeded', operation.details);
            } else if (status === 'failed' || status === 'canceled') {
              updateOperation(operation.id, 'failed', operation.details);
            }
            break;
          }
          case 'devenv': {
            const response = await gatewayFetchJSON<{ state?: string }>(
              `/api/experiments/projects/${operation.poll.projectId}/devenvs/${operation.poll.devEnvId}`,
              { method: 'GET', retry: true },
            );
            const state = response.state?.toLowerCase();
            if (state === 'active') {
              updateOperation(operation.id, 'succeeded', operation.details);
            } else if (state === 'failed' || state === 'expired' || state === 'deleted') {
              updateOperation(operation.id, 'failed', operation.details);
            }
            break;
          }
        }
      } catch {
        // polling failures are non-fatal; retain pending status
      }
    },
    [updateOperation],
  );

  useEffect(() => {
    const interval = setInterval(() => {
      if (pollingRef.current) {
        return;
      }
      const pending = operations.filter((op) => op.status === 'pending' && op.poll);
      if (pending.length === 0) {
        return;
      }
      pollingRef.current = true;
      Promise.all(pending.map((op) => pollOperation(op))).finally(() => {
        pollingRef.current = false;
      });
    }, 8000);
    return () => clearInterval(interval);
  }, [operations, pollOperation]);

  const retryOperation = useCallback(
    async (operation: ConsoleOperation) => {
      if (!operation.retry) {
        return;
      }
      const retryId = `${operation.id}-retry-${Date.now()}`;
      const base: ConsoleOperation = {
        ...operation,
        id: retryId,
        status: 'pending',
        createdAt: new Date().toISOString(),
        updatedAt: undefined,
      };
      addOperation(base);
      try {
        switch (operation.retry.kind) {
          case 'dispatch-run': {
            await gatewayFetchJSON(
              `/api/experiments/projects/${operation.retry.projectId}/runs/${operation.retry.runId}:dispatch`,
              {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({}),
              },
            );
            updateOperation(retryId, 'succeeded', operation.details);
            break;
          }
          case 'open-devenv-session': {
            await gatewayFetchJSON(
              `/api/experiments/projects/${operation.retry.projectId}/devenvs/${operation.retry.devEnvId}:open-ide-session`,
              { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ ttlSeconds: 3600 }) },
            );
            updateOperation(retryId, 'succeeded', operation.details);
            break;
          }
        }
      } catch {
        updateOperation(retryId, 'failed', operation.details);
      }
    },
    [addOperation, updateOperation],
  );

  const value = useMemo(
    () => ({
      operations,
      addOperation,
      updateOperation,
      clearCompleted,
      retryOperation,
    }),
    [operations, addOperation, updateOperation, clearCompleted, retryOperation],
  );

  return <OperationsContext.Provider value={value}>{children}</OperationsContext.Provider>;
}

export function useOperations() {
  const ctx = useContext(OperationsContext);
  if (!ctx) {
    throw new Error('useOperations must be used within OperationsProvider');
  }
  return ctx;
}
