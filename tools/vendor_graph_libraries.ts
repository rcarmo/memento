#!/usr/bin/env bun

import { createHash } from "node:crypto";
import { readFile, writeFile } from "node:fs/promises";
import { basename, dirname, join } from "node:path";

const root = join(import.meta.dir, "..", "src", "memento", "graph_debug", "static", "vendor");
const manifestPath = join(root, "manifest.json");
const check = process.argv.includes("--check");
const manifest = JSON.parse(await readFile(manifestPath, "utf8"));

async function sha256(bytes: Uint8Array): Promise<string> {
  return createHash("sha256").update(bytes).digest("hex");
}

for (const library of manifest.libraries) {
  const target = join(root, library.file);
  if (check) {
    const actual = await sha256(await readFile(target));
    if (actual !== library.sha256) throw new Error(`${library.file}: expected ${library.sha256}, got ${actual}`);
    continue;
  }
  const response = await fetch(library.source, { redirect: "follow" });
  if (!response.ok) throw new Error(`${library.source}: HTTP ${response.status}`);
  const bytes = new Uint8Array(await response.arrayBuffer());
  const actual = await sha256(bytes);
  if (actual !== library.sha256) throw new Error(`${library.file}: expected ${library.sha256}, got ${actual}`);
  await writeFile(target, bytes);
  console.log(`updated ${basename(target)} (${bytes.byteLength} bytes)`);
}

console.log(check ? "graph vendor integrity OK" : "graph vendor files updated");
