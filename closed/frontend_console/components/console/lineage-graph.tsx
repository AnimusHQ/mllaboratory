import { Table, TableContainer, TableEmpty } from '@/components/ui/table';
import { sortLineageEdges, sortLineageNodes, type LineageEdge, type LineageNode } from '@/lib/lineage';
import { formatDateTime } from '@/lib/format';

export function LineageGraph({ nodes, edges }: { nodes: LineageNode[]; edges: LineageEdge[] }) {
  const sortedNodes = sortLineageNodes(nodes);
  const sortedEdges = sortLineageEdges(edges);

  return (
    <div className="flex flex-col gap-4">
      <div>
        <div className="console-section-title">Узлы</div>
        <TableContainer>
          <Table>
            <thead>
              <tr>
                <th>Тип</th>
                <th>ID</th>
              </tr>
            </thead>
            <tbody>
              {sortedNodes.map((node) => (
                <tr key={`${node.type}-${node.id}`}>
                  <td className="text-xs">{node.type}</td>
                  <td className="font-mono text-xs">{node.id}</td>
                </tr>
              ))}
            </tbody>
          </Table>
          {sortedNodes.length === 0 ? <TableEmpty>Узлы отсутствуют.</TableEmpty> : null}
        </TableContainer>
      </div>
      <div>
        <div className="console-section-title">Рёбра</div>
        <TableContainer>
          <Table>
            <thead>
              <tr>
                <th>Время</th>
                <th>Actor</th>
                <th>Subject</th>
                <th>Predicate</th>
                <th>Object</th>
              </tr>
            </thead>
            <tbody>
              {sortedEdges.map((edge) => (
                <tr key={edge.event_id}>
                  <td className="text-xs text-muted-foreground">{formatDateTime(edge.occurred_at)}</td>
                  <td className="text-xs">{edge.actor}</td>
                  <td className="text-xs">
                    {edge.subject_type}:{edge.subject_id}
                  </td>
                  <td className="text-xs">{edge.predicate}</td>
                  <td className="text-xs">
                    {edge.object_type}:{edge.object_id}
                  </td>
                </tr>
              ))}
            </tbody>
          </Table>
          {sortedEdges.length === 0 ? <TableEmpty>Рёбра отсутствуют.</TableEmpty> : null}
        </TableContainer>
      </div>
    </div>
  );
}
