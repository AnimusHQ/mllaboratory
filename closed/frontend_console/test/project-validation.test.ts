import { strict as assert } from 'node:assert';
import { test } from 'node:test';

import { validateProjectCreateInput } from '../lib/projects';

test('validateProjectCreateInput rejects invalid metadata JSON', () => {
  const result = validateProjectCreateInput({ name: 'core-project', metadataText: '{invalid json' });
  assert.equal(result.ok, false);
});

test('validateProjectCreateInput accepts valid payload', () => {
  const result = validateProjectCreateInput({ name: 'core-project', metadataText: '{"tier":"gold"}' });
  assert.equal(result.ok, true);
  if (result.ok) {
    assert.equal(result.payload.name, 'core-project');
    assert.deepEqual(result.payload.metadata, { tier: 'gold' });
  }
});
