'use client';

import { useState } from 'react';
import { useRouter, useSearchParams } from 'next/navigation';

import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';

export function PipelineSelector() {
  const params = useSearchParams();
  const router = useRouter();
  const [runId, setRunId] = useState(params.get('run_id') ?? '');

  const apply = () => {
    const search = new URLSearchParams(params.toString());
    if (runId.trim()) {
      search.set('run_id', runId.trim());
    } else {
      search.delete('run_id');
    }
    router.push(`?${search.toString()}`);
  };

  return (
    <div className="flex flex-wrap items-center gap-3">
      <Input value={runId} onChange={(event) => setRunId(event.target.value)} placeholder="run_id для DAG" />
      <Button variant="secondary" size="sm" onClick={apply}>
        Загрузить граф
      </Button>
    </div>
  );
}
