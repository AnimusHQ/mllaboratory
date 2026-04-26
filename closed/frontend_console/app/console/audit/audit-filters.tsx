'use client';

import { useState } from 'react';
import { useRouter, useSearchParams } from 'next/navigation';

import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';

export function AuditFilters() {
  const params = useSearchParams();
  const router = useRouter();
  const [query, setQuery] = useState(params.get('q') ?? '');

  const apply = () => {
    const search = new URLSearchParams(params.toString());
    if (query.trim()) {
      search.set('q', query.trim());
    } else {
      search.delete('q');
    }
    router.push(`?${search.toString()}`);
  };

  return (
    <div className="flex flex-wrap items-center gap-3">
      <Input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Поиск по actor, action, resource" />
      <Button variant="secondary" size="sm" onClick={apply}>
        Фильтровать
      </Button>
    </div>
  );
}
