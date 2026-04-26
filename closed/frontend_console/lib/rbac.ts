export type EffectiveRole = 'viewer' | 'editor' | 'admin' | 'unknown';

const roleRank: Record<string, number> = {
  viewer: 1,
  editor: 2,
  admin: 3,
  platform_admin: 3,
  'platform-admin': 3,
};

export function deriveEffectiveRole(roles: string[] | undefined): EffectiveRole {
  if (!roles || roles.length === 0) {
    return 'unknown';
  }
  let best: EffectiveRole = 'unknown';
  let score = 0;
  for (const raw of roles) {
    const role = raw.trim().toLowerCase();
    const rank = roleRank[role] ?? 0;
    if (rank > score) {
      score = rank;
      best = rank === roleRank.admin ? 'admin' : (role as EffectiveRole);
    }
  }
  return best;
}

export function roleLabel(role: EffectiveRole): string {
  switch (role) {
    case 'admin':
      return 'Администратор';
    case 'editor':
      return 'Оператор';
    case 'viewer':
      return 'Наблюдатель';
    default:
      return 'Не определена';
  }
}

export function isAdminRole(roles: string[] | undefined): boolean {
  return deriveEffectiveRole(roles) === 'admin';
}

export type Capability =
  | 'dataset:read'
  | 'dataset:write'
  | 'artifact:read'
  | 'devenv:read'
  | 'devenv:write'
  | 'env:read'
  | 'env:write'
  | 'run:read'
  | 'run:write'
  | 'run:approve'
  | 'model:read'
  | 'model:write'
  | 'model:approve'
  | 'model:export'
  | 'audit:read'
  | 'ops:read';

export function capabilityLabel(capability: Capability): string {
  switch (capability) {
    case 'dataset:read':
      return 'DatasetRead';
    case 'dataset:write':
      return 'DatasetWrite';
    case 'artifact:read':
      return 'ArtifactRead';
    case 'devenv:read':
      return 'DevEnvRead';
    case 'devenv:write':
      return 'DevEnvWrite';
    case 'env:read':
      return 'EnvRead';
    case 'env:write':
      return 'EnvWrite';
    case 'run:read':
      return 'RunRead';
    case 'run:write':
      return 'RunWrite';
    case 'run:approve':
      return 'RunApprove';
    case 'model:read':
      return 'ModelRead';
    case 'model:write':
      return 'ModelWrite';
    case 'model:approve':
      return 'ModelApprove';
    case 'model:export':
      return 'ModelExport';
    case 'audit:read':
      return 'AuditRead';
    case 'ops:read':
      return 'OpsRead';
    default:
      return 'RBAC';
  }
}

export function can(role: EffectiveRole, capability: Capability): boolean {
  if (role === 'admin') {
    return true;
  }
  if (role === 'editor') {
    switch (capability) {
      case 'ops:read':
        return false;
      case 'audit:read':
        return false;
      default:
        return true;
    }
  }
  if (role === 'viewer') {
    if (capability === 'audit:read' || capability === 'ops:read') {
      return false;
    }
    return capability.endsWith(':read');
  }
  return false;
}
