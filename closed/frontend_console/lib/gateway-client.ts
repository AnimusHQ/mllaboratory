import type { components, paths } from '@/lib/gateway-openapi';

export type ErrorResponse = components['schemas']['ErrorResponse'];

export class GatewayAPIError extends Error {
  status: number;
  code: string;
  requestId?: string;
  details?: unknown;
  retryable?: boolean;

  constructor(status: number, code: string, message?: string, requestId?: string, details?: unknown, retryable?: boolean) {
    super(message ?? code);
    this.status = status;
    this.code = code;
    this.retryable = retryable;
    this.requestId = requestId;
    this.details = details;
  }
}

export type GatewayFetchOptions = RequestInit & {
  retry?: boolean;
  requestId?: string;
};

export const gatewayBaseURL = (): string => {
  const envBase = process.env.NEXT_PUBLIC_GATEWAY_URL?.trim();
  if (envBase) {
    return envBase.replace(/\/$/, '');
  }
  return '';
};

const PROJECT_COOKIE_KEY = 'animus_project_id';

const readProjectIdFromBrowser = (): string => {
  if (typeof document === 'undefined') {
    return '';
  }
  const cookieValue = document.cookie
    .split(';')
    .map((entry) => entry.trim())
    .find((entry) => entry.startsWith(`${PROJECT_COOKIE_KEY}=`));
  if (!cookieValue) {
    return '';
  }
  const value = decodeURIComponent(cookieValue.split('=')[1] ?? '').trim();
  return value;
};

const isSafeMethod = (method?: string) => {
  if (!method) {
    return true;
  }
  return ['GET', 'HEAD'].includes(method.toUpperCase());
};

const isRetryableStatus = (status: number) => status === 408 || status === 429 || (status >= 500 && status < 600);

const sleep = (ms: number) => new Promise((resolve) => setTimeout(resolve, ms));

export async function gatewayFetch(path: string, init: GatewayFetchOptions = {}): Promise<Response> {
  const base = gatewayBaseURL();
  const url = base ? `${base}${path}` : path;
  const headers = new Headers(init.headers ?? {});
  const requestId =
    init.requestId ??
    (typeof crypto !== 'undefined' && 'randomUUID' in crypto ? crypto.randomUUID() : `req-${Date.now()}`);
  if (requestId) {
    headers.set('X-Request-Id', requestId);
  }
  if (!headers.has('X-Project-Id') && !headers.has('X-Project-ID')) {
    const projectId = readProjectIdFromBrowser();
    if (projectId) {
      headers.set('X-Project-Id', projectId);
    }
  }

  const attempt = async (): Promise<Response> =>
    fetch(url, {
      ...init,
      headers,
    });

  const res = await attempt();
  if (!res.ok && init.retry !== false && isSafeMethod(init.method) && isRetryableStatus(res.status)) {
    await sleep(350);
    return attempt();
  }
  return res;
}

export async function gatewayFetchJSON<T>(path: string, init: GatewayFetchOptions = {}): Promise<T> {
  const requestId =
    init.requestId ??
    (typeof crypto !== 'undefined' && 'randomUUID' in crypto ? crypto.randomUUID() : `req-${Date.now()}`);
  const res = await gatewayFetch(path, { ...init, requestId });
  if (res.status === 204) {
    return undefined as T;
  }
  const responseRequestId = res.headers.get('X-Request-Id') ?? undefined;
  const contentType = res.headers.get('Content-Type') ?? '';
  if (res.ok) {
    if (!contentType.includes('application/json')) {
      return undefined as T;
    }
    return (await res.json()) as T;
  }

  let parsed: unknown = undefined;
  if (contentType.includes('application/json')) {
    try {
      parsed = await res.json();
    } catch {
      parsed = undefined;
    }
  }
  const errorPayload = (parsed ?? {}) as Record<string, unknown>;
  const code = (errorPayload?.error as string) ?? (errorPayload?.code as string) ?? 'gateway_error';
  const message =
    (errorPayload?.message as string) ??
    (errorPayload?.detail as string) ??
    (typeof errorPayload === 'string' ? errorPayload : undefined);
  const requestIdFinal = (errorPayload?.request_id as string) ?? responseRequestId ?? requestId;
  const retryable =
    (typeof errorPayload?.retryable === 'boolean' ? (errorPayload.retryable as boolean) : undefined) ??
    isRetryableStatus(res.status);
  throw new GatewayAPIError(res.status, code, message, requestIdFinal, parsed, retryable);
}

export type GatewayPaths = paths;
