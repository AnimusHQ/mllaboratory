import { strict as assert } from 'node:assert';
import { afterEach, test } from 'node:test';

import { getGatewayLoginUrl } from '../lib/auth/login-url';

const originalGateway = process.env.NEXT_PUBLIC_GATEWAY_URL;
const originalSite = process.env.NEXT_PUBLIC_SITE_URL;

afterEach(() => {
  if (originalGateway === undefined) {
    delete process.env.NEXT_PUBLIC_GATEWAY_URL;
  } else {
    process.env.NEXT_PUBLIC_GATEWAY_URL = originalGateway;
  }
  if (originalSite === undefined) {
    delete process.env.NEXT_PUBLIC_SITE_URL;
  } else {
    process.env.NEXT_PUBLIC_SITE_URL = originalSite;
  }
});

test('getGatewayLoginUrl builds URL with encoded return_to', () => {
  process.env.NEXT_PUBLIC_GATEWAY_URL = 'http://localhost:8080/';
  process.env.NEXT_PUBLIC_SITE_URL = 'http://localhost:3001';
  const url = getGatewayLoginUrl('/console?x=1&y=space here');
  assert.equal(
    url,
    'http://localhost:8080/auth/login?return_to=http%3A%2F%2Flocalhost%3A3001%2Fconsole%3Fx%3D1%26y%3Dspace%20here',
  );
});

test('getGatewayLoginUrl throws when gateway URL missing', () => {
  delete process.env.NEXT_PUBLIC_GATEWAY_URL;
  process.env.NEXT_PUBLIC_SITE_URL = 'http://localhost:3001';
  assert.throws(() => getGatewayLoginUrl('/console'), /NEXT_PUBLIC_GATEWAY_URL is required/);
});

test('getGatewayLoginUrl throws when site URL missing', () => {
  process.env.NEXT_PUBLIC_GATEWAY_URL = 'http://localhost:8080';
  delete process.env.NEXT_PUBLIC_SITE_URL;
  assert.throws(() => getGatewayLoginUrl('/console'), /NEXT_PUBLIC_SITE_URL is required/);
});
