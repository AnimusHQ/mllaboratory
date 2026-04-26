'use client';

import { useMemo } from 'react';
import { useRouter, useSearchParams } from 'next/navigation';

import { Button } from '@/components/ui/button';

const statusOptions = [
  { value: '', label: 'Все' },
  { value: 'PENDING', label: 'Pending' },
  { value: 'DELIVERED', label: 'Delivered' },
  { value: 'FAILED', label: 'Failed' },
  { value: 'DLQ', label: 'DLQ' },
];

export function DeliveryFilters() {
  const params = useSearchParams();
  const router = useRouter();
  const status = params.get('delivery_status') ?? '';

  const setStatus = (value: string) => {
    const search = new URLSearchParams(params.toString());
    if (value) {
      search.set('delivery_status', value);
    } else {
      search.delete('delivery_status');
    }
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

  return <div className="flex flex-wrap gap-2">{chips}</div>;
}
