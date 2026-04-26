export type LineageNode = {
  type: string;
  id: string;
};

export type LineageEdge = {
  event_id: number;
  occurred_at: string;
  actor: string;
  subject_type: string;
  subject_id: string;
  predicate: string;
  object_type: string;
  object_id: string;
};

export const sortLineageNodes = (nodes: LineageNode[]) =>
  [...nodes].sort((a, b) => {
    if (a.type !== b.type) {
      return a.type.localeCompare(b.type);
    }
    return a.id.localeCompare(b.id);
  });

export const sortLineageEdges = (edges: LineageEdge[]) =>
  [...edges].sort((a, b) => {
    const aTime = a.occurred_at ? new Date(a.occurred_at).getTime() : 0;
    const bTime = b.occurred_at ? new Date(b.occurred_at).getTime() : 0;
    if (aTime !== bTime) {
      return aTime - bTime;
    }
    return a.event_id - b.event_id;
  });

export const paginateLineage = <T>(items: T[], page: number, pageSize: number) => {
  const safePage = Number.isFinite(page) && page > 0 ? Math.floor(page) : 1;
  const safeSize = Number.isFinite(pageSize) && pageSize > 0 ? Math.floor(pageSize) : 50;
  const start = (safePage - 1) * safeSize;
  return items.slice(start, start + safeSize);
};

export const paginateLineageNodes = (nodes: LineageNode[], page: number, pageSize: number) =>
  paginateLineage(sortLineageNodes(nodes), page, pageSize);

export const paginateLineageEdges = (edges: LineageEdge[], page: number, pageSize: number) =>
  paginateLineage(sortLineageEdges(edges), page, pageSize);
