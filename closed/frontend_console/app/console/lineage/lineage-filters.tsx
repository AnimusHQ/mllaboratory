'use client';

import { useState } from 'react';
import { useRouter, useSearchParams } from 'next/navigation';

import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';

export function LineageFilters() {
  const params = useSearchParams();
  const router = useRouter();
  const [runId, setRunId] = useState(params.get('run_id') ?? '');
  const [modelVersionId, setModelVersionId] = useState(params.get('model_version_id') ?? '');

  const apply = () => {
    const search = new URLSearchParams(params.toString());
    if (runId.trim()) {
      search.set('run_id', runId.trim());
    } else {
      search.delete('run_id');
    }
    if (modelVersionId.trim()) {
      search.set('model_version_id', modelVersionId.trim());
    } else {
      search.delete('model_version_id');
    }
    router.push(`?${search.toString()}`);
  };

  return (
    <div className="flex flex-wrap items-center gap-3">
      <Input value={runId} onChange={(event) => setRunId(event.target.value)} placeholder="run_id" />
      <Input value={modelVersionId} onChange={(event) => setModelVersionId(event.target.value)} placeholder="model_version_id" />
      <Button variant="secondary" size="sm" onClick={apply}>
        Загрузить
      </Button>
    </div>
  );
}
