let running = false;
self.onmessage = (event) => {
  if (event.data?.type === "stop") { running = false; return; }
  if (event.data?.type !== "layout") return;
  running = true;
  const { nodes, edges, forces, iterations = 90 } = event.data;
  const positions = nodes.map((node) => ({ id: node.id, ...node.coarse_position }));
  const index = new Map(positions.map((item, i) => [item.id, i]));
  const velocity = positions.map(() => ({ x: 0, y: 0, z: 0 }));
  for (let step = 0; step < iterations && running; step++) {
    const alpha = 1 - step / iterations;
    for (const edge of edges) {
      const si = index.get(edge.source), ti = index.get(edge.target);
      if (si == null || ti == null) continue;
      const a = positions[si], b = positions[ti];
      const dx = b.x - a.x, dy = b.y - a.y, dz = b.z - a.z;
      const distance = Math.hypot(dx, dy, dz) || 0.001;
      const kind = edge.kind || "explicit";
      const strength = (forces[kind] ?? forces.explicit ?? 0.08) * alpha;
      const desired = kind === "semantic_similarity" ? 1.8 : 2.6;
      const pull = (distance - desired) * strength / distance;
      velocity[si].x += dx * pull; velocity[si].y += dy * pull; velocity[si].z += dz * pull;
      velocity[ti].x -= dx * pull; velocity[ti].y -= dy * pull; velocity[ti].z -= dz * pull;
    }
    const stride = positions.length > 500 ? Math.max(1, Math.floor(positions.length / 28)) : 1;
    for (let i = 0; i < positions.length; i++) {
      for (let j = i + 1; j < positions.length; j += stride) {
        const a = positions[i], b = positions[j];
        const dx = b.x - a.x, dy = b.y - a.y, dz = b.z - a.z;
        const d2 = dx * dx + dy * dy + dz * dz + 0.05;
        if (d2 > 16) continue;
        const push = 0.004 * alpha / d2;
        velocity[i].x -= dx * push; velocity[i].y -= dy * push; velocity[i].z -= dz * push;
        velocity[j].x += dx * push; velocity[j].y += dy * push; velocity[j].z += dz * push;
      }
      const p = positions[i], v = velocity[i];
      p.x += v.x; p.y += v.y; p.z += v.z;
      v.x *= 0.72; v.y *= 0.72; v.z *= 0.72;
    }
    if (step % 15 === 0 && positions.length <= 500) self.postMessage({ type: "positions", positions, progress: step / iterations });
  }
  self.postMessage({ type: "positions", positions, progress: 1 });
};
