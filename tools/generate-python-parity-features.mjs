import { readFile, writeFile, rm, mkdir } from 'node:fs/promises';
import { join } from 'node:path';
import { existsSync } from 'node:fs';

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

const sentence = (value, limit = 700) => String(value || '').replace(/\s+/g, ' ').trim().slice(0, limit) || 'the captured Python behavior is exercised';
const display = value => value.replaceAll('-', ' ').replaceAll('_', ' ');
const featureText = lines => `${lines.map(line => line.trimEnd()).join('\n').trimEnd()}\n`;

function sourceRule(row) { return row.python_file || row.node_file.split('/').at(-1); }
function behaviorTitle(row) {
  return display(row.node_name.replace(/^test_/, '').replaceAll('.py', '').replaceAll('[', ' (').replaceAll(']', ')')).replace(/^./, value => value.toUpperCase());
}
function expectedOutcome(title) {
  const lower = title.toLowerCase();
  if (/reject|forbid|den(y|ies)|invalid|error|fail|guard/.test(lower)) return 'the request is rejected at the specified boundary and prohibited state is unchanged';
  if (/concurr|race|thread/.test(lower)) return 'the result and persisted state remain deterministic under concurrent execution';
  if (/cancel|timeout|deadline/.test(lower)) return 'cancellation or timeout preserves the specified completion and reconciliation state';
  if (/cursor|page|pagination|limit|bound/.test(lower)) return 'the bounded result and continuation state match the specified contract';
  if (/auth|role|scope|prefix|visible|protected/.test(lower)) return 'the authorization decision and visible result match the specified principal scope';
  if (/create|update|rename|delete|trash|restore|purge|apply|rotate|revoke|migrat|write/.test(lower)) return 'the response and durable state transition match the specified lifecycle';
  return 'the observable response, ordering, warnings and resulting state match the specified contract';
}
function functionalScenario(row) {
  const title = behaviorTitle(row);
  return [`    @${row.row_id}`, `    Scenario: ${title}`, '      Given the controlled domain state and principal described by this behavior', `      When the actor performs: ${title.toLowerCase()}`, `      Then ${expectedOutcome(title)}`, ''];
}
function upstreamScenario(row) {
  const title = behaviorTitle(row);
  return [`    @${row.row_id}`, `    Scenario: ${title}`, '      Given a configured MCP client, server and transport state', `      When the client performs: ${title.toLowerCase()}`, `      Then ${expectedOutcome(title)}`, ''];
}
function surfaceScenarios(row) {
  const required = row.required_arguments.length ? row.required_arguments.join(', ') : 'no required arguments';
  const optional = row.optional_arguments.length ? row.optional_arguments.join(', ') : 'no optional arguments';
  const defaults = Object.keys(row.defaults).length ? JSON.stringify(row.defaults) : 'no declared defaults';
  const fields = [...row.success_envelope, ...row.success_fields].join(', ');
  const lines = [
    `    @${row.row_id}`,
    `    Scenario: ${row.name} succeeds with its declared contract`,
    `      Given required arguments ${required}`,
    `      And optional arguments ${optional}`,
    `      And declared defaults ${defaults}`,
    `      And policy scope ${row.policy_scope}`,
    `      When an authorized client invokes ${row.name}`,
    `      Then the response exposes ${fields}`,
    `      And side effects are ${row.side_effects}`,
    `      And idempotency is ${row.idempotency}`,
    `      And pagination or range behavior is ${row.pagination}`,
    '',
  ];
  for (const profile of ['reader', 'proposer', 'curator', 'admin']) {
    lines.push(
      `    @${row.row_id}_role_${profile}`,
      `    Scenario: ${row.name} as ${profile}`,
      `      Given ${row.category === 'mcp_tool' ? 'the operation is present on the configured tool surface and ' : ''}the canonical ${profile} profile`,
      `      When that principal ${row.category === 'mcp_tool' ? 'lists and calls' : 'uses'} ${row.name}`,
      `      Then the outcome is ${row.profile_outcomes[profile]}`,
      '',
    );
  }
  lines.push(
    `    @${row.row_id}_failure`,
    `    Scenario: ${row.name} rejects invalid or conflicting requests`,
    `      Given required arguments ${required} and declared defaults ${defaults}`,
    `      And malformed, missing, out-of-scope, unavailable, replayed, or conflicting inputs`,
    `      When the client invokes ${row.name}`,
    `      Then one of the specified failures is ${row.error_contract.join('; ')}`,
    `      And failed pre-publication calls do not apply ${row.side_effects}`,
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
    const lines = [`Feature: ${display(target)}`, '', '  These scenarios describe observable behavior independently of its implementation.', '  Stable row tags link each scenario to versioned evidence and executable validation data.', ''];
    for (const file of files) {
      const items = rows.filter(row => sourceRule(row) === file);
      if (!items.length) throw new Error(`${file} has no rows in ${target}`);
      const rule = display(file.replace(/^test_/, '').replace(/\.py$/, '')).replace(/^./, value => value.toUpperCase());
      lines.push(`  Rule: ${rule}`, '');
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
    const lines = [`Feature: ${display(target)}`, '', '  Each operation has a success contract, four canonical role outcomes, and a failure contract.', '  Stable row tags link behavior to validation data without exposing implementation details.', ''];
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

function markdownCell(value) { return String(value ?? '').replaceAll('|', '\\|').replaceAll('\n', ' '); }
async function emitSurfaceMatrix(manifest) {
  const lines = [
    '# Python-to-Go public surface matrix',
    '',
    `Python reference: \`rcarmo/memento@${manifest.python_commit}\`.`,
    '',
    `Counts: ${Object.entries(manifest.counts).map(([key, value]) => `**${value} ${display(key)}**`).join(', ')}.`,
    '',
    '| Category | Python surface | Required/default arguments | Success fields | Side effects / idempotency / pagination | Reader | Proposer | Curator | Admin | Error contract | Executable validation data | Evidence | Go implementation and tests | Status |',
    '|---|---|---|---|---|---|---|---|---|---|---|---|---|---|',
  ];
  for (const row of manifest.rows) {
    const argumentsCell = `required: ${row.required_arguments.join(', ') || 'none'}; defaults: ${JSON.stringify(row.defaults)}`;
    const effects = `${row.side_effects}; idempotency: ${row.idempotency}; pagination/range: ${row.pagination}; policy: ${row.policy_scope}`;
    const evidence = row.behavior_provenance.join('; ');
    const go = `${row.go_source}; ${row.go_tests.map(item => item.name).join(', ')}`;
    const validation = `${row.validation_level}: ${row.validation_cases.join(', ')}`;
    const values = [row.category, row.name, argumentsCell, [...row.success_envelope, ...row.success_fields].join(', '), effects, row.profile_outcomes.reader, row.profile_outcomes.proposer, row.profile_outcomes.curator, row.profile_outcomes.admin, row.error_contract.join('; '), validation, evidence, go, row.status];
    lines.push(`| ${values.map(markdownCell).join(' | ')} |`);
  }
  await writeFile(join(root, 'docs/go-port/python-surface-matrix.md'), `${lines.join('\n')}\n`);
}

const domainSources = {
  'access': ['internal/access/authorization.go', 'internal/access/store.go'],
  'admin http': ['internal/service/admin_http.go'],
  'asset migration': ['internal/service/legacy_skill_migration.go'],
  'asset pack repository': ['internal/assets/accepted.go', 'internal/assets/pack.go', 'internal/service/worktree_assets.go'],
  'asset retrieval': ['internal/assets/retrieval.go', 'internal/service/asset_get.go'],
  'control plane': ['internal/control/proposals.go', 'internal/service/proposal_submit.go'],
  'cpu usage': ['internal/derived/search.go'],
  'derived plane': ['internal/derived/index.go', 'internal/derived/content.go'],
  'evidence': ['internal/service/answer_evidence.go'],
  'graph debug': ['internal/graphdebug/snapshot.go'], 'graph diagnostics': ['internal/graphdebug/diagnostics.go'], 'graph export': ['internal/graphdebug/export.go'], 'graph layout': ['internal/graphdebug/layout.go'], 'graph refresh': ['internal/graphdebug/refresh.go'], 'graph snapshot': ['internal/graphdebug/snapshot.go'], 'graph vendor': ['internal/service/graph_static.go'],
  'legacy blob migration': ['internal/service/legacy_skill_migration.go'], 'load harness': ['cmd/memento-benchmark-go/main.go'], 'memory answer': ['internal/service/answer_endpoint.go'], 'model transport': ['internal/service/model_client.go'],
  'needle corpus': ['internal/needle/model.go'], 'needle ffi': ['internal/needle/model.go'], 'needle router generator': ['internal/needle/generate.go'], 'operations': ['internal/control/operations.go', 'internal/service/operation_get.go'],
  'package': ['cmd/memento-go/main.go'], 'proposal assets': ['internal/service/worktree_assets.go', 'internal/service/proposal_prepare.go'], 'release deploy': ['cmd/memento-go/main.go'], 'repository core': ['internal/repository/bundle.go'],
  'router': ['internal/service/route_endpoint.go', 'internal/service/router.go'], 'runtime models': ['internal/service/runtime_models_off.go'], 'semantic': ['internal/derived/semantic_search.go'], 'semantic deferred': ['internal/derived/semantic_worker.go'],
  'service mcp': ['internal/service/configured_server.go', 'internal/service/proposal_tools.go'], 'skill import': ['internal/assets/skill_import.go'], 'skill packs': ['internal/assets/pack.go'], 'staged assets': ['internal/assets/staging.go', 'internal/service/staging_http.go'],
  'subprocess embeddings': ['internal/service/subprocess_semantic.go'], 'warm subprocess embeddings': ['internal/service/subprocess_semantic.go'], 'warm worker config': ['internal/service/runtime_semantic_config.go'], 'workflows': ['cmd/memento-go/main.go'],
};
function checkedSources(paths, label) {
  const unique = [...new Set(paths)];
  for (const path of unique) if (!existsSync(join(root, path)) || path.endsWith('_test.go')) throw new Error(`missing production source for ${label}: ${path}`);
  return unique;
}
function mappedProductionSources(row, fallback) {
  const output = [];
  for (const test of row.go_tests) {
    const candidate = test.file.replace(/_test\.go$/, '.go');
    if (existsSync(join(root, candidate))) output.push(candidate);
  }
  if (!output.length) output.push(...fallback);
  return checkedSources(output, row.row_id);
}
function umcpSources(row) { return mappedProductionSources(row, ['umcp/server.go']); }
function applicationSources(row) { return mappedProductionSources(row, domainSources[row.domain] || ['cmd/memento-go/main.go']); }
function surfaceBindings(row) {
  const common = { behavior_row_id: row.row_id, evidence_level: row.validation_level, validation_refs: row.validation_cases, go_tests: row.go_tests, go_sources: checkedSources(row.go_sources, row.row_id) };
  const output = [{ ...common, scenario_id: row.row_id, scenario_kind: 'success', expected: { fields: [...row.success_envelope, ...row.success_fields], side_effects: row.side_effects, idempotency: row.idempotency, pagination: row.pagination } }];
  for (const profile of ['reader', 'proposer', 'curator', 'admin']) output.push({ ...common, scenario_id: `${row.row_id}_role_${profile}`, scenario_kind: 'role', expected: { profile, outcome: row.profile_outcomes[profile] } });
  output.push({ ...common, scenario_id: `${row.row_id}_failure`, scenario_kind: 'failure', expected: { errors: row.error_contract, failed_side_effects: `does not apply: ${row.side_effects}` } });
  return output;
}
async function emitScenarioBindings() {
  const bindings = [];
  for (const row of appManifest.rows) bindings.push({ scenario_id: row.row_id, scenario_kind: 'application_behavior', behavior_row_id: row.row_id, evidence_level: row.status, python_test: row.python_test, python_source_sha256: row.python_source_sha256, validation_refs: [`python-row:${row.row_id}`], go_tests: row.go_tests, go_sources: applicationSources(row), expected: { summary: row.behavior_summary } });
  for (const row of umcpManifest.rows) bindings.push({ scenario_id: row.row_id, scenario_kind: 'umcp_behavior', behavior_row_id: row.row_id, evidence_level: row.status, python_test: row.python_test, python_source_sha256: row.python_source_sha256, validation_refs: [`umcp-row:${row.row_id}`], go_tests: row.go_tests, go_sources: umcpSources(row), expected: { summary: row.behavior_summary } });
  for (const row of surfaceManifest.rows) bindings.push(...surfaceBindings(row));
  const ids = new Set(); for (const binding of bindings) { if (ids.has(binding.scenario_id)) throw new Error(`duplicate scenario binding ${binding.scenario_id}`); ids.add(binding.scenario_id); }
  const output = { schema_version: 1, python_commit: surfaceManifest.python_commit, counts: { application: appManifest.rows.length, umcp: umcpManifest.rows.length, surface: surfaceManifest.rows.length * 6, total: bindings.length }, bindings };
  await writeFile(join(root, 'testdata/parity/gherkin-go-bindings.json'), `${JSON.stringify(output, null, 2)}\n`);
}

await emitFunctional(join(root, 'testdata/parity/features/application'), appManifest, appGroups);
await emitSurfaces(join(root, 'testdata/parity/features/surfaces'), surfaceManifest);
await emitFunctional(join(root, 'umcp/testdata/features'), umcpManifest, umcpGroups, true);
await emitSurfaceMatrix(surfaceManifest);
await emitScenarioBindings();
console.log(JSON.stringify({ applicationGroups: Object.keys(appGroups).length, surfaceGroups: Object.keys(surfaceGroups).length, umcpGroups: Object.keys(umcpGroups).length, scenarioBindings: appManifest.rows.length + umcpManifest.rows.length + surfaceManifest.rows.length * 6 }));
