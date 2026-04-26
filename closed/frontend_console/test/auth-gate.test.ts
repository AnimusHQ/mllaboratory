import { strict as assert } from 'node:assert';
import { test } from 'node:test';

import { requiresAuthGate } from '../lib/auth/auth-gate';

test('requiresAuthGate returns true for unauthenticated sessions', () => {
  assert.equal(requiresAuthGate({ mode: 'unauthenticated' }), true);
});

test('requiresAuthGate returns false for authenticated sessions', () => {
  assert.equal(
    requiresAuthGate({ mode: 'authenticated', subject: 'user-1', roles: [] }),
    false,
  );
});
