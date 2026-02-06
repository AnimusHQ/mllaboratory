import { capabilityLabel } from '@/lib/rbac';

export function PolicyHint({ allowed, capability }: { allowed: boolean; capability: Parameters<typeof capabilityLabel>[0] }) {
  if (allowed) {
    return null;
  }
  return (
    <div className="text-xs text-muted-foreground">
      Ограничение доступа: требуется {capabilityLabel(capability)}. Код: RBAC_DENIED.
    </div>
  );
}
