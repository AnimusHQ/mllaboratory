'use client';

import { useMemo, useState } from 'react';
import { useRouter, useSearchParams } from 'next/navigation';

import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';

const stateOptions = [
  { value: '', label: 'Все' },
  { value: 'provisioning', label: 'Provisioning' },
  { value: 'active', label: 'Active' },
  { value: 'failed', label: 'Failed' },
  { value: 'expired', label: 'Expired' },
  { value: 'deleted', label: 'Deleted' },
];

export function DevEnvFilters() {
  const params = useSearchParams();
  const router = useRouter();
  const [query, setQuery] = useState(params.get('q') ?? '');
  const state = params.get('state') ?? '';

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

  const setState = (value: string) => {
    const search = new URLSearchParams(params.toString());
    if (value) {
      search.set('state', value);
    } else {
      search.delete('state');
    }
    search.delete('page');
    router.push(`?${search.toString()}`);
  };

  const chips = useMemo(
    () =>
      stateOptions.map((option) => (
        <Button
          key={option.value || 'all'}
          variant={state === option.value ? 'default' : 'secondary'}
          size="sm"
          onClick={() => setState(option.value)}
        >
          {option.label}
        </Button>
      )),
    [state],
  );

  return (
    <div className="flex flex-wrap items-center gap-3">
      <div className="flex flex-1 items-center gap-2">
        <Input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Поиск по repo, template, id" />
        <Button variant="secondary" size="sm" onClick={apply}>
          Найти
        </Button>
      </div>
      <div className="flex flex-wrap gap-2">{chips}</div>
    </div>
  );
}
