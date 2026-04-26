import { strict as assert } from 'node:assert';
import { test } from 'node:test';

import { buildProjectCookie, persistProjectId, readProjectIdFromStorage } from '../lib/project-context';

test('readProjectIdFromStorage prefers local storage value', () => {
  const cookie = 'animus_project_id=from-cookie; path=/';
  const value = readProjectIdFromStorage('local-project', cookie);
  assert.equal(value, 'local-project');
});

test('readProjectIdFromStorage falls back to cookie value', () => {
  const cookie = 'other=1; animus_project_id=proj%2Falpha; path=/';
  const value = readProjectIdFromStorage('', cookie);
  assert.equal(value, 'proj/alpha');
});

test('persistProjectId writes storage and cookie', () => {
  const storage: { setItem: (key: string, value: string) => void; removeItem: (key: string) => void } & {
    data: Record<string, string>;
  } = {
    data: {},
    setItem(key, value) {
      this.data[key] = value;
    },
    removeItem(key) {
      delete this.data[key];
    },
  };

  const cookie = persistProjectId(storage, ' project-1 ');
  assert.equal(storage.data['animus.console.project'], 'project-1');
  assert.equal(cookie, buildProjectCookie('project-1'));

  const cleared = persistProjectId(storage, '');
  assert.equal(storage.data['animus.console.project'], undefined);
  assert.equal(cleared, buildProjectCookie(''));
});
