const REDACT_KEYS = [
  'secret',
  'token',
  'password',
  'authorization',
  'cookie',
  'session',
  'api_key',
  'apikey',
  'access_key',
  'private',
  'credential',
];

const shouldRedactKey = (key: string) => {
  const normalized = key.toLowerCase();
  return REDACT_KEYS.some((needle) => normalized.includes(needle));
};

export function sanitizePayload(value: unknown, depth = 0): unknown {
  if (value === null || value === undefined) {
    return value;
  }
  if (depth > 6) {
    return '[truncated]';
  }
  if (Array.isArray(value)) {
    return value.map((entry) => sanitizePayload(entry, depth + 1));
  }
  if (typeof value === 'object') {
    const record = value as Record<string, unknown>;
    const result: Record<string, unknown> = {};
    for (const [key, entry] of Object.entries(record)) {
      if (shouldRedactKey(key)) {
        result[key] = '[REDACTED]';
      } else {
        result[key] = sanitizePayload(entry, depth + 1);
      }
    }
    return result;
  }
  return value;
}

export function stringifySafe(value: unknown) {
  return JSON.stringify(sanitizePayload(value), null, 2);
}
