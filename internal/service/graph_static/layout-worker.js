let generation = 0;

self.onmessage = ({ data }) => {
  if (data?.type === "stop") { generation++; return; }
  if (data?.type !== "layout") return;
  const job = ++generation;
  const { nodes, edges, forces = {}, layoutId } = data;
  const positions = nodes.map((node) => ({ id: node.id, x: 0, y: 0, z: 0, ...node.coarse_position }));
  const index = new Map(positions.map((node, i) => [node.id, i]));
  const velocity = positions.map(() => ({ x: 0, y: 0, z: 0 }));
  const links = edges.map((edge) => ({ ...edge, si: index.get(edge.source), ti: index.get(edge.target) }))
    .filter((edge) => edge.si != null && edge.ti != null && edge.si !== edge.ti);
  const degrees = new Map();
  links.forEach(edge => {
    const kind=edge.kind||"explicit";
    if(!degrees.has(kind))degrees.set(kind,positions.map(()=>0));
    if((forces[kind]??0.06)>0){degrees.get(kind)[edge.si]++;degrees.get(kind)[edge.ti]++;}
  });
  let quietSteps = 0;
  let step = 0;

  function tick() {
    if (job !== generation) return;
    let movement = 0;
    // Yield between small batches so controls and cancellation take effect promptly.
    for (let batch = 0; batch < 3; batch++, step++) {
      for (const edge of links) {
        const a = positions[edge.si], b = positions[edge.ti];
        const dx = b.x - a.x, dy = b.y - a.y, dz = b.z - a.z;
        const distance = Math.hypot(dx, dy, dz) || 0.001;
        const kind = edge.kind || "explicit";
        const degree=degrees.get(kind);
        const strength = Math.max(0, forces[kind] ?? 0.06) / Math.sqrt(Math.max(degree[edge.si], degree[edge.ti], 1));
        const desired = (forces.distance ?? 3) * (kind === "semantic_similarity" ? 0.8 : kind === "explicit" ? 1 : 1.2);
        const pull = (distance - desired) * strength / distance;
        velocity[edge.si].x += dx * pull; velocity[edge.si].y += dy * pull; velocity[edge.si].z += dz * pull;
        velocity[edge.ti].x -= dx * pull; velocity[edge.ti].y -= dy * pull; velocity[edge.ti].z -= dz * pull;
      }
      const stride = positions.length > 500 ? Math.max(1, Math.floor(positions.length / 40)) : 1;
      for (let i = 0; i < positions.length; i++) {
        for (let j = i + 1; j < positions.length; j += stride) {
          const a = positions[i], b = positions[j];
          let dx = b.x - a.x, dy = b.y - a.y, dz = b.z - a.z;
          if (Math.hypot(dx, dy, dz) < 0.001) {
            dx = Math.sin(i + j + 1) * 0.01; dy = Math.cos(i + j + 1) * 0.01;
          }
          const d2 = dx * dx + dy * dy + dz * dz + 0.05;
          if (d2 > 256) continue;
          const push = (forces.repulsion ?? 0.12) / (d2 * Math.sqrt(d2));
          velocity[i].x -= dx * push; velocity[i].y -= dy * push; velocity[i].z -= dz * push;
          velocity[j].x += dx * push; velocity[j].y += dy * push; velocity[j].z += dz * push;
        }
      }
      movement = 0;
      positions.forEach((p, i) => {
        const v = velocity[i];
        const speed = Math.hypot(v.x, v.y, v.z);
        if (speed > 0.3) { const scale = 0.3 / speed; v.x *= scale; v.y *= scale; v.z *= scale; }
        p.x += v.x; p.y += v.y; p.z += v.z;
        movement = Math.max(movement, Math.hypot(v.x, v.y, v.z));
        v.x *= 0.65; v.y *= 0.65; v.z *= 0.65;
      });
      quietSteps = movement < 0.001 ? quietSteps + 1 : 0;
    }
    const settled = quietSteps >= 30;
    self.postMessage({ type: "positions", layoutId, positions, settled, movement, step, progress: settled ? 1 : 0 });
    if (!settled) setTimeout(tick, 16);
  }
  tick();
};
