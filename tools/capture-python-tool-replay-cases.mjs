import { readFile, writeFile } from 'node:fs/promises';
import { join } from 'node:path';
import { createHash } from 'node:crypto';
const root = new URL('../', import.meta.url).pathname;
const families = ['proposal-tools.json', 'read-tools.json', 'staging-tool-dispatch.json', 'asset-tool-dispatch.json', 'audit-tool-dispatch.json', 'status-tool-dispatch.json', 'manifest-tool-dispatch.json', 'inventory-tool-dispatch.json', 'metadata-tool-dispatch.json', 'mutation-tool-dispatch.json', 'trash-tool-dispatch.json'];
const byKey = new Map(), definitions = new Map();
for (const fixture of families) {
  const doc = JSON.parse(await readFile(join(root, 'testdata/parity', fixture)));
  for (const definition of doc.definitions || []) {
    const previous = definitions.get(definition.name);
    if (previous && JSON.stringify(previous) !== JSON.stringify(definition)) throw new Error(`conflicting definition ${definition.name}`);
    definitions.set(definition.name, definition);
  }
  for (const [index, item] of doc.calls.entries()) {
    let request = item.request;
    if (typeof request === 'string') request = JSON.parse(request);
    const name = request?.params?.name;
    if (!name?.startsWith('memory_')) continue;
    const expected = item.expected, key = JSON.stringify([request, expected]);
    if (!byKey.has(key)) {
      const digest = createHash('sha256').update(key).digest('hex');
      byKey.set(key, { case_id: `rpc-${digest.slice(0, 16)}`, tool: name, source_fixture: fixture, source_index: index, request, expected, comparator: 'json_value_with_text_json_normalization' });
    }
  }
}
const cases = [...byKey.values()].sort((a, b) => a.tool.localeCompare(b.tool) || a.case_id.localeCompare(b.case_id)), counts = {};
for (const item of cases) counts[item.tool] = (counts[item.tool] || 0) + 1;
const output = { schema_version: 1, python_commit: '7f29e8b003557f0105f47ed353b7f65a33619456', oracle: 'Python-generated JSON-RPC tool registration/dispatch fixtures', families, definitions: [...definitions.values()].sort((a, b) => a.name.localeCompare(b.name)), cases, counts };
await writeFile(join(root, 'testdata/parity/python-tool-replay-cases.json'), `${JSON.stringify(output, null, 2)}\n`);
console.log(JSON.stringify({ families: families.length, cases: cases.length, tools: Object.keys(counts).length, definitions: definitions.size, counts }));
