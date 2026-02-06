import type { Metadata, Viewport } from 'next';

import './globals.css';

const metadataBase = new URL(
  process.env.NEXT_PUBLIC_SITE_URL ??
    (process.env.NODE_ENV === 'development' ? 'http://localhost:3001' : 'https://console.local'),
);

export const metadata: Metadata = {
  metadataBase,
  title: {
    default: 'Animus Datalab — Консоль управления',
    template: '%s · Animus Datalab',
  },
  description: 'Внутренняя консоль управления контрольной плоскостью Animus Datalab.',
};

export const viewport: Viewport = {
  themeColor: '#081018',
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="ru" data-surface="ops">
      <body data-surface="ops">{children}</body>
    </html>
  );
}
