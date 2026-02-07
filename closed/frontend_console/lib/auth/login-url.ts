const normalizeBase = (value: string) => (value.endsWith('/') ? value.slice(0, -1) : value);

const isAbsoluteURL = (value: string) => /^https?:\/\//i.test(value);

export function getGatewayLoginUrl(returnToPath: string = '/console'): string {
  const base = process.env.NEXT_PUBLIC_GATEWAY_URL?.trim();
  if (!base) {
    throw new Error('NEXT_PUBLIC_GATEWAY_URL is required to build login URL');
  }
  const site = process.env.NEXT_PUBLIC_SITE_URL?.trim();
  if (!site) {
    throw new Error('NEXT_PUBLIC_SITE_URL is required to build login URL');
  }
  const normalizedBase = normalizeBase(base);
  const normalizedSite = normalizeBase(site);
  const trimmed = returnToPath.trim() || '/console';
  const returnTo = isAbsoluteURL(trimmed)
    ? trimmed
    : `${normalizedSite}${trimmed.startsWith('/') ? trimmed : `/${trimmed}`}`;
  return `${normalizedBase}/auth/login?return_to=${encodeURIComponent(returnTo)}`;
}
