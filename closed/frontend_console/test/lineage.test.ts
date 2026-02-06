import { strict as assert } from 'node:assert';
import { test } from 'node:test';

import { paginateLineageEdges, paginateLineageNodes, sortLineageEdges, sortLineageNodes } from '../lib/lineage';

test('sortLineageNodes orders by type then id', () => {
  const nodes = [
    { type: 'artifact', id: 'b' },
    { type: 'run', id: 'a' },
    { type: 'artifact', id: 'a' },
  ];
  const sorted = sortLineageNodes(nodes);
  assert.deepEqual(sorted, [
    { type: 'artifact', id: 'a' },
    { type: 'artifact', id: 'b' },
    { type: 'run', id: 'a' },
  ]);
});

test('sortLineageEdges orders by time then event_id', () => {
  const edges = [
    {
      event_id: 2,
      occurred_at: '2024-01-01T00:00:02Z',
      actor: 'a',
      subject_type: 'run',
      subject_id: '1',
      predicate: 'produced',
      object_type: 'artifact',
      object_id: 'a',
    },
    {
      event_id: 1,
      occurred_at: '2024-01-01T00:00:01Z',
      actor: 'a',
      subject_type: 'run',
      subject_id: '1',
      predicate: 'produced',
      object_type: 'artifact',
      object_id: 'b',
    },
  ];
  const sorted = sortLineageEdges(edges);
  assert.equal(sorted[0].event_id, 1);
  assert.equal(sorted[1].event_id, 2);
});

test('paginateLineageNodes slices deterministically after sorting', () => {
  const nodes = [
    { type: 'model', id: '2' },
    { type: 'artifact', id: 'b' },
    { type: 'artifact', id: 'a' },
    { type: 'model', id: '1' },
  ];
  const page = paginateLineageNodes(nodes, 1, 2);
  assert.deepEqual(page, [
    { type: 'artifact', id: 'a' },
    { type: 'artifact', id: 'b' },
  ]);
});

test('paginateLineageEdges slices deterministically after sorting', () => {
  const edges = [
    { event_id: 3, occurred_at: '2024-01-01T00:00:03Z', actor: 'a', subject_type: 'run', subject_id: '1', predicate: 'p', object_type: 'artifact', object_id: 'c' },
    { event_id: 1, occurred_at: '2024-01-01T00:00:01Z', actor: 'a', subject_type: 'run', subject_id: '1', predicate: 'p', object_type: 'artifact', object_id: 'a' },
    { event_id: 2, occurred_at: '2024-01-01T00:00:02Z', actor: 'a', subject_type: 'run', subject_id: '1', predicate: 'p', object_type: 'artifact', object_id: 'b' },
  ];
  const page = paginateLineageEdges(edges, 2, 1);
  assert.equal(page.length, 1);
  assert.equal(page[0].event_id, 2);
});
