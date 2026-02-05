'use client';

import { useMemo, useState } from 'react';

import { CopyButton } from '@/components/console/copy-button';
import { Table, TableContainer, TableEmpty } from '@/components/ui/table';
import type { components } from '@/lib/gateway-openapi';

export type RunArtifact = components['schemas']['ExperimentRunArtifact'];

export function ArtifactsTable({ artifacts }: { artifacts: RunArtifact[] }) {
  const [query, setQuery] = useState('');

  const filtered = useMemo(() => {
    if (!query.trim()) {
      return artifacts;
    }
    const q = query.trim().toLowerCase();
    return artifacts.filter((artifact) =>
      [artifact.artifact_id, artifact.kind, artifact.name ?? '', artifact.filename ?? '']
        .filter(Boolean)
        .some((value) => value.toLowerCase().includes(q)),
    );
  }, [artifacts, query]);

  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-wrap items-center gap-3">
        <input
          className="h-9 w-64 rounded-md border border-input bg-transparent px-3 text-sm"
          placeholder="Поиск по ID, типу, имени"
          value={query}
          onChange={(event) => setQuery(event.target.value)}
        />
        <div className="console-pill">Сортировка: created_at ↓</div>
      </div>
      <TableContainer>
        <Table>
          <thead>
            <tr>
              <th>Artifact ID</th>
              <th>Тип</th>
              <th>Имя</th>
              <th>Файл</th>
              <th>SHA256</th>
              <th>Размер</th>
              <th>Действия</th>
            </tr>
          </thead>
          <tbody>
            {filtered.map((artifact) => (
              <tr key={artifact.artifact_id}>
                <td className="font-mono text-xs">{artifact.artifact_id}</td>
                <td>{artifact.kind}</td>
                <td>{artifact.name ?? '—'}</td>
                <td className="text-muted-foreground">{artifact.filename ?? artifact.object_key}</td>
                <td className="font-mono text-xs text-muted-foreground">{artifact.sha256}</td>
                <td className="text-xs text-muted-foreground">{artifact.size_bytes}</td>
                <td>
                  <CopyButton value={artifact.artifact_id} />
                </td>
              </tr>
            ))}
          </tbody>
        </Table>
        {filtered.length === 0 ? <TableEmpty>Артефакты отсутствуют.</TableEmpty> : null}
      </TableContainer>
    </div>
  );
}
