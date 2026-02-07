import { strict as assert } from 'node:assert';
import fs from 'node:fs';
import path from 'node:path';
import { test } from 'node:test';

test('tokens file defines core color variables', () => {
  const tokensPath = path.resolve(process.cwd(), 'styles', 'tokens.css');
  const contents = fs.readFileSync(tokensPath, 'utf8');
  assert.match(contents, /--background:/);
  assert.match(contents, /--primary:/);
  assert.match(contents, /--radius:/);
});
