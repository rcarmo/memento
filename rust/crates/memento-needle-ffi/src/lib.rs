use memento_needle::{GenerationOptions, Model, NeedleError, NeedleTokenizer, RouterModel};
use std::cell::RefCell;
use std::panic::{catch_unwind, AssertUnwindSafe};
use std::path::PathBuf;
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::{Mutex, RwLock};

pub const ABI_VERSION: u32 = 1;
pub const FEATURE_GENERATE: u64 = 1 << 0;
pub const FEATURE_CANCELLATION: u64 = 1 << 1;
const FEATURES: u64 = FEATURE_GENERATE | FEATURE_CANCELLATION;

thread_local! {
    static LAST_ERROR: RefCell<String> = const { RefCell::new(String::new()) };
}

#[repr(C)]
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum MementoNeedleFfiStatus {
    Ok = 0,
    Null = 1,
    Bounds = 2,
    Utf8 = 3,
    Cancelled = 4,
    Model = 5,
    Io = 6,
    Lock = 7,
    Panic = 8,
}

#[repr(C)]
#[derive(Debug, Clone, Copy)]
pub struct MementoNeedleFfiStringView {
    pub ptr: *const u8,
    pub len: usize,
}

#[repr(C)]
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct MementoNeedleFfiRouterInfo {
    pub abi_version: u32,
    pub d_model: usize,
    pub vocab_size: usize,
    pub num_encoder_layers: usize,
    pub num_decoder_layers: usize,
    pub num_heads: usize,
    pub num_kv_heads: usize,
    pub max_seq_len: usize,
}

pub struct MementoNeedleFfiRouterHandle {
    router: RwLock<RouterModel>,
    tokenizer: Mutex<NeedleTokenizer>,
    info: MementoNeedleFfiRouterInfo,
}

pub struct MementoNeedleFfiCancelToken {
    cancelled: AtomicBool,
}

fn clear_last_error() {
    LAST_ERROR.with(|slot| slot.borrow_mut().clear());
}

fn set_last_error(message: impl Into<String>) {
    LAST_ERROR.with(|slot| {
        *slot.borrow_mut() = message.into();
    });
}

fn with_ffi_status<T>(
    f: impl FnOnce() -> Result<T, MementoNeedleFfiStatus>,
) -> Result<T, MementoNeedleFfiStatus> {
    clear_last_error();
    match catch_unwind(AssertUnwindSafe(f)) {
        Ok(result) => result,
        Err(payload) => {
            let message = if let Some(text) = payload.downcast_ref::<&str>() {
                format!("panic: {text}")
            } else if let Some(text) = payload.downcast_ref::<String>() {
                format!("panic: {text}")
            } else {
                "panic across FFI boundary".to_string()
            };
            set_last_error(message);
            Err(MementoNeedleFfiStatus::Panic)
        }
    }
}

fn fail(status: MementoNeedleFfiStatus, message: impl Into<String>) -> MementoNeedleFfiStatus {
    set_last_error(message);
    status
}

unsafe fn require_ref<'a, T>(ptr: *const T, what: &str) -> Result<&'a T, MementoNeedleFfiStatus> {
    if ptr.is_null() {
        return Err(fail(
            MementoNeedleFfiStatus::Null,
            format!("{what} is null"),
        ));
    }
    Ok(&*ptr)
}

unsafe fn require_mut_ref<'a, T>(
    ptr: *mut T,
    what: &str,
) -> Result<&'a mut T, MementoNeedleFfiStatus> {
    if ptr.is_null() {
        return Err(fail(
            MementoNeedleFfiStatus::Null,
            format!("{what} is null"),
        ));
    }
    Ok(&mut *ptr)
}

unsafe fn require_slice<'a, T>(
    ptr: *const T,
    len: usize,
    what: &str,
) -> Result<&'a [T], MementoNeedleFfiStatus> {
    if len == 0 {
        return Ok(&[]);
    }
    if ptr.is_null() {
        return Err(fail(
            MementoNeedleFfiStatus::Null,
            format!("{what} is null"),
        ));
    }
    Ok(std::slice::from_raw_parts(ptr, len))
}

unsafe fn require_mut_slice<'a, T>(
    ptr: *mut T,
    len: usize,
    what: &str,
) -> Result<&'a mut [T], MementoNeedleFfiStatus> {
    if len == 0 {
        return Ok(&mut []);
    }
    if ptr.is_null() {
        return Err(fail(
            MementoNeedleFfiStatus::Null,
            format!("{what} is null"),
        ));
    }
    Ok(std::slice::from_raw_parts_mut(ptr, len))
}

unsafe fn string_view_to_str<'a>(
    view: MementoNeedleFfiStringView,
    what: &str,
) -> Result<&'a str, MementoNeedleFfiStatus> {
    let bytes = require_slice(view.ptr, view.len, what)?;
    std::str::from_utf8(bytes).map_err(|err| {
        fail(
            MementoNeedleFfiStatus::Utf8,
            format!("{what} is not valid UTF-8: {err}"),
        )
    })
}

fn map_needle_error(err: NeedleError) -> MementoNeedleFfiStatus {
    match err {
        NeedleError::Io(io_err) => fail(MementoNeedleFfiStatus::Io, io_err.to_string()),
        NeedleError::Cancelled(stage) => fail(
            MementoNeedleFfiStatus::Cancelled,
            format!("cancelled at checkpoint: {stage}"),
        ),
        NeedleError::GenerationTooLong(max) => fail(
            MementoNeedleFfiStatus::Bounds,
            format!("generation exceeded max length {max}"),
        ),
        other => fail(MementoNeedleFfiStatus::Model, other.to_string()),
    }
}

fn cancellation_checkpoint(
    token: Option<&MementoNeedleFfiCancelToken>,
) -> impl FnMut(&'static str) -> Result<(), NeedleError> + '_ {
    move |stage| {
        if let Some(token) = token {
            if token.cancelled.load(Ordering::Relaxed) {
                return Err(NeedleError::Cancelled(stage));
            }
        }
        Ok(())
    }
}

fn router_info_from_model(model: &Model) -> MementoNeedleFfiRouterInfo {
    let config = model.config();
    MementoNeedleFfiRouterInfo {
        abi_version: ABI_VERSION,
        d_model: config.d_model as usize,
        vocab_size: config.vocab_size as usize,
        num_encoder_layers: config.num_encoder_layers as usize,
        num_decoder_layers: config.num_decoder_layers as usize,
        num_heads: config.num_heads as usize,
        num_kv_heads: config.num_kv_heads as usize,
        max_seq_len: config.max_seq_len as usize,
    }
}

#[no_mangle]
pub extern "C" fn memento_needle_ffi_abi_version() -> u32 {
    with_ffi_status(|| Ok(ABI_VERSION)).unwrap_or_default()
}

#[no_mangle]
pub extern "C" fn memento_needle_ffi_features() -> u64 {
    with_ffi_status(|| Ok(FEATURES)).unwrap_or_default()
}

#[no_mangle]
/// # Safety
///
/// `out_required_len` must be valid for writes. If `buffer_len > 0`, `buffer` must point to a
/// writable region of at least `buffer_len` bytes.
pub unsafe extern "C" fn memento_needle_ffi_last_error_message(
    buffer: *mut u8,
    buffer_len: usize,
    out_required_len: *mut usize,
) -> MementoNeedleFfiStatus {
    match catch_unwind(AssertUnwindSafe(|| {
        let out_required_len = require_mut_ref(out_required_len, "out_required_len")?;
        LAST_ERROR.with(|slot| {
            let message = slot.borrow();
            *out_required_len = message.len();
            if buffer_len > 0 {
                let output = require_mut_slice(buffer, buffer_len, "buffer")?;
                let bytes = message.as_bytes();
                let copied = bytes.len().min(output.len());
                output[..copied].copy_from_slice(&bytes[..copied]);
                if copied < output.len() {
                    output[copied..].fill(0);
                }
            }
            Ok(MementoNeedleFfiStatus::Ok)
        })
    })) {
        Ok(result) => result.unwrap_or_else(|status| status),
        Err(payload) => {
            let message = if let Some(text) = payload.downcast_ref::<&str>() {
                format!("panic: {text}")
            } else if let Some(text) = payload.downcast_ref::<String>() {
                format!("panic: {text}")
            } else {
                "panic across FFI boundary".to_string()
            };
            set_last_error(message);
            MementoNeedleFfiStatus::Panic
        }
    }
}

#[no_mangle]
/// # Safety
///
/// If the path lengths are non-zero, the corresponding pointers must reference readable bytes.
/// `out_handle` must be valid for writes.
pub unsafe extern "C" fn memento_needle_ffi_router_load_paths(
    model_path_ptr: *const u8,
    model_path_len: usize,
    tokenizer_path_ptr: *const u8,
    tokenizer_path_len: usize,
    out_handle: *mut *mut MementoNeedleFfiRouterHandle,
) -> MementoNeedleFfiStatus {
    with_ffi_status(|| {
        let out_handle = require_mut_ref(out_handle, "out_handle")?;
        let model_path =
            std::str::from_utf8(require_slice(model_path_ptr, model_path_len, "model_path")?)
                .map_err(|err| {
                    fail(
                        MementoNeedleFfiStatus::Utf8,
                        format!("model_path is not valid UTF-8: {err}"),
                    )
                })?;
        let tokenizer_path = std::str::from_utf8(require_slice(
            tokenizer_path_ptr,
            tokenizer_path_len,
            "tokenizer_path",
        )?)
        .map_err(|err| {
            fail(
                MementoNeedleFfiStatus::Utf8,
                format!("tokenizer_path is not valid UTF-8: {err}"),
            )
        })?;
        let model = Model::from_path(PathBuf::from(model_path)).map_err(map_needle_error)?;
        let info = router_info_from_model(&model);
        let router = RouterModel::from_ndl(&model).map_err(map_needle_error)?;
        let tokenizer = NeedleTokenizer::from_model_path(PathBuf::from(tokenizer_path))
            .map_err(map_needle_error)?;
        if tokenizer.vocab_size() != info.vocab_size {
            return Err(fail(
                MementoNeedleFfiStatus::Model,
                format!(
                    "tokenizer vocab size {} does not match model vocab size {}",
                    tokenizer.vocab_size(),
                    info.vocab_size
                ),
            ));
        }
        *out_handle = Box::into_raw(Box::new(MementoNeedleFfiRouterHandle {
            router: RwLock::new(router),
            tokenizer: Mutex::new(tokenizer),
            info,
        }));
        Ok(MementoNeedleFfiStatus::Ok)
    })
    .unwrap_or_else(|status| status)
}

#[no_mangle]
/// # Safety
///
/// `handle` must be a pointer previously returned by `memento_needle_ffi_router_load_paths` and
/// must not be freed more than once.
pub unsafe extern "C" fn memento_needle_ffi_router_free(
    handle: *mut MementoNeedleFfiRouterHandle,
) -> MementoNeedleFfiStatus {
    with_ffi_status(|| {
        if handle.is_null() {
            return Err(fail(MementoNeedleFfiStatus::Null, "handle is null"));
        }
        drop(Box::from_raw(handle));
        Ok(MementoNeedleFfiStatus::Ok)
    })
    .unwrap_or_else(|status| status)
}

#[no_mangle]
/// # Safety
///
/// `handle` must be a valid router handle. `out_info` must be valid for writes.
pub unsafe extern "C" fn memento_needle_ffi_router_info(
    handle: *const MementoNeedleFfiRouterHandle,
    out_info: *mut MementoNeedleFfiRouterInfo,
) -> MementoNeedleFfiStatus {
    with_ffi_status(|| {
        let handle = require_ref(handle, "handle")?;
        let out_info = require_mut_ref(out_info, "out_info")?;
        let _guard = handle
            .router
            .read()
            .map_err(|_| fail(MementoNeedleFfiStatus::Lock, "router lock poisoned"))?;
        *out_info = handle.info;
        Ok(MementoNeedleFfiStatus::Ok)
    })
    .unwrap_or_else(|status| status)
}

#[no_mangle]
/// # Safety
///
/// `out_token` must be valid for writes.
pub unsafe extern "C" fn memento_needle_ffi_cancel_token_new(
    out_token: *mut *mut MementoNeedleFfiCancelToken,
) -> MementoNeedleFfiStatus {
    with_ffi_status(|| {
        let out_token = require_mut_ref(out_token, "out_token")?;
        *out_token = Box::into_raw(Box::new(MementoNeedleFfiCancelToken {
            cancelled: AtomicBool::new(false),
        }));
        Ok(MementoNeedleFfiStatus::Ok)
    })
    .unwrap_or_else(|status| status)
}

#[no_mangle]
/// # Safety
///
/// `token` must be a valid token pointer returned by `memento_needle_ffi_cancel_token_new`.
pub unsafe extern "C" fn memento_needle_ffi_cancel_token_cancel(
    token: *mut MementoNeedleFfiCancelToken,
) -> MementoNeedleFfiStatus {
    with_ffi_status(|| {
        let token = require_mut_ref(token, "token")?;
        token.cancelled.store(true, Ordering::Relaxed);
        Ok(MementoNeedleFfiStatus::Ok)
    })
    .unwrap_or_else(|status| status)
}

#[no_mangle]
/// # Safety
///
/// `token` must be a pointer previously returned by `memento_needle_ffi_cancel_token_new` and
/// must not be freed more than once.
pub unsafe extern "C" fn memento_needle_ffi_cancel_token_free(
    token: *mut MementoNeedleFfiCancelToken,
) -> MementoNeedleFfiStatus {
    with_ffi_status(|| {
        if token.is_null() {
            return Err(fail(MementoNeedleFfiStatus::Null, "token is null"));
        }
        drop(Box::from_raw(token));
        Ok(MementoNeedleFfiStatus::Ok)
    })
    .unwrap_or_else(|status| status)
}

#[no_mangle]
/// # Safety
///
/// `handle` must be a valid router handle. Query and tools views must reference readable bytes
/// when non-empty. `cancel_token`, if non-null, must be valid. `inout_output_len` must be valid
/// for reads and writes. If `*inout_output_len > 0`, `output_buffer` must reference at least that
/// many writable bytes.
pub unsafe extern "C" fn memento_needle_ffi_router_generate(
    handle: *const MementoNeedleFfiRouterHandle,
    query: MementoNeedleFfiStringView,
    tools_json: MementoNeedleFfiStringView,
    max_enc_len: usize,
    max_gen_len: usize,
    constrained: bool,
    cancel_token: *const MementoNeedleFfiCancelToken,
    output_buffer: *mut u8,
    inout_output_len: *mut usize,
) -> MementoNeedleFfiStatus {
    with_ffi_status(|| {
        let handle = require_ref(handle, "handle")?;
        let query = string_view_to_str(query, "query")?;
        let tools_json = string_view_to_str(tools_json, "tools_json")?;
        if max_enc_len == 0 {
            return Err(fail(
                MementoNeedleFfiStatus::Bounds,
                "max_enc_len must be greater than zero",
            ));
        }
        if max_gen_len == 0 {
            return Err(fail(
                MementoNeedleFfiStatus::Bounds,
                "max_gen_len must be greater than zero",
            ));
        }
        let inout_output_len = require_mut_ref(inout_output_len, "inout_output_len")?;
        let capacity = *inout_output_len;
        let cancel_token = if cancel_token.is_null() {
            None
        } else {
            Some(require_ref(cancel_token, "cancel_token")?)
        };
        let router = handle
            .router
            .read()
            .map_err(|_| fail(MementoNeedleFfiStatus::Lock, "router lock poisoned"))?;
        let tokenizer = handle
            .tokenizer
            .lock()
            .map_err(|_| fail(MementoNeedleFfiStatus::Lock, "tokenizer lock poisoned"))?;
        let mut checkpoint = cancellation_checkpoint(cancel_token);
        let text = router
            .generate(
                &tokenizer,
                query,
                tools_json,
                GenerationOptions {
                    max_gen_len,
                    max_enc_len,
                    constrained,
                },
                Some(&mut checkpoint),
            )
            .map_err(map_needle_error)?;
        let bytes = text.as_bytes();
        *inout_output_len = bytes.len();
        if capacity < bytes.len() {
            return Err(fail(
                MementoNeedleFfiStatus::Bounds,
                format!(
                    "output buffer too small: need {}, have {}",
                    bytes.len(),
                    capacity
                ),
            ));
        }
        if capacity > 0 {
            let output = require_mut_slice(output_buffer, capacity, "output_buffer")?;
            output[..bytes.len()].copy_from_slice(bytes);
            if bytes.len() < output.len() {
                output[bytes.len()..].fill(0);
            }
        }
        Ok(MementoNeedleFfiStatus::Ok)
    })
    .unwrap_or_else(|status| status)
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::env;
    use std::fs;
    use std::path::PathBuf;
    use std::process::Command;
    use std::ptr;

    const DEFAULT_TOOLS_JSON: &str = "[{\"name\":\"memory_search\",\"parameters\":{\"query\":{\"type\":\"string\"},\"limit\":{\"type\":\"integer\"},\"search_mode\":{\"type\":\"string\"}}},{\"name\":\"memory_read\",\"parameters\":{\"id_or_path\":{\"type\":\"string\"}}},{\"name\":\"memory_status\",\"parameters\":{\"field\":{\"type\":\"string\"}}},{\"name\":\"memory_execute\",\"parameters\":{\"plan\":{\"type\":\"string\"}}}]";

    fn model_path() -> PathBuf {
        PathBuf::from(env!("CARGO_MANIFEST_DIR")).join("../../../models/needle/memento-router.ndl")
    }

    fn tokenizer_path() -> PathBuf {
        PathBuf::from(env!("CARGO_MANIFEST_DIR")).join("../../../models/needle/needle.model")
    }

    fn vendored_model_available() -> bool {
        for path in [model_path(), tokenizer_path()] {
            let Ok(bytes) = fs::read(&path) else {
                eprintln!(
                    "skipping Needle FFI real-model test; missing {}",
                    path.display()
                );
                return false;
            };
            if bytes.starts_with(b"version https://git-lfs.github.com/spec/v1\n") {
                eprintln!(
                    "skipping Needle FFI real-model test; git lfs pull {}",
                    path.display()
                );
                return false;
            }
        }
        true
    }

    unsafe fn load_handle() -> *mut MementoNeedleFfiRouterHandle {
        let model = model_path();
        let tokenizer = tokenizer_path();
        let model_bytes = model.as_os_str().to_string_lossy();
        let tokenizer_bytes = tokenizer.as_os_str().to_string_lossy();
        let mut handle = ptr::null_mut();
        let status = memento_needle_ffi_router_load_paths(
            model_bytes.as_bytes().as_ptr(),
            model_bytes.len(),
            tokenizer_bytes.as_bytes().as_ptr(),
            tokenizer_bytes.len(),
            &raw mut handle,
        );
        assert_eq!(status, MementoNeedleFfiStatus::Ok);
        assert!(!handle.is_null());
        handle
    }

    fn last_error_string() -> String {
        let mut required = 0;
        let status =
            unsafe { memento_needle_ffi_last_error_message(ptr::null_mut(), 0, &raw mut required) };
        assert_eq!(status, MementoNeedleFfiStatus::Ok);
        let mut buffer = vec![0_u8; required];
        let status = unsafe {
            memento_needle_ffi_last_error_message(
                buffer.as_mut_ptr(),
                buffer.len(),
                &raw mut required,
            )
        };
        assert_eq!(status, MementoNeedleFfiStatus::Ok);
        String::from_utf8(buffer).expect("utf8")
    }

    #[test]
    fn exports_abi_and_features() {
        assert_eq!(memento_needle_ffi_abi_version(), ABI_VERSION);
        assert_eq!(memento_needle_ffi_features(), FEATURES);
    }

    #[test]
    fn invalid_utf8_path_is_reported() {
        let mut handle = ptr::null_mut();
        let status = unsafe {
            memento_needle_ffi_router_load_paths(
                [0xff_u8].as_ptr(),
                1,
                b"ok".as_ptr(),
                2,
                &raw mut handle,
            )
        };
        assert_eq!(status, MementoNeedleFfiStatus::Utf8);
        assert!(last_error_string().contains("model_path is not valid UTF-8"));
    }

    #[test]
    fn generate_reports_required_length_and_router_info_when_model_present() {
        if !vendored_model_available() {
            return;
        }
        let handle = unsafe { load_handle() };
        let mut info = MementoNeedleFfiRouterInfo {
            abi_version: 0,
            d_model: 0,
            vocab_size: 0,
            num_encoder_layers: 0,
            num_decoder_layers: 0,
            num_heads: 0,
            num_kv_heads: 0,
            max_seq_len: 0,
        };
        let status = unsafe { memento_needle_ffi_router_info(handle, &raw mut info) };
        assert_eq!(status, MementoNeedleFfiStatus::Ok);
        assert_eq!(info.abi_version, ABI_VERSION);
        assert_eq!(info.d_model, 512);
        assert_eq!(info.num_encoder_layers, 12);
        assert_eq!(info.vocab_size, 8192);

        let query = b"Find Piclaw";
        let mut required = 8;
        let mut tiny = [0_u8; 8];
        let status = unsafe {
            memento_needle_ffi_router_generate(
                handle,
                MementoNeedleFfiStringView {
                    ptr: query.as_ptr(),
                    len: query.len(),
                },
                MementoNeedleFfiStringView {
                    ptr: DEFAULT_TOOLS_JSON.as_bytes().as_ptr(),
                    len: DEFAULT_TOOLS_JSON.len(),
                },
                1024,
                128,
                true,
                ptr::null(),
                tiny.as_mut_ptr(),
                &raw mut required,
            )
        };
        assert_eq!(status, MementoNeedleFfiStatus::Bounds);
        assert!(required > 8);

        let mut buffer = vec![0_u8; required];
        let status = unsafe {
            memento_needle_ffi_router_generate(
                handle,
                MementoNeedleFfiStringView {
                    ptr: query.as_ptr(),
                    len: query.len(),
                },
                MementoNeedleFfiStringView {
                    ptr: DEFAULT_TOOLS_JSON.as_bytes().as_ptr(),
                    len: DEFAULT_TOOLS_JSON.len(),
                },
                1024,
                128,
                true,
                ptr::null(),
                buffer.as_mut_ptr(),
                &raw mut required,
            )
        };
        assert_eq!(status, MementoNeedleFfiStatus::Ok);
        let text = String::from_utf8(buffer[..required].to_vec()).expect("utf8 output");
        assert!(text.contains("memory_search"));
        assert!(text.contains("Piclaw"));

        let status = unsafe { memento_needle_ffi_router_free(handle) };
        assert_eq!(status, MementoNeedleFfiStatus::Ok);
    }

    #[test]
    fn cancelled_generation_is_reported_when_model_present() {
        if !vendored_model_available() {
            return;
        }
        let handle = unsafe { load_handle() };
        let mut token = ptr::null_mut();
        let status = unsafe { memento_needle_ffi_cancel_token_new(&raw mut token) };
        assert_eq!(status, MementoNeedleFfiStatus::Ok);
        let status = unsafe { memento_needle_ffi_cancel_token_cancel(token) };
        assert_eq!(status, MementoNeedleFfiStatus::Ok);
        let query = b"Find Piclaw";
        let mut required = 1024;
        let mut buffer = vec![0_u8; required];
        let status = unsafe {
            memento_needle_ffi_router_generate(
                handle,
                MementoNeedleFfiStringView {
                    ptr: query.as_ptr(),
                    len: query.len(),
                },
                MementoNeedleFfiStringView {
                    ptr: DEFAULT_TOOLS_JSON.as_bytes().as_ptr(),
                    len: DEFAULT_TOOLS_JSON.len(),
                },
                1024,
                128,
                true,
                token,
                buffer.as_mut_ptr(),
                &raw mut required,
            )
        };
        assert_eq!(status, MementoNeedleFfiStatus::Cancelled);
        assert!(last_error_string().contains("cancelled"));
        assert_eq!(
            unsafe { memento_needle_ffi_cancel_token_free(token) },
            MementoNeedleFfiStatus::Ok
        );
        assert_eq!(
            unsafe { memento_needle_ffi_router_free(handle) },
            MementoNeedleFfiStatus::Ok
        );
    }

    #[test]
    fn c_smoke_test_runs_when_compiler_and_model_exist() {
        if !vendored_model_available() {
            return;
        }
        let compiler = ["cc", "clang", "gcc"].into_iter().find(|candidate| {
            Command::new("sh")
                .arg("-c")
                .arg(format!("command -v {candidate}"))
                .status()
                .is_ok_and(|status| status.success())
        });
        let Some(compiler) = compiler else {
            eprintln!("skipping Needle FFI C smoke test; no C compiler found");
            return;
        };

        let workspace_root = PathBuf::from(env!("CARGO_MANIFEST_DIR")).join("../../..");
        let status = Command::new("cargo")
            .arg("build")
            .arg("-p")
            .arg("memento-needle-ffi")
            .current_dir(workspace_root.join("rust"))
            .status()
            .expect("build needle ffi cdylib");
        assert!(status.success());

        let lib_dir = workspace_root.join("rust/target/debug");
        let header =
            workspace_root.join("rust/crates/memento-needle-ffi/include/memento_needle_ffi.h");
        let source = workspace_root.join("rust/target/memento_needle_ffi_smoke.c");
        let binary = workspace_root.join("rust/target/memento_needle_ffi_smoke");
        let model = model_path();
        let tokenizer = tokenizer_path();
        let tools_c = DEFAULT_TOOLS_JSON.replace('"', "\\\"");
        let c_source = format!(
            "#include <stdint.h>\n#include <stdio.h>\n#include \"{}\"\nint main(void) {{\n  const char *model = \"{}\";\n  const char *tokenizer = \"{}\";\n  const char *query = \"Find Piclaw\";\n  const char *tools = \"{}\";\n  MementoNeedleFfiRouterHandle *handle = NULL;\n  size_t out_len = 512u;\n  uint8_t output[512] = {{0}};\n  if (memento_needle_ffi_router_load_paths((const uint8_t *)model, {}u, (const uint8_t *)tokenizer, {}u, &handle) != MEMENTO_NEEDLE_FFI_STATUS_OK) return 1;\n  if (memento_needle_ffi_router_generate(handle, (MementoNeedleFfiStringView){{(const uint8_t *)query, 11u}}, (MementoNeedleFfiStringView){{(const uint8_t *)tools, {}u}}, 1024u, 128u, true, NULL, output, &out_len) != MEMENTO_NEEDLE_FFI_STATUS_OK) return 2;\n  if (out_len == 0u) return 3;\n  if (memento_needle_ffi_router_free(handle) != MEMENTO_NEEDLE_FFI_STATUS_OK) return 4;\n  return 0;\n}}\n",
            header.display(),
            model.display(),
            tokenizer.display(),
            tools_c,
            model.as_os_str().to_string_lossy().len(),
            tokenizer.as_os_str().to_string_lossy().len(),
            DEFAULT_TOOLS_JSON.len(),
        );
        fs::write(&source, c_source).expect("write C source");

        let link_arg = format!("-Wl,-rpath,{}", lib_dir.display());
        let status = Command::new(compiler)
            .arg(&source)
            .arg("-o")
            .arg(&binary)
            .arg("-L")
            .arg(&lib_dir)
            .arg("-lmemento_needle_ffi")
            .arg(&link_arg)
            .status()
            .expect("compile C smoke test");
        assert!(status.success());

        let status = Command::new(&binary).status().expect("run C smoke test");
        assert!(status.success());

        let _ = fs::remove_file(source);
        let _ = fs::remove_file(binary);
    }
}
