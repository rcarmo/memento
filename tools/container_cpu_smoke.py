from __future__ import annotations

import json
import os
import resource

from memento.ffi import RustFfiLibrary
from memento.needle_ffi import NeedleFfiLibrary
from memento.router import CANONICAL_TRAINED_SHALLOW_TOOLS_JSON


def main() -> int:

    gte = RustFfiLibrary(os.environ["MEMENTO_FFI_LIBRARY"]).load_model(
        os.environ["MEMENTO_GTE_MODEL"]
    )
    try:
        embedding = gte.embed("DiskStation compatibility check")
        if len(embedding) != 384:
            raise RuntimeError(f"unexpected embedding size: {len(embedding)}")
    finally:
        gte.close()

    needle = NeedleFfiLibrary(os.environ["MEMENTO_NEEDLE_FFI_LIBRARY"]).load_router(
        os.environ["MEMENTO_NEEDLE_MODEL"],
        os.environ["MEMENTO_NEEDLE_TOKENIZER"],
    )
    try:
        output = needle.generate(
            "what revision is indexed",
            CANONICAL_TRAINED_SHALLOW_TOOLS_JSON,
        )
        calls = json.loads(output)
        if not isinstance(calls, list) or len(calls) != 1:
            raise RuntimeError(f"unexpected Needle output: {output}")
    finally:
        needle.close()

    rss_mib = resource.getrusage(resource.RUSAGE_SELF).ru_maxrss / 1024
    max_rss = os.environ.get("MEMENTO_SMOKE_MAX_RSS_MIB")
    if max_rss is not None and rss_mib > float(max_rss):
        raise RuntimeError(f"peak RSS {rss_mib:.1f} MiB exceeds {max_rss} MiB")
    print(
        json.dumps(
            {
                "embedding_dimensions": len(embedding),
                "needle_output": calls,
                "peak_rss_mib": round(rss_mib, 1),
            },
            sort_keys=True,
        )
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
