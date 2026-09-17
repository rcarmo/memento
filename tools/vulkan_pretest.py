"""Bounded real-GTE1 CPU/Vulkan parity and cold-worker benchmark; no service state."""

from __future__ import annotations

import argparse
import hashlib
import json
import math
import platform
import statistics
import struct
import subprocess
import time
from pathlib import Path
from typing import Any


def invoke(
    worker: Path, model: Path, backend: str, device: str, texts: list[str]
) -> dict[str, Any]:
    request = json.dumps({"method": "embed_batch", "id": "pretest", "texts": texts}).encode()
    command = [str(worker.resolve()), str(model.resolve()), backend]
    if device:
        command.append(device)
    start = time.monotonic()
    result = subprocess.run(
        command,
        input=struct.pack("<I", len(request)) + request,
        capture_output=True,
        timeout=120,
        check=False,
    )
    elapsed = time.monotonic() - start
    if result.returncode:
        raise RuntimeError(
            f"{backend} failed ({result.returncode}): {result.stderr.decode(errors='replace')[:1000]}"
        )
    data = result.stdout
    if len(data) < 8:
        raise RuntimeError("truncated worker frame")
    total, length = struct.unpack_from("<II", data)
    if total + 4 != len(data) or length > total - 4:
        raise RuntimeError("invalid worker frame")
    header = json.loads(data[8 : 8 + length])
    if not header.get("ok") or header.get("dimensions") != 384 or header.get("count") != len(texts):
        raise RuntimeError(f"invalid embedding response: {header}")
    raw = data[8 + length :]
    if len(raw) != len(texts) * 384 * 4:
        raise RuntimeError("invalid embedding payload size")
    values = struct.unpack(f"<{len(texts) * 384}f", raw)
    if not all(math.isfinite(v) for v in values):
        raise RuntimeError("nonfinite embedding")
    if backend == "vulkan" and header.get("backend", {}).get("selected") != "vulkan":
        raise RuntimeError("hardware test fell back to CPU")
    return {
        "seconds": elapsed,
        "backend": header.get("backend"),
        "vectors": [values[i * 384 : (i + 1) * 384] for i in range(len(texts))],
    }


def compare(left: list[tuple[float, ...]], right: list[tuple[float, ...]]) -> dict[str, float]:
    cosines, errors = [], []
    for a, b in zip(left, right, strict=True):
        aa, bb = sum(x * x for x in a), sum(y * y for y in b)
        if not (0.999 < aa < 1.001 and 0.999 < bb < 1.001):
            raise RuntimeError("embedding norm outside unit tolerance")
        cosines.append(sum(x * y for x, y in zip(a, b, strict=True)) / math.sqrt(aa * bb))
        errors.append(max(abs(x - y) for x, y in zip(a, b, strict=True)))
    return {"minimum_cosine": min(cosines), "max_abs_error": max(errors)}


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--model", required=True, type=Path)
    parser.add_argument("--worker", required=True, type=Path)
    parser.add_argument("--device", default="")
    parser.add_argument("--output", required=True, type=Path)
    parser.add_argument("--repeats", type=int, default=3)
    args = parser.parse_args()
    if not 1 <= args.repeats <= 5:
        parser.error("repeats must be 1..5")
    cases = {
        "short": ["Shared durable memory and reviewed proposals."],
        "unicode-batch": ["", "Read the network service record.", "Olá mundo! 東京 — café."],
        "medium": ["The project uses semantic embeddings to retrieve shared knowledge. " * 10],
        "token-limit": ["Memory namespace proposal review asset archive. " * 180],
    }
    report: dict[str, Any] = {
        "platform": platform.platform(),
        "model": args.model.name,
        "model_sha256": hashlib.sha256(args.model.read_bytes()).hexdigest(),
        "worker_sha256": hashlib.sha256(args.worker.read_bytes()).hexdigest(),
        "requested_device": args.device,
        "repeats": args.repeats,
        "cases": [],
        "passed": True,
        "timing_scope": "cold subprocess including model load, adapter/pipelines and self-test",
    }
    for name, texts in cases.items():
        samples = []
        for _ in range(args.repeats):
            cpu = invoke(args.worker, args.model, "cpu", "", texts)
            gpu = invoke(args.worker, args.model, "vulkan", args.device, texts)
            parity = compare(cpu["vectors"], gpu["vectors"])
            passed = parity["minimum_cosine"] >= 0.99999 and parity["max_abs_error"] <= 0.001
            report["passed"] &= passed
            samples.append(
                {
                    "cpu_seconds": cpu["seconds"],
                    "vulkan_seconds": gpu["seconds"],
                    "backend": gpu["backend"],
                    "parity": parity,
                    "passed": passed,
                }
            )
        report["cases"].append(
            {
                "name": name,
                "batch": len(texts),
                "samples": samples,
                "median_cpu_seconds": statistics.median(s["cpu_seconds"] for s in samples),
                "median_vulkan_seconds": statistics.median(s["vulkan_seconds"] for s in samples),
            }
        )
    fallback = invoke(args.worker, args.model, "auto", "NONEXISTENT-GPU-MEMENTO", cases["short"])
    if fallback["backend"]["selected"] != "cpu" or not fallback["backend"]["fallback_reason"]:
        raise RuntimeError("missing-device auto fallback failed")
    report["missing_device_fallback"] = fallback["backend"]
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(json.dumps(report, indent=2) + "\n")
    print(json.dumps({"passed": report["passed"], "output": str(args.output), "cases": len(cases)}))
    if not report["passed"]:
        raise SystemExit(1)


if __name__ == "__main__":
    main()
