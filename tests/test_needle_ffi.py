from __future__ import annotations

import subprocess
import sys
from pathlib import Path

import pytest

from memento.config import IntelligentTiersConfig
from memento.needle_ffi import (
    NeedleFfiBoundsError,
    NeedleFfiCancelledError,
    NeedleFfiClosedError,
    NeedleFfiLibrary,
    NeedleFfiModelError,
)
from memento.router import CANONICAL_TRAINED_SHALLOW_TOOLS_JSON, parse_needle_router_output

DEFAULT_TOOLS_JSON = (
    "["
    '{"name":"memory_search","parameters":{"query":{"type":"string"},"limit":{"type":"integer"},"search_mode":{"type":"string"}}},'
    '{"name":"memory_read","parameters":{"id_or_path":{"type":"string"}}},'
    '{"name":"memory_status","parameters":{"field":{"type":"string"}}},'
    '{"name":"memory_execute","parameters":{"plan":{"type":"string"}}}'
    "]"
)


def build_rust_cdylib(package: str, stem: str) -> Path:
    project_root = Path(__file__).resolve().parents[1]
    rust_dir = project_root / "rust"
    target_dir = rust_dir / "target" / "debug"
    suffix = ".dylib" if sys.platform == "darwin" else ".dll" if sys.platform == "win32" else ".so"
    library_path = target_dir / f"{stem}{suffix}"
    subprocess.run(["cargo", "build", "-p", package], cwd=rust_dir, check=True)
    return library_path


def vendored_model_paths() -> tuple[Path, Path]:
    project_root = Path(__file__).resolve().parents[1]
    return (
        project_root / "models" / "needle" / "memento-router.ndl",
        project_root / "models" / "needle" / "needle.model",
    )


def require_vendored_model() -> tuple[Path, Path]:
    model_path, tokenizer_path = vendored_model_paths()
    for path in (model_path, tokenizer_path):
        if not path.exists():
            pytest.skip(f"missing vendored Needle artifact: {path}")
        data = path.read_bytes()
        if data.startswith(b"version https://git-lfs.github.com/spec/v1\n"):
            pytest.skip(f"git lfs artifact not fetched: {path}")
    return model_path, tokenizer_path


def test_needle_router_config_defaults_are_disabled_with_default_paths() -> None:
    config = IntelligentTiersConfig()
    assert config.needle_router.enabled is False
    assert config.needle_router.ffi_library_path.endswith("libmemento_needle_ffi.so")
    assert config.needle_router.model_path.endswith("memento-router.ndl")
    assert config.needle_router.tokenizer_path.endswith("needle.model")


def test_real_ffi_router_output_parses_to_one_action() -> None:
    model_path, tokenizer_path = require_vendored_model()
    ffi_library_path = build_rust_cdylib("memento-needle-ffi", "libmemento_needle_ffi")
    library = NeedleFfiLibrary(ffi_library_path)
    with library.load_router(model_path, tokenizer_path) as router:
        parsed = parse_needle_router_output(
            router.generate("find Piclaw", CANONICAL_TRAINED_SHALLOW_TOOLS_JSON)
        )
        assert parsed.action in {
            "search_then_read",
            "search_paths",
            "status_field",
            "search_then_graph",
            "read_field",
            "UNKNOWN",
        }


def test_python_ctypes_wrapper_router_lifecycle_generate_cancel_and_errors() -> None:
    model_path, tokenizer_path = require_vendored_model()
    ffi_library_path = build_rust_cdylib("memento-needle-ffi", "libmemento_needle_ffi")
    library = NeedleFfiLibrary(ffi_library_path)

    with library.new_cancel_token() as token:
        assert token.pointer is not None
        token.cancel()

    with library.load_router(model_path, tokenizer_path) as router:
        info = router.info()
        assert info.abi_version == 1
        assert info.d_model == 512
        assert info.vocab_size == 8192
        result = router.generate("Find Piclaw", DEFAULT_TOOLS_JSON)
        assert "memory_search" in result
        assert "Piclaw" in result
        with pytest.raises(NeedleFfiCancelledError):
            router.generate("Find Piclaw", DEFAULT_TOOLS_JSON, cancelled=lambda: True)
        with pytest.raises(NeedleFfiBoundsError):
            router.generate("Find Piclaw", DEFAULT_TOOLS_JSON, max_gen_len=0)

    closed_router = library.load_router(model_path, tokenizer_path)
    closed_router.close()
    with pytest.raises(NeedleFfiClosedError):
        closed_router.info()
    with pytest.raises(NeedleFfiClosedError):
        closed_router.generate("Find Piclaw", DEFAULT_TOOLS_JSON)

    with pytest.raises(NeedleFfiModelError):
        library.load_router(model_path, model_path)
