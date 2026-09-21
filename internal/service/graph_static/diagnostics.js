// Diagnostics retain their full-snapshot meaning, but the displayed targets
// must stay inside the user's declared selection/view scope.
export function scopedDiagnostics(graph, selected, visibleNodes) {
  const visible = new Set(visibleNodes.map(node => node.id));
  let ids;
  if (selected && !selected.member_count) ids = new Set([selected.id]);
  else if (selected?.member_count) {
    ids = new Set((graph?.memberships || []).filter(([, cluster]) => cluster === selected.id).map(([id]) => id));
    for (const node of visibleNodes) if (!node.member_count) ids.add(node.id);
  } else if (graph?.mode === "aggregated") {
    ids = new Set((graph.memberships || []).filter(([, cluster]) => visible.has(cluster)).map(([id]) => id));
  } else ids = visible;
  const seen = new Set();
  const result = [];
  for (const diagnostic of graph?.diagnostics || []) {
    if (seen.has(diagnostic.id)) continue;
    const targets = [...new Set(diagnostic.concept_ids || [])].filter(id => ids.has(id));
    if (!targets.length) continue;
    seen.add(diagnostic.id);
    result.push({ ...diagnostic, concept_ids: targets, scope_limited: diagnostic.scope_limited || targets.length !== (diagnostic.concept_ids || []).length });
  }
  return result;
}

export function diagnosticTargets(diagnostic, nodes) {
  const byId = new Map(nodes.map(node => [node.id, node]));
  return [...new Set(diagnostic.concept_ids || [])].map(id => ({
    id, label: byId.get(id)?.path || byId.get(id)?.title || `Open memory ${id}`,
  }));
}

export function relationshipSummary(detail) {
  if (detail?.loading) return "Loading relationships…";
  if (detail?.error || !Array.isArray(detail?.inbound) || !Array.isArray(detail?.outbound)) return "Relationships unavailable — retry loading this node.";
  const internal = edges => edges.filter(edge => edge.kind === "explicit" && edge.resolution === "resolved");
  const inbound = detail.node?.explicit_in_degree ?? new Set(internal(detail.inbound).map(edge => edge.source)).size;
  const outbound = detail.node?.explicit_out_degree ?? new Set(internal(detail.outbound).map(edge => edge.target)).size;
  const external = detail.outbound.filter(edge => edge.resolution === "external").length;
  const assets = detail.outbound.filter(edge => edge.resolution === "asset").length;
  const broken = detail.outbound.filter(edge => edge.resolution === "broken").length;
  return `${inbound} inbound / ${outbound} outbound concept links · ${external} external · ${assets} assets · ${broken} unresolved${detail.truncated ? " · list truncated" : ""}`;
}
