'use client';

import Link from 'next/link';
import { useMemo, useState } from 'react';
import { useSearchParams } from 'next/navigation';

import { CopyButton } from '@/components/console/copy-button';
import { PolicyHint } from '@/components/console/policy-hint';
import { Pagination } from '@/components/console/pagination';
import { Table, TableContainer, TableEmpty } from '@/components/ui/table';
import type { components } from '@/lib/gateway-openapi';
import { can, type EffectiveRole } from '@/lib/rbac';

export type RunArtifact = components['schemas']['ExperimentRunArtifact'];

export function ArtifactsTable({ artifacts, role }: { artifacts: RunArtifact[]; role: EffectiveRole }) {
  const [query, setQuery] = useState('');
  const params = useSearchParams();
  const pageRaw = Number(params.get('page') ?? '1');
  const page = Number.isFinite(pageRaw) && pageRaw > 0 ? pageRaw : 1;
  const canRead = can(role, 'artifact:read');

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

  const sorted = useMemo(
    () =>
      [...filtered].sort((a, b) => {
        const aTime = a.created_at ? new Date(a.created_at).getTime() : 0;
        const bTime = b.created_at ? new Date(b.created_at).getTime() : 0;
        return bTime - aTime;
      }),
    [filtered],
  );

  const pageSize = 20;
  const totalPages = Math.max(1, Math.ceil(sorted.length / pageSize));
  const safePage = Math.min(page, totalPages);
  const slice = sorted.slice((safePage - 1) * pageSize, safePage * pageSize);

  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-wrap items-center gap-3">
        <input
          className="h-9 w-64 rounded-xl border border-white/15 bg-[#0b1626]/80 px-3 text-sm text-white placeholder:text-white/40 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand-400"
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
            {slice.map((artifact) => (
              <tr key={artifact.artifact_id}>
                <td className="font-mono text-xs">{artifact.artifact_id}</td>
                <td>{artifact.kind}</td>
                <td>{artifact.name ?? '—'}</td>
                <td className="text-white/60">{artifact.filename ?? artifact.object_key}</td>
                <td className="font-mono text-xs text-white/60">{artifact.sha256}</td>
                <td className="text-xs text-white/60">{artifact.size_bytes}</td>
                <td className="space-y-1">
                  <div className="flex items-center gap-2">
                    <CopyButton value={artifact.artifact_id} />
                    {canRead ? (
                      <Link
                        href={`/api/experiments/experiment-runs/${artifact.run_id}/artifacts/${artifact.artifact_id}/download`}
                        className="text-xs font-semibold text-accent hover:text-white"
                      >
                        Скачать
                      </Link>
                    ) : (
                      <span className="text-xs text-white/40">Скачивание недоступно</span>
                    )}
                  </div>
                  <PolicyHint allowed={canRead} capability="artifact:read" />
                </td>
              </tr>
            ))}
          </tbody>
        </Table>
        {slice.length === 0 ? <TableEmpty>Артефакты отсутствуют.</TableEmpty> : null}
      </TableContainer>
      <Pagination page={safePage} totalPages={totalPages} />
    </div>
  );
}
