from __future__ import annotations

import argparse
import hashlib
import json
import pickle
import struct
from collections.abc import Iterable, Mapping
from dataclasses import dataclass
from pathlib import Path
from typing import Final

import numpy as np
from sentencepiece import sentencepiece_model_pb2

MAGIC: Final[bytes] = b"NDL1"
VERSION: Final[int] = 1
SECTION_KINDS: Final[tuple[bytes, ...]] = (b"CONF", b"TOKN", b"META", b"TDIR", b"DATA")
DTYPE_BF16: Final[int] = 1


@dataclass(frozen=True, slots=True)
class TensorRecord:
    name: str
    shape: tuple[int, ...]
    data: bytes
    sha256: str

    @property
    def byte_len(self) -> int:
        return len(self.data)


@dataclass(frozen=True, slots=True)
class SectionRecord:
    kind: bytes
    payload: bytes

    @property
    def sha256(self) -> str:
        return hashlib.sha256(self.payload).hexdigest()


@dataclass(frozen=True, slots=True)
class OutputManifest:
    output_sha256: str
    output_size: int
    config_sha256: str
    tokenizer_sha256: str
    metadata_sha256: str
    tensor_directory_sha256: str
    tensor_data_sha256: str
    tensor_count: int
    tokenizer_piece_count: int

    def to_json(self) -> str:
        return canonical_json(
            {
                "config_sha256": self.config_sha256,
                "metadata_sha256": self.metadata_sha256,
                "output_sha256": self.output_sha256,
                "output_size": self.output_size,
                "tensor_count": self.tensor_count,
                "tensor_data_sha256": self.tensor_data_sha256,
                "tensor_directory_sha256": self.tensor_directory_sha256,
                "tokenizer_piece_count": self.tokenizer_piece_count,
                "tokenizer_sha256": self.tokenizer_sha256,
            }
        )


class ConversionError(RuntimeError):
    pass


class _RestrictedUnpickler(pickle.Unpickler):
    _ALLOWED_GLOBALS: Final[dict[tuple[str, str], object]] = {
        ("numpy._core.multiarray", "_reconstruct"): np.core.multiarray._reconstruct,
        ("numpy", "ndarray"): np.ndarray,
        ("numpy", "dtype"): np.dtype,
        ("numpy._core.numeric", "_frombuffer"): np.core.numeric._frombuffer,
    }

    def find_class(self, module: str, name: str) -> object:
        try:
            return self._ALLOWED_GLOBALS[(module, name)]
        except KeyError as exc:
            raise ConversionError(f"unsupported pickle global: {module}.{name}") from exc


def canonical_json(value: object) -> str:
    return json.dumps(value, sort_keys=True, separators=(",", ":"), ensure_ascii=False)


def sha256_hex(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        for block in iter(lambda: handle.read(1 << 20), b""):
            digest.update(block)
    return digest.hexdigest()


def flatten_tensors(prefix: str, value: object) -> Iterable[tuple[str, np.ndarray]]:
    if isinstance(value, np.ndarray):
        yield prefix, value
        return
    if isinstance(value, Mapping):
        for key in sorted(value):
            next_prefix = f"{prefix}.{key}" if prefix else str(key)
            yield from flatten_tensors(next_prefix, value[key])
        return
    raise ConversionError(f"unsupported tensor tree value at {prefix!r}: {type(value)!r}")


def float32_to_bf16_le_bytes(values: np.ndarray) -> bytes:
    float32 = np.asarray(values, dtype=np.float32)
    bits = float32.view(np.uint32)
    rounding_bias = np.uint32(0x7FFF) + ((bits >> 16) & np.uint32(1))
    bf16 = ((bits + rounding_bias) >> 16).astype(np.uint16)
    return bf16.astype("<u2", copy=False).tobytes()


def load_checkpoint(path: Path) -> tuple[dict[str, object], dict[str, object]]:
    with path.open("rb") as handle:
        payload = _RestrictedUnpickler(handle).load()
    if not isinstance(payload, dict):
        raise ConversionError(f"checkpoint root must be dict, got {type(payload)!r}")
    params = payload.get("params")
    config = payload.get("config")
    if not isinstance(params, dict):
        raise ConversionError("checkpoint params must be a dict")
    if not isinstance(config, dict):
        raise ConversionError("checkpoint config must be a dict")
    return params, config


def build_tokenizer_section(tokenizer_model_path: Path) -> tuple[bytes, int]:
    proto = sentencepiece_model_pb2.ModelProto()
    proto.ParseFromString(tokenizer_model_path.read_bytes())
    pieces = []
    for piece in proto.pieces:
        raw = piece.piece.encode("utf-8")
        pieces.append(
            struct.pack("<I", len(raw))
            + raw
            + struct.pack("<Bf", int(piece.type), float(piece.score))
        )
    return struct.pack("<I", len(proto.pieces)) + b"".join(pieces), len(proto.pieces)


def build_tensor_records(params: dict[str, object]) -> list[TensorRecord]:
    tensors: list[TensorRecord] = []
    for name, array in flatten_tensors("", params):
        if not isinstance(array, np.ndarray):
            raise ConversionError(f"tensor {name} is not an ndarray")
        bf16_data = float32_to_bf16_le_bytes(array)
        tensors.append(
            TensorRecord(
                name=name,
                shape=tuple(int(dim) for dim in array.shape),
                data=bf16_data,
                sha256=hashlib.sha256(bf16_data).hexdigest(),
            )
        )
    return tensors


def build_tensor_sections(tensors: list[TensorRecord]) -> tuple[bytes, bytes]:
    directory_chunks: list[bytes] = [struct.pack("<I", len(tensors))]
    data_chunks: list[bytes] = []
    data_offset = 0
    for tensor in tensors:
        name = tensor.name.encode("utf-8")
        directory_chunks.append(
            struct.pack("<I", len(name))
            + name
            + struct.pack("<BB", DTYPE_BF16, len(tensor.shape))
            + b"".join(struct.pack("<I", dim) for dim in tensor.shape)
            + struct.pack("<QQ", data_offset, tensor.byte_len)
            + bytes.fromhex(tensor.sha256)
        )
        data_chunks.append(tensor.data)
        data_offset += tensor.byte_len
    return b"".join(directory_chunks), b"".join(data_chunks)


def build_sections(
    checkpoint_path: Path,
    tokenizer_model_path: Path,
) -> tuple[list[SectionRecord], OutputManifest, dict[str, object]]:
    params, config = load_checkpoint(checkpoint_path)
    tensors = build_tensor_records(params)
    tokenizer_section, tokenizer_piece_count = build_tokenizer_section(tokenizer_model_path)
    tensor_directory, tensor_data = build_tensor_sections(tensors)

    source = {
        "checkpoint": {
            "name": checkpoint_path.name,
            "sha256": sha256_hex(checkpoint_path),
        },
        "tokenizer_model": {
            "name": tokenizer_model_path.name,
            "sha256": sha256_hex(tokenizer_model_path),
        },
    }
    config_payload = canonical_json(config).encode("utf-8")
    metadata_payload = canonical_json(
        {
            "converter": {
                "format": "NDL1",
                "tool": "tools/experiments/needle/convert_needle.py",
                "version": VERSION,
            },
            "section_hashes": {
                "config_sha256": hashlib.sha256(config_payload).hexdigest(),
                "tensor_data_sha256": hashlib.sha256(tensor_data).hexdigest(),
                "tensor_directory_sha256": hashlib.sha256(tensor_directory).hexdigest(),
                "tokenizer_sha256": hashlib.sha256(tokenizer_section).hexdigest(),
            },
            "source": source,
            "tensor_count": len(tensors),
            "tokenizer_piece_count": tokenizer_piece_count,
        }
    ).encode("utf-8")

    sections = [
        SectionRecord(b"CONF", config_payload),
        SectionRecord(b"TOKN", tokenizer_section),
        SectionRecord(b"META", metadata_payload),
        SectionRecord(b"TDIR", tensor_directory),
        SectionRecord(b"DATA", tensor_data),
    ]
    manifest_payload = {
        "config": config,
        "format": "NDL1",
        "source": source,
        "tensor_count": len(tensors),
        "tensor_names": [tensor.name for tensor in tensors],
        "tokenizer_piece_count": tokenizer_piece_count,
    }
    manifest = OutputManifest(
        output_sha256="",
        output_size=0,
        config_sha256=sections[0].sha256,
        tokenizer_sha256=sections[1].sha256,
        metadata_sha256=sections[2].sha256,
        tensor_directory_sha256=sections[3].sha256,
        tensor_data_sha256=sections[4].sha256,
        tensor_count=len(tensors),
        tokenizer_piece_count=tokenizer_piece_count,
    )
    return sections, manifest, manifest_payload


def encode_file(sections: list[SectionRecord]) -> bytes:
    kinds = tuple(section.kind for section in sections)
    if kinds != SECTION_KINDS:
        raise ConversionError(f"unexpected section order: {kinds!r}")
    header_len = 12 + len(sections) * (4 + 8 + 8 + 32)
    payload_offset = header_len
    descriptors: list[bytes] = []
    payloads: list[bytes] = []
    for section in sections:
        payload = section.payload
        descriptors.append(
            section.kind
            + struct.pack("<QQ", payload_offset, len(payload))
            + bytes.fromhex(section.sha256)
        )
        payloads.append(payload)
        payload_offset += len(payload)
    return (
        MAGIC
        + struct.pack("<HHI", VERSION, len(sections), 0)
        + b"".join(descriptors)
        + b"".join(payloads)
    )


def write_output(path: Path, data: bytes | str) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    if isinstance(data, bytes):
        path.write_bytes(data)
    else:
        path.write_text(data, encoding="utf-8")


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Convert a Needle pickle checkpoint into NDL1")
    parser.add_argument(
        "tokenizer_model",
        type=Path,
        help="Path to the upstream SentencePiece tokenizer model (for example tokenizer.model)",
    )
    parser.add_argument(
        "--input",
        type=Path,
        default=Path("models/needle/memento-router.pkl"),
        help="Input Needle pickle checkpoint",
    )
    parser.add_argument(
        "--output",
        type=Path,
        default=Path("models/needle/memento-router.ndl"),
        help="Output NDL1 file path",
    )
    parser.add_argument(
        "--manifest",
        type=Path,
        default=None,
        help="Optional JSON manifest output path",
    )
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    sections, manifest, manifest_payload = build_sections(args.input, args.tokenizer_model)
    binary = encode_file(sections)
    write_output(args.output, binary)

    completed_manifest = OutputManifest(
        output_sha256=hashlib.sha256(binary).hexdigest(),
        output_size=len(binary),
        config_sha256=manifest.config_sha256,
        tokenizer_sha256=manifest.tokenizer_sha256,
        metadata_sha256=manifest.metadata_sha256,
        tensor_directory_sha256=manifest.tensor_directory_sha256,
        tensor_data_sha256=manifest.tensor_data_sha256,
        tensor_count=manifest.tensor_count,
        tokenizer_piece_count=manifest.tokenizer_piece_count,
    )
    if args.manifest is not None:
        manifest_payload.update(json.loads(completed_manifest.to_json()))
        write_output(args.manifest, canonical_json(manifest_payload) + "\n")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
