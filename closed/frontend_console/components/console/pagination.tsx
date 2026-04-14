'use client';

import { useRouter, useSearchParams } from 'next/navigation';

import { Button } from '@/components/ui/button';

export function Pagination({ page, totalPages, param = 'page' }: { page: number; totalPages: number; param?: string }) {
  const router = useRouter();
  const params = useSearchParams();

  if (totalPages <= 1) {
    return null;
  }

  const setPage = (next: number) => {
    const search = new URLSearchParams(params.toString());
    search.set(param, String(next));
    router.push(`?${search.toString()}`);
  };

  return (
    <div className="flex flex-wrap items-center gap-2">
      <Button variant="secondary" size="sm" onClick={() => setPage(Math.max(1, page - 1))} disabled={page <= 1}>
        Назад
      </Button>
      <div className="text-xs text-white/60">
        Страница {page} из {totalPages}
      </div>
      <Button
        variant="secondary"
        size="sm"
        onClick={() => setPage(Math.min(totalPages, page + 1))}
        disabled={page >= totalPages}
      >
        Вперёд
      </Button>
    </div>
  );
}
