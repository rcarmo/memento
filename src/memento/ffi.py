from __future__ import annotations

import ctypes
from collections.abc import Callable, Sequence
from dataclasses import dataclass
from pathlib import Path

from memento.semantic import EmbeddingClient, EmbeddingModelInfo, validate_embedding

ABI_VERSION = 1
FEATURE_EMBED = 1 << 0
FEATURE_EMBED_BATCH = 1 << 1
FEATURE_CANCELLATION = 1 << 2
FEATURE_VECTOR_COSINE = 1 << 3
FEATURE_VECTOR_VALIDATE = 1 << 4
REQUIRED_FEATURES = FEATURE_EMBED | FEATURE_EMBED_BATCH | FEATURE_CANCELLATION


class FfiError(RuntimeError):
    pass


class FfiAbiError(FfiError):
    pass


class FfiNullError(FfiError):
    pass


class FfiBoundsError(FfiError):
    pass


class FfiUtf8Error(FfiError):
    pass


class FfiFiniteError(FfiError):
    pass


class FfiCancelledError(FfiError):
    pass


class FfiModelError(FfiError):
    pass


class FfiVectorError(FfiError):
    pass


class FfiIoError(FfiError):
    pass


class FfiPanicError(FfiError):
    pass


class _Status:
    OK = 0
    NULL = 1
    BOUNDS = 2
    UTF8 = 3
    FINITE = 4
    CANCELLED = 5
    MODEL = 6
    VECTOR = 7
    IO = 8
    PANIC = 9


class _StringView(ctypes.Structure):
    _fields_ = [("ptr", ctypes.POINTER(ctypes.c_uint8)), ("len", ctypes.c_size_t)]


class _ModelInfo(ctypes.Structure):
    _fields_ = [
        ("abi_version", ctypes.c_uint32),
        ("hidden_size", ctypes.c_size_t),
        ("vocab_size", ctypes.c_size_t),
        ("num_layers", ctypes.c_size_t),
        ("num_heads", ctypes.c_size_t),
        ("intermediate_size", ctypes.c_size_t),
        ("max_seq_len", ctypes.c_size_t),
    ]


class _ModelHandle(ctypes.Structure):
    pass


class _CancelHandle(ctypes.Structure):
    pass


@dataclass(frozen=True, slots=True)
class RustModelInfo:
    abi_version: int
    dimensions: int
    vocab_size: int
    num_layers: int
    num_heads: int
    intermediate_size: int
    max_seq_len: int


class RustCancelToken:
    def __init__(self, library: ctypes.CDLL) -> None:
        self._library = library
        self._token = ctypes.POINTER(_CancelHandle)()
        _raise_for_status(library, library.memento_ffi_cancel_token_new(ctypes.byref(self._token)))
        if not self._token:
            raise FfiNullError("FFI returned a null cancel token")
        self._closed = False

    @property
    def pointer(self) -> object:
        return self._token

    def cancel(self) -> None:
        _raise_for_status(self._library, self._library.memento_ffi_cancel_token_cancel(self._token))

    def close(self) -> None:
        if self._closed:
            return
        _raise_for_status(self._library, self._library.memento_ffi_cancel_token_free(self._token))
        self._closed = True

    def __enter__(self) -> RustCancelToken:
        return self

    def __exit__(self, exc_type: object, exc: object, tb: object) -> None:
        self.close()


class RustFfiLibrary:
    def __init__(self, library_path: Path | str) -> None:
        self._path = Path(library_path)
        self._lib = ctypes.CDLL(str(self._path))
        self._configure()
        abi_version = int(self._lib.memento_ffi_abi_version())
        if abi_version != ABI_VERSION:
            raise FfiAbiError(f"unsupported memento-ffi ABI version: {abi_version}")
        features = int(self._lib.memento_ffi_features())
        missing = REQUIRED_FEATURES & ~features
        if missing:
            raise FfiAbiError(f"memento-ffi missing required features mask 0x{missing:x}")

    @property
    def path(self) -> Path:
        return self._path

    def new_cancel_token(self) -> RustCancelToken:
        return RustCancelToken(self._lib)

    def load_model(self, model_path: Path | str) -> RustFfiModel:
        return RustFfiModel(self, model_path)

    def vector_cosine(self, left: Sequence[float], right: Sequence[float]) -> float:
        left_array = _float_array(left)
        right_array = _float_array(right)
        out = ctypes.c_float(0.0)
        status = self._lib.memento_ffi_vector_cosine(
            left_array,
            len(left),
            right_array,
            len(right),
            ctypes.byref(out),
        )
        _raise_for_status(self._lib, status)
        return float(out.value)

    def _configure(self) -> None:
        self._lib.memento_ffi_abi_version.argtypes = []
        self._lib.memento_ffi_abi_version.restype = ctypes.c_uint32
        self._lib.memento_ffi_features.argtypes = []
        self._lib.memento_ffi_features.restype = ctypes.c_uint64
        self._lib.memento_ffi_last_error_message.argtypes = [
            ctypes.POINTER(ctypes.c_uint8),
            ctypes.c_size_t,
            ctypes.POINTER(ctypes.c_size_t),
        ]
        self._lib.memento_ffi_last_error_message.restype = ctypes.c_int
        self._lib.memento_ffi_model_load_path.argtypes = [
            ctypes.POINTER(ctypes.c_uint8),
            ctypes.c_size_t,
            ctypes.POINTER(ctypes.POINTER(_ModelHandle)),
        ]
        self._lib.memento_ffi_model_load_path.restype = ctypes.c_int
        self._lib.memento_ffi_model_free.argtypes = [ctypes.POINTER(_ModelHandle)]
        self._lib.memento_ffi_model_free.restype = ctypes.c_int
        self._lib.memento_ffi_model_info.argtypes = [
            ctypes.POINTER(_ModelHandle),
            ctypes.POINTER(_ModelInfo),
        ]
        self._lib.memento_ffi_model_info.restype = ctypes.c_int
        self._lib.memento_ffi_cancel_token_new.argtypes = [
            ctypes.POINTER(ctypes.POINTER(_CancelHandle))
        ]
        self._lib.memento_ffi_cancel_token_new.restype = ctypes.c_int
        self._lib.memento_ffi_cancel_token_cancel.argtypes = [ctypes.POINTER(_CancelHandle)]
        self._lib.memento_ffi_cancel_token_cancel.restype = ctypes.c_int
        self._lib.memento_ffi_cancel_token_free.argtypes = [ctypes.POINTER(_CancelHandle)]
        self._lib.memento_ffi_cancel_token_free.restype = ctypes.c_int
        self._lib.memento_ffi_embed.argtypes = [
            ctypes.POINTER(_ModelHandle),
            _StringView,
            ctypes.POINTER(_CancelHandle),
            ctypes.c_size_t,
            ctypes.POINTER(ctypes.c_float),
            ctypes.c_size_t,
        ]
        self._lib.memento_ffi_embed.restype = ctypes.c_int
        self._lib.memento_ffi_embed_batch.argtypes = [
            ctypes.POINTER(_ModelHandle),
            ctypes.POINTER(_StringView),
            ctypes.c_size_t,
            ctypes.POINTER(_CancelHandle),
            ctypes.c_size_t,
            ctypes.c_size_t,
            ctypes.POINTER(ctypes.c_float),
            ctypes.c_size_t,
        ]
        self._lib.memento_ffi_embed_batch.restype = ctypes.c_int
        self._lib.memento_ffi_vector_validate.argtypes = [
            ctypes.POINTER(ctypes.c_float),
            ctypes.c_size_t,
        ]
        self._lib.memento_ffi_vector_validate.restype = ctypes.c_int
        self._lib.memento_ffi_vector_cosine.argtypes = [
            ctypes.POINTER(ctypes.c_float),
            ctypes.c_size_t,
            ctypes.POINTER(ctypes.c_float),
            ctypes.c_size_t,
            ctypes.POINTER(ctypes.c_float),
        ]
        self._lib.memento_ffi_vector_cosine.restype = ctypes.c_int


class RustFfiModel(EmbeddingClient):
    def __init__(self, library: RustFfiLibrary, model_path: Path | str) -> None:
        self._library = library
        self._model_path = Path(model_path)
        self._handle = ctypes.POINTER(_ModelHandle)()
        encoded, encoded_ptr, encoded_len = _encode_bytes(str(self._model_path))
        status = self._library._lib.memento_ffi_model_load_path(
            encoded_ptr,
            encoded_len,
            ctypes.byref(self._handle),
        )
        _raise_for_status(self._library._lib, status)
        if not self._handle:
            raise FfiNullError("FFI returned a null model handle")
        self._closed = False

    @property
    def model_path(self) -> Path:
        return self._model_path

    def info(self) -> RustModelInfo:
        raw = _ModelInfo()
        status = self._library._lib.memento_ffi_model_info(self._handle, ctypes.byref(raw))
        _raise_for_status(self._library._lib, status)
        return RustModelInfo(
            abi_version=int(raw.abi_version),
            dimensions=int(raw.hidden_size),
            vocab_size=int(raw.vocab_size),
            num_layers=int(raw.num_layers),
            num_heads=int(raw.num_heads),
            intermediate_size=int(raw.intermediate_size),
            max_seq_len=int(raw.max_seq_len),
        )

    def model_info(self) -> EmbeddingModelInfo:
        info = self.info()
        return EmbeddingModelInfo(
            model_id=self._model_path.name,
            dimensions=info.dimensions,
            revision=self._model_path.name,
            max_batch=1024,
            max_input_chars=max(1, info.max_seq_len),
        )

    def embed(self, text: str, *, cancelled: Callable[[], bool] | None = None) -> tuple[float, ...]:
        info = self.info()
        output = (ctypes.c_float * info.dimensions)()
        encoded, encoded_ptr, encoded_len = _encode_bytes(text)
        _ = encoded
        view = _StringView(ptr=encoded_ptr, len=encoded_len)
        token: RustCancelToken | None = None
        try:
            if cancelled is not None:
                token = self._library.new_cancel_token()
                if cancelled():
                    token.cancel()
            status = self._library._lib.memento_ffi_embed(
                self._handle,
                view,
                token.pointer if token is not None else None,
                len(text),
                output,
                info.dimensions,
            )
            _raise_for_status(self._library._lib, status)
        finally:
            if token is not None:
                token.close()
        result = tuple(float(value) for value in output)
        validate_embedding(result, dimensions=info.dimensions)
        return result

    def embed_batch(
        self, texts: Sequence[str], *, cancelled: Callable[[], bool] | None = None
    ) -> tuple[tuple[float, ...], ...]:
        if not texts:
            return ()
        info = self.info()
        encoded_items = [_encode_bytes(text) for text in texts]
        views = (_StringView * len(texts))(
            *[_StringView(ptr=ptr, len=size) for _buffer, ptr, size in encoded_items]
        )
        output = (ctypes.c_float * (len(texts) * info.dimensions))()
        token: RustCancelToken | None = None
        try:
            if cancelled is not None:
                token = self._library.new_cancel_token()
                if cancelled():
                    token.cancel()
            status = self._library._lib.memento_ffi_embed_batch(
                self._handle,
                views,
                len(texts),
                token.pointer if token is not None else None,
                len(texts),
                max(len(text) for text in texts),
                output,
                len(texts) * info.dimensions,
            )
            _raise_for_status(self._library._lib, status)
        finally:
            if token is not None:
                token.close()
        results = []
        values = [float(item) for item in output]
        for index in range(0, len(values), info.dimensions):
            row = tuple(values[index : index + info.dimensions])
            validate_embedding(row, dimensions=info.dimensions)
            results.append(row)
        return tuple(results)

    def close(self) -> None:
        if self._closed:
            return
        _raise_for_status(
            self._library._lib, self._library._lib.memento_ffi_model_free(self._handle)
        )
        self._closed = True

    def __enter__(self) -> RustFfiModel:
        return self

    def __exit__(self, exc_type: object, exc: object, tb: object) -> None:
        self.close()


def _encode_bytes(value: str) -> tuple[object, object, int]:
    data = value.encode("utf-8")
    buffer = ctypes.create_string_buffer(data)
    return buffer, ctypes.cast(buffer, ctypes.POINTER(ctypes.c_uint8)), len(data)


def _float_array(values: Sequence[float]) -> ctypes.Array[ctypes.c_float]:
    return (ctypes.c_float * len(values))(*[float(value) for value in values])


def _last_error(library: ctypes.CDLL) -> str:
    required = ctypes.c_size_t(0)
    status = library.memento_ffi_last_error_message(None, 0, ctypes.byref(required))
    if int(status) != _Status.OK:
        return f"memento-ffi error {int(status)}"
    if required.value == 0:
        return "memento-ffi call failed"
    buffer = (ctypes.c_uint8 * required.value)()
    status = library.memento_ffi_last_error_message(buffer, required.value, ctypes.byref(required))
    if int(status) != _Status.OK:
        return f"memento-ffi error {int(status)}"
    return bytes(buffer).decode("utf-8", errors="replace")


def _raise_for_status(library: ctypes.CDLL, status: int) -> None:
    code = int(status)
    if code == _Status.OK:
        return
    message = _last_error(library)
    error_cls = {
        _Status.NULL: FfiNullError,
        _Status.BOUNDS: FfiBoundsError,
        _Status.UTF8: FfiUtf8Error,
        _Status.FINITE: FfiFiniteError,
        _Status.CANCELLED: FfiCancelledError,
        _Status.MODEL: FfiModelError,
        _Status.VECTOR: FfiVectorError,
        _Status.IO: FfiIoError,
        _Status.PANIC: FfiPanicError,
    }.get(code, FfiError)
    raise error_cls(message)
