'use client';

import Link from 'next/link';
import { useMemo, useState } from 'react';
import { useSearchParams } from 'next/navigation';

import { CopyButton } from '@/components/console/copy-button';
import { Button } from '@/components/ui/button';
import { Table, TableContainer, TableEmpty } from '@/components/ui/table';
import type { components } from '@/lib/gateway-openapi';
import { Pagination } from '@/components/console/pagination';
import { formatDateTime } from '@/lib/format';

export type Dataset = components['schemas']['Dataset'];

export function DatasetsTable({ datasets }: { datasets: Dataset[] }) {
  const [query, setQuery] = useState('');
  const [filter, setFilter] = useState<'all' | 'with_description' | 'without_description'>('all');
  const params = useSearchParams();
  const pageRaw = Number(params.get('page') ?? '1');
  const page = Number.isFinite(pageRaw) && pageRaw > 0 ? pageRaw : 1;

  const filtered = useMemo(() => {
    const sorted = [...datasets].sort((a, b) => {
      const aTime = a.created_at ? new Date(a.created_at).getTime() : 0;
      const bTime = b.created_at ? new Date(b.created_at).getTime() : 0;
      return bTime - aTime;
    });
    if (!query.trim()) {
      return sorted.filter((dataset) => {
        if (filter === 'with_description') {
          return Boolean(dataset.description);
        }
        if (filter === 'without_description') {
          return !dataset.description;
        }
        return true;
      });
    }
    const q = query.trim().toLowerCase();
    return sorted.filter((dataset) =>
      [dataset.dataset_id, dataset.name, dataset.description ?? ''].some((value) => value.toLowerCase().includes(q)),
    );
  }, [datasets, query, filter]);

  const pageSize = 20;
  const totalPages = Math.max(1, Math.ceil(filtered.length / pageSize));
  const safePage = Math.min(page, totalPages);
  const slice = filtered.slice((safePage - 1) * pageSize, safePage * pageSize);

  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-wrap items-center gap-3">
        <input
          className="h-9 w-64 rounded-xl border border-white/15 bg-[#0b1626]/80 px-3 text-sm text-white placeholder:text-white/40 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand-400"
          placeholder="Поиск по имени или идентификатору"
          value={query}
          onChange={(event) => setQuery(event.target.value)}
        />
        <div className="console-pill">Сортировка: created_at ↓</div>
        <div className="flex flex-wrap gap-2">
          <Button variant={filter === 'all' ? 'default' : 'secondary'} size="sm" onClick={() => setFilter('all')}>
            Все
          </Button>
          <Button
            variant={filter === 'with_description' ? 'default' : 'secondary'}
            size="sm"
            onClick={() => setFilter('with_description')}
          >
            С описанием
          </Button>
          <Button
            variant={filter === 'without_description' ? 'default' : 'secondary'}
            size="sm"
            onClick={() => setFilter('without_description')}
          >
            Без описания
          </Button>
        </div>
      </div>
      <TableContainer>
        <Table>
          <thead>
            <tr>
              <th>Dataset ID</th>
              <th>Имя</th>
              <th>Описание</th>
              <th>Создан</th>
              <th>Действия</th>
            </tr>
          </thead>
          <tbody>
            {slice.map((dataset) => (
              <tr key={dataset.dataset_id}>
                <td className="font-mono text-xs">{dataset.dataset_id}</td>
                <td>
                  <Link
                    href={`/console/datasets?dataset_id=${dataset.dataset_id}`}
                    className="text-sm font-semibold text-accent hover:text-white"
                  >
                    {dataset.name}
                  </Link>
                </td>
                <td className="text-white/60">{dataset.description ?? '—'}</td>
                <td className="text-xs text-white/60">{formatDateTime(dataset.created_at)}</td>
                <td>
                  <CopyButton value={dataset.dataset_id} />
                </td>
              </tr>
            ))}
          </tbody>
        </Table>
        {slice.length === 0 ? <TableEmpty>Ничего не найдено.</TableEmpty> : null}
      </TableContainer>
      <Pagination page={safePage} totalPages={totalPages} />
    </div>
  );
}
