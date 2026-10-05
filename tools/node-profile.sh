#!/usr/bin/env bash
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/project-env.sh" || exit 1
set -euo pipefail
mkdir -p "$PROFILE_ROOT/node"
out=$(mktemp -d "$PROFILE_ROOT/node/run-$(date -u +%Y%m%dT%H%M%SZ)-XXXXXX")
node --version > "$out/metadata.txt"
printf 'command: node ' >> "$out/metadata.txt"; printf '%q ' "$@" >> "$out/metadata.txt"; printf '\nCPU default1000us; heap sampling524288 bytes\n' >> "$out/metadata.txt"
git -C "$MEMENTO_REPO_ROOT" rev-parse HEAD >> "$out/metadata.txt"
git -C "$MEMENTO_REPO_ROOT" diff HEAD > "$out/worktree.patch"
# Preserve matching JS sources, including untracked wrappers, outside clean scope.
mkdir -p "$out/source"
mkdir -p "$out/source/browser"
find "$MEMENTO_REPO_ROOT/tools/browser" -maxdepth 1 -type f -exec cp {} "$out/source/browser/" \;
cp "$MEMENTO_REPO_ROOT"/tools/*.mjs "$out/source/"
status=0
node --cpu-prof --heap-prof --cpu-prof-dir="$out" --heap-prof-dir="$out" "$@" 2>&1 | tee "$out/test.log" || status=$?
node "$MEMENTO_REPO_ROOT/tools/node-profile-report.mjs" "$out" > "$out/analysis.txt"
cat "$out/analysis.txt"
exit "$status"
