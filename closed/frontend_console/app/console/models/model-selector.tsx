'use client';

import { useState } from 'react';
import { useRouter, useSearchParams } from 'next/navigation';

import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';

export function ModelSelector() {
  const params = useSearchParams();
  const router = useRouter();
  const [modelId, setModelId] = useState(params.get('model_id') ?? '');

  const apply = () => {
    const search = new URLSearchParams(params.toString());
    if (modelId.trim()) {
      search.set('model_id', modelId.trim());
    } else {
      search.delete('model_id');
    }
    router.push(`?${search.toString()}`);
  };

  return (
    <div className="flex flex-wrap items-center gap-3">
      <Input value={modelId} onChange={(event) => setModelId(event.target.value)} placeholder="model_id" />
      <Button variant="secondary" size="sm" onClick={apply}>
        Показать версии
      </Button>
    </div>
  );
}
