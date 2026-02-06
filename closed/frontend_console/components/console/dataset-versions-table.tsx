import Link from 'next/link';

import { CopyButton } from '@/components/console/copy-button';
import { PolicyHint } from '@/components/console/policy-hint';
import { Table, TableContainer, TableEmpty } from '@/components/ui/table';
import type { components } from '@/lib/gateway-openapi';
import { formatDateTime } from '@/lib/format';
import { can, type EffectiveRole } from '@/lib/rbac';

export type DatasetVersion = components['schemas']['DatasetVersion'];

export function DatasetVersionsTable({ versions, role }: { versions: DatasetVersion[]; role: EffectiveRole }) {
  const canRead = can(role, 'dataset:read');
  return (
    <TableContainer>
      <Table>
        <thead>
          <tr>
            <th>Version ID</th>
            <th>Dataset</th>
            <th>SHA256</th>
            <th>Размер</th>
            <th>Создано</th>
            <th>Действия</th>
          </tr>
        </thead>
        <tbody>
          {versions.map((version) => (
            <tr key={version.version_id}>
              <td className="font-mono text-xs">
                <div className="flex items-center gap-2">
                  {version.version_id}
                  <CopyButton value={version.version_id} />
                </div>
              </td>
              <td className="text-xs">{version.dataset_id}</td>
              <td className="font-mono text-xs">{version.content_sha256}</td>
              <td className="text-xs text-muted-foreground">{version.size_bytes ?? '—'}</td>
              <td className="text-xs text-muted-foreground">{formatDateTime(version.created_at)}</td>
              <td>
                <div className="space-y-1">
                  {canRead ? (
                    <Link
                      href={`/api/dataset-registry/dataset-versions/${version.version_id}/download`}
                      className="text-sm font-semibold text-primary"
                    >
                      Скачать
                    </Link>
                  ) : (
                    <span className="text-xs text-muted-foreground">Скачивание недоступно</span>
                  )}
                  <PolicyHint allowed={canRead} capability="dataset:read" />
                </div>
              </td>
            </tr>
          ))}
        </tbody>
      </Table>
      {versions.length === 0 ? <TableEmpty>Версии отсутствуют.</TableEmpty> : null}
    </TableContainer>
  );
}
