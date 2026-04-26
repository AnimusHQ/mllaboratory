#!/usr/bin/env node

const modeArg = process.argv.find((arg) => arg.startsWith('--mode='));
const mode = modeArg ? modeArg.split('=')[1] : null;

if (!mode || !['build', 'runtime'].includes(mode)) {
  console.error('Usage: node scripts/validate-env.mjs --mode=build|runtime');
  process.exit(1);
}

import fs from 'node:fs';
import path from 'node:path';

const requiredByMode = {
  build: ['NEXT_PUBLIC_SITE_URL', 'NEXT_PUBLIC_GATEWAY_URL'],
  runtime: ['NEXT_PUBLIC_SITE_URL', 'NEXT_PUBLIC_GATEWAY_URL'],
};

const errors = [];
const required = requiredByMode[mode];

const envFiles = ['.env.local', '.env'];
for (const file of envFiles) {
  const filePath = path.resolve(process.cwd(), file);
  if (!fs.existsSync(filePath)) {
    continue;
  }
  const contents = fs.readFileSync(filePath, 'utf8');
  const lines = contents.split(/\r?\n/);
  for (const rawLine of lines) {
    const line = rawLine.trim();
    if (!line || line.startsWith('#')) {
      continue;
    }
    const eqIndex = line.indexOf('=');
    if (eqIndex === -1) {
      continue;
    }
    const key = line.slice(0, eqIndex).trim();
    if (!key) {
      continue;
    }
    let value = line.slice(eqIndex + 1).trim();
    if (
      (value.startsWith('"') && value.endsWith('"')) ||
      (value.startsWith("'") && value.endsWith("'"))
    ) {
      value = value.slice(1, -1);
    }
    if (!(key in process.env)) {
      process.env[key] = value;
    }
  }
}

for (const name of required) {
  const value = process.env[name];
  if (!value || !value.trim()) {
    errors.push(`Missing required environment variable: ${name}`);
  }
}

const siteUrl = process.env.NEXT_PUBLIC_SITE_URL?.trim();
if (siteUrl) {
  try {
    const parsed = new URL(siteUrl);
    if (!['http:', 'https:'].includes(parsed.protocol)) {
      errors.push('NEXT_PUBLIC_SITE_URL must start with http:// or https://');
    }
  } catch {
    errors.push('NEXT_PUBLIC_SITE_URL must be a valid absolute URL');
  }
}

const gatewayUrl = process.env.NEXT_PUBLIC_GATEWAY_URL?.trim();
if (gatewayUrl) {
  try {
    const parsed = new URL(gatewayUrl);
    if (!['http:', 'https:'].includes(parsed.protocol)) {
      errors.push('NEXT_PUBLIC_GATEWAY_URL must start with http:// or https://');
    }
  } catch {
    errors.push('NEXT_PUBLIC_GATEWAY_URL must be a valid absolute URL');
  }
}

if (errors.length) {
  console.error(`\nConfiguration error(s):\n- ${errors.join('\n- ')}\n`);
  process.exit(1);
}
