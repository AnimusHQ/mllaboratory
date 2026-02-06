import { strict as assert } from 'node:assert';
import { test } from 'node:test';

import { can } from '../lib/rbac';

test('rbac gating respects role capability mapping', () => {
  assert.equal(can('viewer', 'run:read'), true);
  assert.equal(can('viewer', 'run:write'), false);
  assert.equal(can('viewer', 'dataset:read'), true);
  assert.equal(can('viewer', 'dataset:write'), false);
  assert.equal(can('viewer', 'artifact:read'), true);
  assert.equal(can('viewer', 'env:write'), false);
  assert.equal(can('viewer', 'audit:read'), false);
  assert.equal(can('editor', 'model:write'), true);
  assert.equal(can('editor', 'dataset:write'), true);
  assert.equal(can('editor', 'env:write'), true);
  assert.equal(can('editor', 'audit:read'), false);
  assert.equal(can('admin', 'ops:read'), true);
  assert.equal(can('unknown', 'run:read'), false);
});
