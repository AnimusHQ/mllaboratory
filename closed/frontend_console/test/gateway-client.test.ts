import { strict as assert } from 'node:assert';
import { afterEach, test } from 'node:test';

import { GatewayAPIError, gatewayFetchJSON } from '../lib/gateway-client';

const isGatewayError = (err: unknown): err is GatewayAPIError => err instanceof GatewayAPIError;

afterEach(() => {
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  (globalThis as any).fetch = undefined;
});

test('gatewayFetchJSON parses successful JSON response', async () => {
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  (globalThis as any).fetch = async () =>
    new Response(JSON.stringify({ ok: true }), {
      status: 200,
      headers: { 'Content-Type': 'application/json', 'X-Request-Id': 'req-1' },
    });

  const result = await gatewayFetchJSON<{ ok: boolean }>('/healthz');
  assert.equal(result.ok, true);
});

test('gatewayFetchJSON surfaces ErrorResponse with request_id', async () => {
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  (globalThis as any).fetch = async () =>
    new Response(JSON.stringify({ error: 'VALIDATION_FAILED', request_id: 'req-2' }), {
      status: 400,
      headers: { 'Content-Type': 'application/json' },
    });

  await assert.rejects(
    () => gatewayFetchJSON('/bad'),
    (err: unknown) => isGatewayError(err) && err.code === 'VALIDATION_FAILED' && err.requestId === 'req-2',
  );
});

test('gatewayFetchJSON marks retryable errors and propagates message', async () => {
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  (globalThis as any).fetch = async () =>
    new Response(JSON.stringify({ error: 'UPSTREAM_UNAVAILABLE', detail: 'Downstream timeout' }), {
      status: 503,
      headers: { 'Content-Type': 'application/json' },
    });

  await assert.rejects(
    () => gatewayFetchJSON('/unavailable', { requestId: 'req-503' }),
    (err: unknown) =>
      isGatewayError(err) &&
      err.code === 'UPSTREAM_UNAVAILABLE' &&
      err.message === 'Downstream timeout' &&
      err.requestId === 'req-503' &&
      err.retryable === true,
  );
});

test('gatewayFetchJSON marks non-retryable 4xx', async () => {
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  (globalThis as any).fetch = async () =>
    new Response(JSON.stringify({ error: 'VALIDATION_FAILED', message: 'bad input' }), {
      status: 422,
      headers: { 'Content-Type': 'application/json' },
    });

  await assert.rejects(
    () => gatewayFetchJSON('/bad', { requestId: 'req-422' }),
    (err: unknown) =>
      isGatewayError(err) &&
      err.code === 'VALIDATION_FAILED' &&
      err.message === 'bad input' &&
      err.requestId === 'req-422' &&
      err.retryable === false,
  );
});
