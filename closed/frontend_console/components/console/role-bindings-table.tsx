'use client';

import { useMemo, useState } from 'react';

import { CopyButton } from '@/components/console/copy-button';
import { Table, TableContainer, TableEmpty } from '@/components/ui/table';
import type { components } from '@/lib/gateway-openapi';

export type RoleBinding = components['schemas']['RoleBinding'];

export function RoleBindingsTable({ bindings }: { bindings: RoleBinding[] }) {
  const [query, setQuery] = useState('');

  const filtered = useMemo(() => {
    if (!query.trim()) {
      return bindings;
    }
    const q = query.trim().toLowerCase();
    return bindings.filter((binding) =>
      [binding.binding_id, binding.subject, binding.role, binding.subject_type]
        .filter(Boolean)
        .some((value) => value.toLowerCase().includes(q)),
    );
  }, [bindings, query]);

  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-wrap items-center gap-3">
        <input
          className="h-9 w-64 rounded-xl border border-white/15 bg-[#0b1626]/80 px-3 text-sm text-white placeholder:text-white/40 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand-400"
          placeholder="Поиск по субъекту или роли"
          value={query}
          onChange={(event) => setQuery(event.target.value)}
        />
        <div className="console-pill">Сортировка: created_at ↓</div>
      </div>
      <TableContainer>
        <Table>
          <thead>
            <tr>
              <th>Binding ID</th>
              <th>Тип субъекта</th>
              <th>Субъект</th>
              <th>Роль</th>
              <th>Создан</th>
              <th>Действия</th>
            </tr>
          </thead>
          <tbody>
            {filtered.map((binding) => (
              <tr key={binding.binding_id}>
                <td className="font-mono text-xs">{binding.binding_id}</td>
                <td>{binding.subject_type}</td>
                <td className="text-white/60">{binding.subject}</td>
                <td>{binding.role}</td>
                <td className="text-xs text-white/60">{binding.created_at}</td>
                <td>
                  <CopyButton value={binding.binding_id} />
                </td>
              </tr>
            ))}
          </tbody>
        </Table>
        {filtered.length === 0 ? <TableEmpty>Результаты отсутствуют.</TableEmpty> : null}
      </TableContainer>
    </div>
  );
}
