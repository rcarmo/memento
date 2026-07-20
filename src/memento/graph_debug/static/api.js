const prefix = document.documentElement.dataset.graphPrefix || "/graph";

async function request(path, options = {}) {
  const started = performance.now();
  const response = await fetch(`${prefix}/api/v1${path}`, {
    cache: "no-store",
    headers: { "Content-Type": "application/json", ...(options.headers || {}) },
    ...options,
  });
  const text = await response.text();
  let payload = {};
  try { payload = text ? JSON.parse(text) : {}; } catch { throw new Error(`Invalid graph response (${response.status})`); }
  if (!response.ok) throw new Error(payload.error || `Graph request failed (${response.status})`);
  return { payload, elapsed: performance.now() - started, bytes: text.length };
}

async function download(path, payload) {
  const response = await fetch(`${prefix}/api/v1${path}`, {
    method: "POST",
    cache: "no-store",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
  });
  if (!response.ok) {
    let message = `Graph export failed (${response.status})`;
    try { message = (await response.json()).error || message; } catch {}
    throw new Error(message);
  }
  return response.blob();
}

export const graphApi = {
  status: () => request("/status"),
  overview: () => request("/overview"),
  detail: (id) => request(`/memories/${encodeURIComponent(id)}`),
  neighbourhood: (id) => request(`/neighbourhood/${encodeURIComponent(id)}`),
  cluster: (id) => request(`/clusters/${encodeURIComponent(id)}`),
  refreshStatus: () => request("/embeddings/status"),
  refresh: (scope, conceptIds = [], confirmFull = false) => request("/embeddings/refresh", {
    method: "POST",
    body: JSON.stringify({ scope, concept_ids: conceptIds, confirm_full: confirmFull }),
  }),
  exportSvg: (conceptIds, settings = {}) => download("/export/svg", { concept_ids: conceptIds, settings }),
  exportJson: (conceptIds, settings = {}) => download("/export/json", { concept_ids: conceptIds, settings }),
};
