'use client';

import { useState } from 'react';

import { Button } from '@/components/ui/button';

export function CopyButton({ value }: { value: string }) {
  const [copied, setCopied] = useState(false);
  const onCopy = async () => {
    try {
      await navigator.clipboard.writeText(value);
      setCopied(true);
      setTimeout(() => setCopied(false), 1200);
    } catch {
      setCopied(false);
    }
  };

  return (
    <Button variant="ghost" size="sm" onClick={onCopy}>
      {copied ? 'Скопировано' : 'Скопировать ID'}
    </Button>
  );
}
