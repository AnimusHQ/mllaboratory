'use client';

import { useMemo, useState } from 'react';
import { useRouter, useSearchParams } from 'next/navigation';

import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';

const statusOptions = [
  { value: '', label: 'Все' },
  { value: 'pending', label: 'Ожидает' },
  { value: 'running', label: 'Выполняется' },
  { value: 'succeeded', label: 'Успешно' },
  { value: 'failed', label: 'Ошибка' },
  { value: 'canceled', label: 'Отменено' },
];

export function RunFilters() {
  const params = useSearchParams();
  const router = useRouter();
  const [query, setQuery] = useState(params.get('q') ?? '');
  const status = params.get('status') ?? '';

  const onSearch = () => {
    const search = new URLSearchParams(params.toString());
    if (query.trim()) {
      search.set('q', query.trim());
    } else {
      search.delete('q');
    }
    search.delete('page');
    router.push(`?${search.toString()}`);
  };

  const onStatus = (next: string) => {
    const search = new URLSearchParams(params.toString());
    if (next) {
      search.set('status', next);
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
          onClick={() => onStatus(option.value)}
        >
          {option.label}
        </Button>
      )),
    [status],
  );

  return (
    <div className="flex flex-wrap items-center gap-3">
      <div className="flex flex-1 items-center gap-2">
        <Input
          value={query}
          onChange={(event) => setQuery(event.target.value)}
          placeholder="Поиск по run_id, experiment_id, dataset_version_id, git"
          className="max-w-lg"
        />
        <Button variant="secondary" size="sm" onClick={onSearch}>
          Найти
        </Button>
      </div>
      <div className="flex flex-wrap gap-2">{chips}</div>
    </div>
  );
}
