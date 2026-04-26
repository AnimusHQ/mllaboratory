import { StatusPill } from '@/components/console/status-pill';

type PipelineNode = {
  id: string;
  name: string;
  deps: string[];
};

const normalizeDeps = (value: unknown): string[] => {
  if (!value) {
    return [];
  }
  if (Array.isArray(value)) {
    return value.map(String);
  }
  if (typeof value === 'string') {
    return [value];
  }
  return [];
};

export function PipelineGraph({
  pipelineSpec,
  attemptsByStep,
}: {
  pipelineSpec: Record<string, unknown>;
  attemptsByStep?: Record<string, number>;
}) {
  const rawSteps = (pipelineSpec?.steps as unknown[]) ?? (pipelineSpec?.nodes as unknown[]) ?? [];
  const nodes: PipelineNode[] = Array.isArray(rawSteps)
    ? rawSteps.map((step, index) => {
        const obj = (step as Record<string, unknown>) ?? {};
        const id = String(obj.id ?? obj.name ?? obj.step ?? `step-${index + 1}`);
        const name = String(obj.name ?? obj.id ?? obj.step ?? `Шаг ${index + 1}`);
        const deps = normalizeDeps(obj.depends_on ?? obj.dependsOn ?? obj.needs ?? obj.requires);
        return { id, name, deps };
      })
    : [];

  if (nodes.length === 0) {
    return (
      <div className="rounded-[24px] border border-white/12 bg-[#0b1626]/85 px-4 py-6 text-sm text-white/70 shadow-[0_18px_36px_rgba(3,10,18,0.6)]">
        Структура pipelineSpec не содержит явных шагов. Отображается только JSON‑представление.
      </div>
    );
  }

  return (
    <div className="overflow-auto rounded-[24px] border border-white/12 bg-[#0b1626]/90 shadow-[0_18px_36px_rgba(3,10,18,0.6)]">
      <table className="console-table">
        <thead>
          <tr>
            <th>Шаг</th>
            <th>Зависимости</th>
            <th>Попытки</th>
            <th>Статус</th>
          </tr>
        </thead>
        <tbody>
          {nodes.map((node) => (
            <tr key={node.id}>
              <td className="font-mono text-xs">{node.name}</td>
              <td className="text-xs text-white/60">
                {node.deps.length ? node.deps.join(', ') : '—'}
              </td>
              <td className="text-xs">{attemptsByStep?.[node.id] ?? 0}</td>
              <td>
                <StatusPill status="pending" label="Не исполнено" />
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
