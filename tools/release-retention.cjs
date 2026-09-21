// Registry manifests are retained until dependency-aware GC is available.
// A package-version age/tag count is not a manifest reachability calculation.
const protectedTags = ['v0.5.9', 'v1.0.2']; // recovery baseline and live predecessor

function retentionPlan(releases, runs, keep = 5, protectedRefs = protectedTags) {
  const application = releases.filter(r => /^v\d+\.\d+\.\d+(?:[-+].*)?$/.test(r.tag_name));
  application.sort((a, b) => Date.parse(b.created_at) - Date.parse(a.created_at));
  const kept = application.filter((r, i) => i < keep || protectedRefs.includes(r.tag_name));
  const tags = new Set([...protectedRefs, ...kept.map(r => r.tag_name)]);
  const removed = application.filter(r => !tags.has(r.tag_name));
  // Never delete runs merely because failed/manual attempts are newer. Runs
  // are removable only when tied unambiguously to a removed release tag.
  const doomedTags = new Set(removed.map(r => r.tag_name));
  const removedRuns = runs.filter(r => r.event === 'push' && r.status === 'completed' && doomedTags.has(r.head_branch) && !tags.has(r.head_branch));
  return { releaseIDs: removed.map(r => r.id), tags: removed.map(r => r.tag_name), runIDs: removedRuns.map(r => r.id), packageVersionIDs: [] };
}
module.exports = { retentionPlan, protectedTags };
