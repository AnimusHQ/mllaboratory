'use client';

import { useMemo, useState } from 'react';
import { useRouter, useSearchParams } from 'next/navigation';

import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';

const statusOptions = [
  { value: '', label: 'Все' },
  { value: 'draft', label: 'Draft' },
  { value: 'validated', label: 'Validated' },
  { value: 'approved', label: 'Approved' },
  { value: 'deprecated', label: 'Deprecated' },
];

export function ModelFilters() {
  const params = useSearchParams();
  const router = useRouter();
  const [query, setQuery] = useState(params.get('q') ?? '');
  const status = params.get('status') ?? '';

  const apply = () => {
    const search = new URLSearchParams(params.toString());
    if (query.trim()) {
      search.set('q', query.trim());
    } else {
      search.delete('q');
    }
    search.delete('page');
    router.push(`?${search.toString()}`);
  };

  const setStatus = (value: string) => {
    const search = new URLSearchParams(params.toString());
    if (value) {
      search.set('status', value);
    } else {
      search.delete('status');
    }
    search.delete('page');
    router.push(`?${search.toString()}`);
  };

  const chips = useMemo(
    () =>
      statusOptions.map((option) => (
        <Button
          key={option.value || 'all'}
          variant={status === option.value ? 'default' : 'secondary'}
          size="sm"
          onClick={() => setStatus(option.value)}
        >
          {option.label}
        </Button>
      )),
    [status],
  );

  return (
    <div className="flex flex-wrap items-center gap-3">
      <div className="flex flex-1 items-center gap-2">
        <Input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Поиск по модели и статусу" />
        <Button variant="secondary" size="sm" onClick={apply}>
          Найти
        </Button>
      </div>
      <div className="flex flex-wrap gap-2">{chips}</div>
    </div>
  );
}
