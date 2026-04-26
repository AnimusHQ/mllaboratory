import { gatewayBaseURL } from '@/lib/gateway-client';

export const buildDevEnvProxyUrl = (proxyPath: string, path: string = '') => {
  const base = gatewayBaseURL();
  const normalizedProxy = proxyPath.startsWith('/') ? proxyPath : `/${proxyPath}`;
  const normalizedPath = path ? (path.startsWith('/') ? path : `/${path}`) : '';
  const prefix = normalizedProxy.startsWith('/api/experiments') ? '' : '/api/experiments';
  return `${base}${prefix}${normalizedProxy}${normalizedPath}`;
};
