import test from 'node:test';
import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';

// Exercise the exported pure display functions without loading WebGL or a DOM.
const source = await readFile(new URL('../internal/service/graph_static/app.js', import.meta.url), 'utf8');
const helpers = source.slice(0, source.indexOf('function App()')).replace(/^import .*;\n/gm, '');
const { semanticDisplayEdges, semanticLayerStatus } = await import(`data:text/javascript;base64,${Buffer.from(helpers).toString('base64')}`);
const explicit = { id: 'e', source: 'a', target: 'b', kind: 'explicit' };
const semantic = (id, source, target, similarity) => ({ id, source, target, similarity, kind: 'semantic_similarity' });
const edges = [explicit, semantic('ab', 'a', 'b', .95), semantic('ac', 'a', 'c', .9), semantic('bc', 'b', 'c', .8)];

test('toggle, cosine and per-node neighbour limits control the scene edges', () => {
  assert.deepEqual(semanticDisplayEdges(edges, null, false, .85, 1), [explicit]);
  assert.deepEqual(semanticDisplayEdges(edges, null, true, .85, 1).map(e => e.id), ['e', 'ab']);
  assert.deepEqual(semanticDisplayEdges(edges, null, true, .98, 5), [explicit]);
  assert.deepEqual(semanticDisplayEdges(edges, 'c', true, .85, 1).map(e => e.id), ['e', 'ab', 'ac']);
  assert.deepEqual(semanticDisplayEdges(edges, null, true, .85, 1).map(e => e.id), ['e', 'ab']);
});

test('layer status explains hidden, partial and empty results', () => {
  const graph = { mode: 'direct', edges, revisions: { embedding: 'partial' } };
  assert.match(semanticLayerStatus(null, .85, true), /Loading/);
  assert.match(semanticLayerStatus(graph, .85, false), /hidden · 2 links/);
  assert.match(semanticLayerStatus(graph, .85, true), /2 semantic links available/);
  assert.match(semanticLayerStatus(graph, .85, true), /incomplete; current ready vectors/);
  assert.match(semanticLayerStatus(graph, .98, true), /No semantic links/);
  assert.match(semanticLayerStatus({ mode: 'aggregated', cluster_edges: [] }, .85, true), /view limits/);
  assert.doesNotMatch(semanticLayerStatus({ mode: 'direct', edges }, .85, true), /incomplete/);
});
