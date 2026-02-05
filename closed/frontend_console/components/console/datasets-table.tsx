'use client';

import { useMemo, useState } from 'react';

import { CopyButton } from '@/components/console/copy-button';
import { Table, TableContainer, TableEmpty } from '@/components/ui/table';
import type { components } from '@/lib/gateway-openapi';

export type Dataset = components['schemas']['Dataset'];

export function DatasetsTable({ datasets }: { datasets: Dataset[] }) {
  const [query, setQuery] = useState('');

  const filtered = useMemo(() => {
    if (!query.trim()) {
      return datasets;
    }
    const q = query.trim().toLowerCase();
    return datasets.filter((dataset) =>
      [dataset.dataset_id, dataset.name, dataset.description ?? ''].some((value) => value.toLowerCase().includes(q)),
    );
  }, [datasets, query]);

  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-wrap items-center gap-3">
        <input
          className="h-9 w-64 rounded-md border border-input bg-transparent px-3 text-sm"
          placeholder="Поиск по имени или идентификатору"
          value={query}
          onChange={(event) => setQuery(event.target.value)}
        />
        <div className="console-pill">Сортировка: created_at ↓</div>
        <div className="console-pill">Фильтр: все</div>
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
            {filtered.map((dataset) => (
              <tr key={dataset.dataset_id}>
                <td className="font-mono text-xs">{dataset.dataset_id}</td>
                <td>{dataset.name}</td>
                <td className="text-muted-foreground">{dataset.description ?? '—'}</td>
                <td className="text-xs text-muted-foreground">{dataset.created_at}</td>
                <td>
                  <CopyButton value={dataset.dataset_id} />
                </td>
              </tr>
            ))}
          </tbody>
        </Table>
        {filtered.length === 0 ? <TableEmpty>Ничего не найдено.</TableEmpty> : null}
      </TableContainer>
    </div>
  );
}
