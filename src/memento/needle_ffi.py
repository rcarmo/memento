from __future__ import annotations

import ctypes
import threading
import time
from collections.abc import Callable
from contextlib import suppress
from dataclasses import dataclass
from pathlib import Path

ABI_VERSION = 1
FEATURE_GENERATE = 1 << 0
FEATURE_CANCELLATION = 1 << 1
REQUIRED_FEATURES = FEATURE_GENERATE | FEATURE_CANCELLATION


class NeedleFfiError(RuntimeError):
    def __init__(self, message: str, *, status_code: int | None = None) -> None:
        super().__init__(message)
        self.status_code = status_code


class NeedleFfiAbiError(NeedleFfiError):
    pass


class NeedleFfiNullError(NeedleFfiError):
    pass


class NeedleFfiClosedError(NeedleFfiError):
    pass


class NeedleFfiBoundsError(NeedleFfiError):
    pass


class NeedleFfiUtf8Error(NeedleFfiError):
    pass


class NeedleFfiCancelledError(NeedleFfiError):
    pass


class NeedleFfiModelError(NeedleFfiError):
    pass


class NeedleFfiIoError(NeedleFfiError):
    pass


class NeedleFfiLockError(NeedleFfiError):
    pass


class NeedleFfiPanicError(NeedleFfiError):
    pass


class _Status:
    OK = 0
    NULL = 1
    BOUNDS = 2
    UTF8 = 3
    CANCELLED = 4
    MODEL = 5
    IO = 6
    LOCK = 7
    PANIC = 8


class _StringView(ctypes.Structure):
    _fields_ = [("ptr", ctypes.POINTER(ctypes.c_uint8)), ("len", ctypes.c_size_t)]


class _RouterInfo(ctypes.Structure):
    _fields_ = [
        ("abi_version", ctypes.c_uint32),
        ("d_model", ctypes.c_size_t),
        ("vocab_size", ctypes.c_size_t),
        ("num_encoder_layers", ctypes.c_size_t),
        ("num_decoder_layers", ctypes.c_size_t),
        ("num_heads", ctypes.c_size_t),
        ("num_kv_heads", ctypes.c_size_t),
        ("max_seq_len", ctypes.c_size_t),
    ]


class _RouterHandle(ctypes.Structure):
    pass


class _CancelHandle(ctypes.Structure):
    pass


@dataclass(frozen=True, slots=True)
class NeedleRouterInfo:
    abi_version: int
    d_model: int
    vocab_size: int
    num_encoder_layers: int
    num_decoder_layers: int
    num_heads: int
    num_kv_heads: int
    max_seq_len: int


class NeedleCancelToken:
    def __init__(self, library: ctypes.CDLL) -> None:
        self._library = library
        self._token = ctypes.POINTER(_CancelHandle)()
        _raise_for_status(
            library,
            library.memento_needle_ffi_cancel_token_new(ctypes.byref(self._token)),
        )
        if not self._token:
            raise NeedleFfiNullError("FFI returned a null cancel token")
        self._closed = False

    @property
    def pointer(self) -> object:
        return self._token

    def cancel(self) -> None:
        _raise_for_status(
            self._library,
            self._library.memento_needle_ffi_cancel_token_cancel(self._token),
        )

    def close(self) -> None:
        if self._closed:
            return
        _raise_for_status(
            self._library,
            self._library.memento_needle_ffi_cancel_token_free(self._token),
        )
        self._token = ctypes.POINTER(_CancelHandle)()
        self._closed = True

    def __enter__(self) -> NeedleCancelToken:
        return self

    def __exit__(self, exc_type: object, exc: object, tb: object) -> None:
        self.close()


class NeedleFfiLibrary:
    def __init__(self, library_path: Path | str) -> None:
        self._path = Path(library_path)
        self._lib = ctypes.CDLL(str(self._path))
        self._configure()
        abi_version = int(self._lib.memento_needle_ffi_abi_version())
        if abi_version != ABI_VERSION:
            raise NeedleFfiAbiError(f"unsupported memento-needle-ffi ABI version: {abi_version}")
        features = int(self._lib.memento_needle_ffi_features())
        missing = REQUIRED_FEATURES & ~features
        if missing:
            raise NeedleFfiAbiError(
                f"memento-needle-ffi missing required features mask 0x{missing:x}"
            )

    @property
    def path(self) -> Path:
        return self._path

    def new_cancel_token(self) -> NeedleCancelToken:
        return NeedleCancelToken(self._lib)

    def load_router(self, model_path: Path | str, tokenizer_path: Path | str) -> NeedleRouter:
        return NeedleRouter(self, model_path=model_path, tokenizer_path=tokenizer_path)

    def _configure(self) -> None:
        self._lib.memento_needle_ffi_abi_version.argtypes = []
        self._lib.memento_needle_ffi_abi_version.restype = ctypes.c_uint32
        self._lib.memento_needle_ffi_features.argtypes = []
        self._lib.memento_needle_ffi_features.restype = ctypes.c_uint64
        self._lib.memento_needle_ffi_last_error_message.argtypes = [
            ctypes.POINTER(ctypes.c_uint8),
            ctypes.c_size_t,
            ctypes.POINTER(ctypes.c_size_t),
        ]
        self._lib.memento_needle_ffi_last_error_message.restype = ctypes.c_int
        self._lib.memento_needle_ffi_router_load_paths.argtypes = [
            ctypes.POINTER(ctypes.c_uint8),
            ctypes.c_size_t,
            ctypes.POINTER(ctypes.c_uint8),
            ctypes.c_size_t,
            ctypes.POINTER(ctypes.POINTER(_RouterHandle)),
        ]
        self._lib.memento_needle_ffi_router_load_paths.restype = ctypes.c_int
        self._lib.memento_needle_ffi_router_free.argtypes = [ctypes.POINTER(_RouterHandle)]
        self._lib.memento_needle_ffi_router_free.restype = ctypes.c_int
        self._lib.memento_needle_ffi_router_info.argtypes = [
            ctypes.POINTER(_RouterHandle),
            ctypes.POINTER(_RouterInfo),
        ]
        self._lib.memento_needle_ffi_router_info.restype = ctypes.c_int
        self._lib.memento_needle_ffi_cancel_token_new.argtypes = [
            ctypes.POINTER(ctypes.POINTER(_CancelHandle))
        ]
        self._lib.memento_needle_ffi_cancel_token_new.restype = ctypes.c_int
        self._lib.memento_needle_ffi_cancel_token_cancel.argtypes = [ctypes.POINTER(_CancelHandle)]
        self._lib.memento_needle_ffi_cancel_token_cancel.restype = ctypes.c_int
        self._lib.memento_needle_ffi_cancel_token_free.argtypes = [ctypes.POINTER(_CancelHandle)]
        self._lib.memento_needle_ffi_cancel_token_free.restype = ctypes.c_int
        self._lib.memento_needle_ffi_router_generate.argtypes = [
            ctypes.POINTER(_RouterHandle),
            _StringView,
            _StringView,
            ctypes.c_size_t,
            ctypes.c_size_t,
            ctypes.c_bool,
            ctypes.POINTER(_CancelHandle),
            ctypes.POINTER(ctypes.c_uint8),
            ctypes.POINTER(ctypes.c_size_t),
        ]
        self._lib.memento_needle_ffi_router_generate.restype = ctypes.c_int


class NeedleRouter:
    def __init__(
        self, library: NeedleFfiLibrary, *, model_path: Path | str, tokenizer_path: Path | str
    ) -> None:
        self._library = library
        self._model_path = Path(model_path)
        self._tokenizer_path = Path(tokenizer_path)
        self._handle = ctypes.POINTER(_RouterHandle)()
        model_buf, model_ptr, model_len = _encode_bytes(str(self._model_path))
        tok_buf, tok_ptr, tok_len = _encode_bytes(str(self._tokenizer_path))
        self._buffers = (model_buf, tok_buf)
        status = self._library._lib.memento_needle_ffi_router_load_paths(
            model_ptr,
            model_len,
            tok_ptr,
            tok_len,
            ctypes.byref(self._handle),
        )
        _raise_for_status(self._library._lib, status)
        if not self._handle:
            raise NeedleFfiNullError("FFI returned a null router handle")
        self._closed = False

    @property
    def model_path(self) -> Path:
        return self._model_path

    @property
    def tokenizer_path(self) -> Path:
        return self._tokenizer_path

    def _require_open(self) -> None:
        if self._closed or not self._handle:
            raise NeedleFfiClosedError("router is closed")

    def info(self) -> NeedleRouterInfo:
        self._require_open()
        raw = _RouterInfo()
        status = self._library._lib.memento_needle_ffi_router_info(self._handle, ctypes.byref(raw))
        _raise_for_status(self._library._lib, status)
        return NeedleRouterInfo(
            abi_version=int(raw.abi_version),
            d_model=int(raw.d_model),
            vocab_size=int(raw.vocab_size),
            num_encoder_layers=int(raw.num_encoder_layers),
            num_decoder_layers=int(raw.num_decoder_layers),
            num_heads=int(raw.num_heads),
            num_kv_heads=int(raw.num_kv_heads),
            max_seq_len=int(raw.max_seq_len),
        )

    def generate(
        self,
        query: str,
        tools_json: str,
        *,
        max_enc_len: int = 1024,
        max_gen_len: int = 128,
        constrained: bool = True,
        cancelled: Callable[[], bool] | None = None,
    ) -> str:
        self._require_open()
        query_buf, query_ptr, query_len = _encode_bytes(query)
        tools_buf, tools_ptr, tools_len = _encode_bytes(tools_json)
        _ = (query_buf, tools_buf)
        token: NeedleCancelToken | None = None
        stop_event: threading.Event | None = None
        watcher: threading.Thread | None = None
        callback_error: list[BaseException] = []
        try:
            if cancelled is not None:
                token = self._library.new_cancel_token()
                stop_event = threading.Event()

                def watch_cancel() -> None:
                    while stop_event is not None and not stop_event.is_set():
                        try:
                            should_cancel = cancelled()
                        except BaseException as exc:  # pragma: no cover - defensive
                            callback_error.append(exc)
                            should_cancel = True
                        if should_cancel:
                            with suppress(NeedleFfiError):
                                token.cancel()
                            return
                        time.sleep(0.005)

                watcher = threading.Thread(target=watch_cancel, daemon=True)
                watcher.start()
            return self._generate_with_retry(
                _StringView(ptr=query_ptr, len=query_len),
                _StringView(ptr=tools_ptr, len=tools_len),
                max_enc_len=max_enc_len,
                max_gen_len=max_gen_len,
                constrained=constrained,
                token=token,
            )
        finally:
            if stop_event is not None:
                stop_event.set()
            if watcher is not None:
                watcher.join()
            if token is not None:
                token.close()
            if callback_error:
                raise NeedleFfiError(
                    f"cancel callback failed: {callback_error[0]}",
                    status_code=_Status.CANCELLED,
                ) from callback_error[0]

    def _generate_with_retry(
        self,
        query: _StringView,
        tools_json: _StringView,
        *,
        max_enc_len: int,
        max_gen_len: int,
        constrained: bool,
        token: NeedleCancelToken | None,
    ) -> str:
        size = ctypes.c_size_t(256)
        for _attempt in range(4):
            buffer = (ctypes.c_uint8 * size.value)() if size.value > 0 else None
            status = self._library._lib.memento_needle_ffi_router_generate(
                self._handle,
                query,
                tools_json,
                max_enc_len,
                max_gen_len,
                constrained,
                token.pointer if token is not None else None,
                buffer,
                ctypes.byref(size),
            )
            if int(status) == _Status.OK:
                raw = bytes(buffer[: size.value]) if buffer is not None else b""
                return raw.decode("utf-8")
            if int(status) != _Status.BOUNDS:
                _raise_for_status(self._library._lib, status)
            required = int(size.value)
            if required <= 0:
                _raise_for_status(self._library._lib, status)
            size = ctypes.c_size_t(required)
        raise NeedleFfiBoundsError("router output buffer did not stabilize after retries")

    def close(self) -> None:
        if self._closed:
            return
        _raise_for_status(
            self._library._lib,
            self._library._lib.memento_needle_ffi_router_free(self._handle),
        )
        self._handle = ctypes.POINTER(_RouterHandle)()
        self._closed = True

    def __enter__(self) -> NeedleRouter:
        return self

    def __exit__(self, exc_type: object, exc: object, tb: object) -> None:
        self.close()


def _encode_bytes(value: str) -> tuple[object, object, int]:
    data = value.encode("utf-8")
    buffer = ctypes.create_string_buffer(data)
    return buffer, ctypes.cast(buffer, ctypes.POINTER(ctypes.c_uint8)), len(data)


def _last_error(library: ctypes.CDLL) -> str:
    required = ctypes.c_size_t(0)
    status = library.memento_needle_ffi_last_error_message(None, 0, ctypes.byref(required))
    if int(status) != _Status.OK:
        return f"memento-needle-ffi error {int(status)}"
    if required.value == 0:
        return "memento-needle-ffi call failed"
    buffer = (ctypes.c_uint8 * required.value)()
    status = library.memento_needle_ffi_last_error_message(
        buffer, required.value, ctypes.byref(required)
    )
    if int(status) != _Status.OK:
        return f"memento-needle-ffi error {int(status)}"
    return bytes(buffer).decode("utf-8", errors="replace")


def _raise_for_status(library: ctypes.CDLL, status: int) -> None:
    code = int(status)
    if code == _Status.OK:
        return
    message = _last_error(library)
    error_cls = {
        _Status.NULL: NeedleFfiNullError,
        _Status.BOUNDS: NeedleFfiBoundsError,
        _Status.UTF8: NeedleFfiUtf8Error,
        _Status.CANCELLED: NeedleFfiCancelledError,
        _Status.MODEL: NeedleFfiModelError,
        _Status.IO: NeedleFfiIoError,
        _Status.LOCK: NeedleFfiLockError,
        _Status.PANIC: NeedleFfiPanicError,
    }.get(code, NeedleFfiError)
    raise error_cls(message, status_code=code)
