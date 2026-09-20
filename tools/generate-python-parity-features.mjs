import { readFile, writeFile, rm, mkdir } from 'node:fs/promises';
import { join } from 'node:path';

const root = new URL('../', import.meta.url).pathname;
const appManifest = JSON.parse(await readFile(join(root, 'testdata/parity/python-functional-manifest.json')));
const surfaceManifest = JSON.parse(await readFile(join(root, 'testdata/parity/python-surface-manifest.json')));
const umcpManifest = JSON.parse(await readFile(join(root, 'umcp/testdata/python-functional-manifest.json')));

const appGroups = {
  'access/managed-access': ['test_access.py', 'test_admin_http.py'],
  'repository/concepts-and-git': ['test_repository_core.py', 'test_legacy_blob_migration.py'],
  'repository/assets-and-skills': ['test_asset_migration.py', 'test_asset_pack_repository.py', 'test_asset_retrieval.py', 'test_skill_import.py', 'test_skill_packs.py', 'test_staged_assets.py'],
  'proposals/control-and-assets': ['test_control_plane.py', 'test_operations.py', 'test_proposal_assets.py'],
  'retrieval/search-answer-and-routing': ['test_evidence.py', 'test_memory_answer.py', 'test_router.py'],
  'models/semantic-and-embeddings': ['test_semantic.py', 'test_semantic_deferred.py', 'test_subprocess_embeddings.py', 'test_warm_subprocess_embeddings.py', 'test_warm_worker_config.py', 'test_cpu_usage.py'],
  'models/needle-and-model-transport': ['test_model_transport.py', 'test_needle_corpus.py', 'test_needle_ffi.py', 'test_needle_router_generator.py'],
  'graph/snapshots-diagnostics-and-ui': ['test_graph_debug.py', 'test_graph_diagnostics.py', 'test_graph_export.py', 'test_graph_layout.py', 'test_graph_refresh.py', 'test_graph_snapshot.py', 'test_graph_vendor.py'],
  'runtime/config-packaging-and-operations': ['test_derived_plane.py', 'test_load_harness.py', 'test_package.py', 'test_release_deploy.py', 'test_runtime_models.py', 'test_workflows.py'],
  'mcp/service-tools-and-workflows': ['test_service_mcp.py'],
};
const umcpGroups = {
  'mcp/schema-tools-and-coercion': ['test_annotations.py', 'test_coercion.py', 'test_introspection.py', 'test_schema_fallbacks.py', 'test_schema_generation.py', 'test_tools.py'],
  'mcp/resources-prompts-and-completion': ['test_async_prompts.py', 'test_completion_logging.py', 'test_prompts.py', 'test_prompts_extra.py', 'test_resources.py', 'test_resources_extra.py'],
  'mcp/discovery-notifications-and-progress': ['test_discovery_pagination.py', 'test_notifications.py', 'test_progress_cancellation.py', 'test_tool_outputs_and_runtime_notifications.py'],
  'mcp/protocol-errors-and-context': ['test_protocol_errors.py', 'test_shared_negotiation_context.py'],
  'mcp/streamable-http-and-sessions': ['test_streamable_http_regressions.py', 'test_streamable_http_sessions.py', 'test_streamable_http_sync_async.py'],
  'mcp/transports-and-integrated-servers': ['test_async_servers.py', 'test_movieserver.py', 'test_transports.py'],
};
const surfaceGroups = {
  'mcp/discovery-and-reading': row => row.category === 'mcp_tool' && /memory_(help|status|search|read|list|inventory|compare_manifest|graph)$/.test(row.name),
  'mcp/answer-routing-and-execute': row => row.category === 'mcp_tool' && /memory_(answer|route|execute)$/.test(row.name),
  'mcp/proposal-lifecycle': row => row.category === 'mcp_tool' && /^memory_(audit|propose(?:_|$)|proposal_|operation_get)/.test(row.name),
  'mcp/assets-and-staging': row => row.category === 'mcp_tool' && /^memory_asset_/.test(row.name),
  'mcp/direct-mutations': row => row.category === 'mcp_tool' && /^memory_(create|patch|trash|restore|purge|rename)$/.test(row.name),
  'mcp/access-administration': row => row.category === 'mcp_tool' && /^access_/.test(row.name),
  'mcp/resources-prompts-and-completion': row => ['mcp_resource', 'mcp_resource_template', 'mcp_prompt'].includes(row.category) || (row.category === 'mcp_protocol' && /^(resources|prompts|completion)\//.test(row.name)),
  'mcp/protocol-lifecycle': row => row.category === 'mcp_protocol' && !/^(resources|prompts|completion)\//.test(row.name),
  'http/admin': row => row.category === 'http_endpoint' && row.name.includes('/admin'),
  'http/staging': row => row.category === 'http_endpoint' && row.name.includes('/assets/staging'),
  'http/graph': row => row.category === 'http_endpoint' && row.name.includes('/graph'),
  'http/mcp-transport': row => row.category === 'http_endpoint' && row.name.includes('/mcp'),
  'cli/operations': row => row.category === 'cli',
};

const safe = value => value.replace(/[^A-Za-z0-9_.:-]+/g, '_');
const sentence = (value, limit = 700) => String(value || '').replace(/\s+/g, ' ').trim().slice(0, limit) || 'the captured Python behavior is exercised';
const display = value => value.replaceAll('-', ' ').replaceAll('_', ' ');
const featureText = lines => `${lines.map(line => line.trimEnd()).join('\n').trimEnd()}\n`;

function goTags(row) { return row.go_tests.map(item => `@go_${safe(item.name)}`).join(' '); }
function sourceRule(row) { return row.python_file || row.node_file.split('/').at(-1); }
function functionalScenario(row) {
  const parts = sentence(row.behavior_summary).split(' checks ', 2);
  const when = parts[0].replace(/^Calls /, '');
  const then = parts[1] || 'the result, state transitions, and error boundary match the captured behavior';
  return [
    `    @${row.row_id} @python_${safe(row.node_name)} ${goTags(row)}`,
    `    Scenario: ${row.node_name}`,
    '      Given the pinned Python reference fixtures and controlled inputs',
    `      When ${when}`,
    `      Then ${then}`,
    '',
  ];
}
function upstreamScenario(row) {
  return [
    `    @${row.row_id} ${goTags(row)}`,
    `    Scenario: ${row.node_name}`,
    '      Given the pinned Python uMCP server or client and controlled protocol inputs',
    `      When ${display(row.node_name.replace(/^test_/, ''))}`,
    '      Then the Go uMCP result matches the captured Python decision, payload, ordering, or error',
    '',
  ];
}
function surfaceScenarios(row) {
  const lines = [
    `    @${row.row_id} ${goTags(row)}`,
    `    Scenario: ${row.name} preserves the Python success contract`,
    `      Given the Python request contract ${sentence(row.python_request, 500)}`,
    `      When an authorized client invokes ${row.name}`,
    `      Then the response matches ${sentence(row.python_response, 500)}`,
    '',
  ];
  for (const profile of ['reader', 'proposer', 'curator', 'admin']) {
    lines.push(
      `    @${row.row_id}_role_${profile} ${goTags(row)}`,
      `    Scenario: ${row.name} as ${profile}`,
      `      Given the canonical ${profile} profile`,
      `      When that principal discovers or invokes ${row.name}`,
      `      Then the Python outcome is ${row.profile_outcomes[profile]}`,
      '',
    );
  }
  lines.push(
    `    @${row.row_id}_failure ${goTags(row)}`,
    `    Scenario: ${row.name} preserves Python validation and failure behavior`,
    '      Given malformed, missing, out-of-scope, or unavailable inputs',
    `      When the client invokes ${row.name}`,
    `      Then ${sentence(row.python_errors, 600)}`,
    '',
  );
  return lines;
}
async function emitFunctional(base, manifest, groups, upstream = false) {
  await rm(base, { recursive: true, force: true });
  const claimed = new Set();
  for (const [target, files] of Object.entries(groups)) {
    const rows = manifest.rows.filter(row => files.includes(sourceRule(row)));
    if (!rows.length) throw new Error(`empty logical group ${target}`);
    const lines = [`Feature: ${display(target)}`, '', `  The scenarios capture Python ${upstream ? 'uMCP' : 'Memento'} behavior at ${manifest.python_commit}.`, '  Rules retain source-module traceability while features group related user behavior.', ''];
    for (const file of files) {
      const items = rows.filter(row => sourceRule(row) === file);
      if (!items.length) throw new Error(`${file} has no rows in ${target}`);
      lines.push(`  Rule: Behavior captured from ${file}`, '');
      for (const row of items) { claimed.add(row.row_id); lines.push(...(upstream ? upstreamScenario(row) : functionalScenario(row))); }
    }
    const path = join(base, `${target}.feature`); await mkdir(join(path, '..'), { recursive: true }); await writeFile(path, featureText(lines));
  }
  if (claimed.size !== manifest.rows.length) throw new Error(`logical groups claimed ${claimed.size}/${manifest.rows.length} rows`);
}
async function emitSurfaces(base, manifest) {
  const claimed = new Set();
  for (const [target, predicate] of Object.entries(surfaceGroups)) {
    const rows = manifest.rows.filter(predicate);
    if (!rows.length) throw new Error(`empty surface group ${target}`);
    const lines = [`Feature: ${display(target)}`, '', '  Each operation has a success contract, four canonical role outcomes, and a failure contract.', ''];
    const workflows = new Map();
    for (const row of rows) {
      const key = row.category === 'mcp_tool' ? row.operation.split('_')[0] : row.category;
      if (!workflows.has(key)) workflows.set(key, []);
      workflows.get(key).push(row);
    }
    for (const [workflow, items] of workflows) {
      lines.push(`  Rule: ${display(workflow)} workflow`, '');
      for (const row of items) { claimed.add(row.row_id); lines.push(...surfaceScenarios(row)); }
    }
    const path = join(base, `${target}.feature`); await mkdir(join(path, '..'), { recursive: true }); await writeFile(path, featureText(lines));
  }
  if (claimed.size !== manifest.rows.length) {
    const missed = manifest.rows.filter(row => !claimed.has(row.row_id)).map(row => `${row.category}:${row.name}`);
    throw new Error(`surface groups claimed ${claimed.size}/${manifest.rows.length}: ${missed.join(', ')}`);
  }
}

await emitFunctional(join(root, 'testdata/parity/features/application'), appManifest, appGroups);
await emitSurfaces(join(root, 'testdata/parity/features/surfaces'), surfaceManifest);
await emitFunctional(join(root, 'umcp/testdata/features'), umcpManifest, umcpGroups, true);
console.log(JSON.stringify({ applicationGroups: Object.keys(appGroups).length, surfaceGroups: Object.keys(surfaceGroups).length, umcpGroups: Object.keys(umcpGroups).length }));
