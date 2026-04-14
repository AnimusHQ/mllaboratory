'use client';

import { useState } from 'react';

import { CopyButton } from '@/components/console/copy-button';
import { ErrorState } from '@/components/console/error-state';
import { PolicyHint } from '@/components/console/policy-hint';
import { StatusPill } from '@/components/console/status-pill';
import { Button } from '@/components/ui/button';
import { Table, TableContainer, TableEmpty } from '@/components/ui/table';
import { GatewayAPIError, gatewayFetchJSON } from '@/lib/gateway-client';
import type { components } from '@/lib/gateway-openapi';
import { formatDateTime } from '@/lib/format';
import { useOperations } from '@/lib/operations';
import { can, type EffectiveRole } from '@/lib/rbac';

type Delivery = components['schemas']['ExportDelivery'];

export function AuditDeliveryTable({ deliveries, role }: { deliveries: Delivery[]; role: EffectiveRole }) {
  const [rows, setRows] = useState(deliveries);
  const [error, setError] = useState<GatewayAPIError | null>(null);
  const { addOperation, updateOperation } = useOperations();
  const allowed = can(role, 'ops:read');

  const replay = async (delivery: Delivery) => {
    if (!can(role, 'ops:read')) {
      return;
    }
    setError(null);
    const operationId = `audit-replay-${delivery.delivery_id}-${Date.now()}`;
    addOperation({
      id: operationId,
      label: 'Replay DLQ',
      status: 'pending',
      createdAt: new Date().toISOString(),
      details: String(delivery.delivery_id),
    });
    try {
      await gatewayFetchJSON(`/api/audit/admin/audit/exports/dlq/${delivery.delivery_id}:replay`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({}),
      });
      updateOperation(operationId, 'succeeded', String(delivery.delivery_id));
    } catch (err) {
      if (err instanceof GatewayAPIError) {
        setError(err);
        updateOperation(operationId, 'failed', err.code);
      } else {
        updateOperation(operationId, 'failed', 'unknown');
      }
    }
  };

  return (
    <div className="flex flex-col gap-4">
      {error ? <ErrorState code={error.code} requestId={error.requestId} status={error.status} details={error.details} message={error.message} retryable={error.retryable} /> : null}
      <TableContainer>
        <Table>
          <thead>
            <tr>
              <th>Delivery ID</th>
              <th>Sink</th>
              <th>Status</th>
              <th>Попытки</th>
              <th>Время</th>
              <th>DLQ</th>
            </tr>
          </thead>
          <tbody>
            {rows.map((delivery) => (
              <tr key={delivery.delivery_id}>
                <td className="font-mono text-xs">
                  <div className="flex items-center gap-2">
                    {delivery.delivery_id}
                    <CopyButton value={String(delivery.delivery_id)} />
                  </div>
                </td>
                <td className="text-xs">
                  <div>sink: {delivery.sink_id}</div>
                  <div className="text-white/60">event: {delivery.event_id}</div>
                </td>
                <td>
                  <StatusPill status={delivery.status} />
                </td>
                <td className="text-xs">{delivery.attempt_count}</td>
                <td className="text-xs text-white/60">
                  <div>Создано: {formatDateTime(delivery.created_at)}</div>
                  <div>Обновлено: {formatDateTime(delivery.updated_at)}</div>
                </td>
                <td>
                    <div className="space-y-1">
                      <Button
                        variant="secondary"
                        size="sm"
                        onClick={() => replay(delivery)}
                        disabled={!allowed || delivery.status !== 'DLQ'}
                      >
                        Replay
                      </Button>
                      <PolicyHint allowed={allowed} capability="ops:read" />
                    </div>
                  </td>
                </tr>
              ))}
          </tbody>
        </Table>
        {rows.length === 0 ? <TableEmpty>Доставки отсутствуют.</TableEmpty> : null}
      </TableContainer>
    </div>
  );
}
