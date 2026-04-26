import { strict as assert } from 'node:assert';
import { test } from 'node:test';

import { shouldRedirectToProjectSelection } from '../lib/project-routing';

test('shouldRedirectToProjectSelection redirects when project_id_required', () => {
  assert.equal(shouldRedirectToProjectSelection('project_id_required', '/console/runs'), true);
});

test('shouldRedirectToProjectSelection does not redirect when already on projects', () => {
  assert.equal(shouldRedirectToProjectSelection('project_id_required', '/console/projects'), false);
});
