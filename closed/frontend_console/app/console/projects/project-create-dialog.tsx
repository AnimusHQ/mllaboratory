'use client';

import { useEffect, useMemo, useState } from 'react';
import { useRouter } from 'next/navigation';

import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Textarea } from '@/components/ui/textarea';
import { getGatewayLoginUrl } from '@/lib/auth/login-url';
import { GatewayAPIError, gatewayFetchJSON } from '@/lib/gateway-client';
import type { components } from '@/lib/gateway-openapi';
import { useProjectContext } from '@/lib/project-context';
import { validateProjectCreateInput } from '@/lib/projects';
import { isAdminRole } from '@/lib/rbac';

type CreateDialogProps = {
  open: boolean;
  onClose: () => void;
};

export function ProjectCreateDialog({ open, onClose }: CreateDialogProps) {
  const router = useRouter();
  const { setProjectId } = useProjectContext();
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [metadataText, setMetadataText] = useState('{}');
  const [busy, setBusy] = useState(false);
  const [validationError, setValidationError] = useState<string | null>(null);
  const [apiError, setApiError] = useState<GatewayAPIError | null>(null);

  useEffect(() => {
    if (!open) {
      setBusy(false);
      setValidationError(null);
      setApiError(null);
    }
  }, [open]);

  const submit = async () => {
    setValidationError(null);
    setApiError(null);
    const validation = validateProjectCreateInput({ name, description, metadataText });
    if (!validation.ok) {
      setValidationError(validation.error);
      return;
    }
    setBusy(true);
    try {
      const response = await gatewayFetchJSON<components['schemas']['Project']>('/api/dataset-registry/projects', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify(validation.payload),
        credentials: 'include',
      });
      setProjectId(response.project_id);
      router.push('/console/runs');
      onClose();
    } catch (err) {
      if (err instanceof GatewayAPIError) {
        setApiError(err);
      } else {
        setApiError(new GatewayAPIError(500, 'gateway_unexpected'));
      }
    } finally {
      setBusy(false);
    }
  };

  const errorMessage = useMemo(() => {
    if (validationError) {
      return validationError;
    }
    if (!apiError) {
      return null;
    }
    if (apiError.status === 401) {
      return 'Сессия истекла. Повторите вход через шлюз.';
    }
    if (apiError.status === 403) {
      return 'Недостаточно прав: требуется роль администратора.';
    }
    if (apiError.code === 'project_name_exists' || apiError.status === 409) {
      return 'Проект с таким именем уже существует.';
    }
    if (apiError.code === 'name_required') {
      return 'Имя проекта обязательно.';
    }
    return 'Сбой создания проекта. Проверьте параметры и повторите запрос.';
  }, [validationError, apiError]);

  if (!open) {
    return null;
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 px-4 py-8">
      <div className="console-surface w-full max-w-2xl">
        <div className="flex items-center justify-between border-b border-white/10 px-6 py-4">
          <div>
            <div className="text-sm font-semibold">Создание проекта</div>
            <div className="text-xs text-white/60">Операция требует административных прав.</div>
          </div>
          <Button variant="ghost" size="sm" onClick={onClose}>
            Закрыть
          </Button>
        </div>
        <div className="grid gap-4 px-6 py-5">
          <div className="grid gap-2">
            <label className="text-xs text-white/60">Имя проекта</label>
            <Input
              value={name}
              onChange={(event) => setName(event.target.value)}
              placeholder="research-core"
            />
            <p className="text-xs text-white/60">
              Допустимо: 3–64 символа, латиница, цифры, “.”, “_”, “-”.
            </p>
          </div>
          <div className="grid gap-2">
            <label className="text-xs text-white/60">Описание</label>
            <Input
              value={description}
              onChange={(event) => setDescription(event.target.value)}
              placeholder="Контур исследований"
            />
          </div>
          <div className="grid gap-2">
            <label className="text-xs text-white/60">Metadata (JSON)</label>
            <Textarea
              value={metadataText}
              onChange={(event) => setMetadataText(event.target.value)}
              rows={4}
            />
          </div>
          {errorMessage ? (
            <Card>
              <CardHeader>
                <CardTitle className="text-sm text-rose-200">Ошибка создания</CardTitle>
                <CardDescription>{errorMessage}</CardDescription>
              </CardHeader>
              {apiError?.status === 401 ? (
                <CardContent>
                  <Button asChild variant="secondary" size="sm">
                    <a href={getGatewayLoginUrl('/console/projects')}>Войти</a>
                  </Button>
                </CardContent>
              ) : null}
              {apiError?.requestId ? (
                <CardContent className="pt-0 text-xs text-white/60">
                  Request ID: {apiError.requestId}
                </CardContent>
              ) : null}
            </Card>
          ) : null}
        </div>
        <div className="flex items-center justify-end gap-3 border-t border-white/10 px-6 py-4">
          <Button variant="ghost" size="sm" onClick={onClose}>
            Отмена
          </Button>
          <Button variant="primary" size="sm" onClick={submit} disabled={busy}>
            {busy ? 'Создание…' : 'Создать проект'}
          </Button>
        </div>
      </div>
    </div>
  );
}

type CreateButtonProps = {
  roles: string[];
  label?: string;
  size?: 'sm' | 'md' | 'lg';
};

export function ProjectCreateButton({ roles, label = 'Создать проект', size = 'sm' }: CreateButtonProps) {
  const [open, setOpen] = useState(false);
  const isAdmin = isAdminRole(roles);

  return (
    <>
      <Button
        variant="secondary"
        size={size}
        disabled={!isAdmin}
        title={isAdmin ? undefined : 'Требуется роль администратора'}
        onClick={() => setOpen(true)}
      >
        {label}
      </Button>
      <ProjectCreateDialog open={open} onClose={() => setOpen(false)} />
    </>
  );
}
