export type ProjectCreateInput = {
  name: string;
  description?: string;
  metadataText?: string;
};

export type ProjectCreatePayload = {
  name: string;
  description?: string;
  metadata: Record<string, unknown>;
};

export type ProjectCreateValidation =
  | { ok: true; payload: ProjectCreatePayload }
  | { ok: false; error: string };

const NAME_PATTERN = /^[a-zA-Z0-9._-]{3,64}$/;

export function validateProjectCreateInput(input: ProjectCreateInput): ProjectCreateValidation {
  const name = (input.name ?? '').trim();
  if (!name) {
    return { ok: false, error: 'Имя проекта обязательно.' };
  }
  if (!NAME_PATTERN.test(name)) {
    return { ok: false, error: 'Имя проекта должно быть 3–64 символа: латиница, цифры, ".", "_" или "-".' };
  }

  const description = (input.description ?? '').trim();
  const metadataRaw = (input.metadataText ?? '').trim();
  let metadata: Record<string, unknown> = {};
  if (metadataRaw) {
    try {
      const parsed = JSON.parse(metadataRaw);
      if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
        return { ok: false, error: 'Metadata должна быть JSON-объектом.' };
      }
      metadata = parsed as Record<string, unknown>;
    } catch {
      return { ok: false, error: 'Metadata должна быть корректным JSON-объектом.' };
    }
  }

  return {
    ok: true,
    payload: {
      name,
      description: description || undefined,
      metadata,
    },
  };
}
