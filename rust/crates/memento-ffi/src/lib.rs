use memento_gte::{BatchOptions, GteError, Model, ModelConfig};
use memento_vector::{cosine, VectorError};
use std::cell::RefCell;
#[cfg(test)]
use std::fs;
use std::panic::{catch_unwind, AssertUnwindSafe};
use std::path::PathBuf;
#[cfg(test)]
use std::ptr;
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::RwLock;

pub const ABI_VERSION: u32 = 1;
pub const FEATURE_EMBED: u64 = 1 << 0;
pub const FEATURE_EMBED_BATCH: u64 = 1 << 1;
pub const FEATURE_CANCELLATION: u64 = 1 << 2;
pub const FEATURE_VECTOR_COSINE: u64 = 1 << 3;
pub const FEATURE_VECTOR_VALIDATE: u64 = 1 << 4;
const FEATURES: u64 = FEATURE_EMBED
    | FEATURE_EMBED_BATCH
    | FEATURE_CANCELLATION
    | FEATURE_VECTOR_COSINE
    | FEATURE_VECTOR_VALIDATE;

thread_local! {
    static LAST_ERROR: RefCell<String> = const { RefCell::new(String::new()) };
}

#[repr(C)]
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum MementoFfiStatus {
    Ok = 0,
    Null = 1,
    Bounds = 2,
    Utf8 = 3,
    Finite = 4,
    Cancelled = 5,
    Model = 6,
    Vector = 7,
    Io = 8,
    Panic = 9,
}

#[repr(C)]
#[derive(Debug, Clone, Copy)]
pub struct MementoFfiStringView {
    pub ptr: *const u8,
    pub len: usize,
}

#[repr(C)]
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct MementoFfiModelInfo {
    pub abi_version: u32,
    pub hidden_size: usize,
    pub vocab_size: usize,
    pub num_layers: usize,
    pub num_heads: usize,
    pub intermediate_size: usize,
    pub max_seq_len: usize,
}

pub struct MementoFfiModelHandle {
    model: RwLock<Model>,
}

pub struct MementoFfiCancelToken {
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
    f: impl FnOnce() -> Result<T, MementoFfiStatus>,
) -> Result<T, MementoFfiStatus> {
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
            Err(MementoFfiStatus::Panic)
        }
    }
}

fn fail(status: MementoFfiStatus, message: impl Into<String>) -> MementoFfiStatus {
    set_last_error(message);
    status
}

unsafe fn require_ref<'a, T>(ptr: *const T, what: &str) -> Result<&'a T, MementoFfiStatus> {
    if ptr.is_null() {
        return Err(fail(MementoFfiStatus::Null, format!("{what} is null")));
    }
    Ok(&*ptr)
}

unsafe fn require_mut_ref<'a, T>(ptr: *mut T, what: &str) -> Result<&'a mut T, MementoFfiStatus> {
    if ptr.is_null() {
        return Err(fail(MementoFfiStatus::Null, format!("{what} is null")));
    }
    Ok(&mut *ptr)
}

unsafe fn require_slice<'a, T>(
    ptr: *const T,
    len: usize,
    what: &str,
) -> Result<&'a [T], MementoFfiStatus> {
    if len == 0 {
        return Ok(&[]);
    }
    if ptr.is_null() {
        return Err(fail(MementoFfiStatus::Null, format!("{what} is null")));
    }
    Ok(std::slice::from_raw_parts(ptr, len))
}

unsafe fn require_mut_slice<'a, T>(
    ptr: *mut T,
    len: usize,
    what: &str,
) -> Result<&'a mut [T], MementoFfiStatus> {
    if len == 0 {
        return Ok(&mut []);
    }
    if ptr.is_null() {
        return Err(fail(MementoFfiStatus::Null, format!("{what} is null")));
    }
    Ok(std::slice::from_raw_parts_mut(ptr, len))
}

unsafe fn string_view_to_str<'a>(
    view: MementoFfiStringView,
    what: &str,
) -> Result<&'a str, MementoFfiStatus> {
    let bytes = require_slice(view.ptr, view.len, what)?;
    std::str::from_utf8(bytes).map_err(|err| {
        fail(
            MementoFfiStatus::Utf8,
            format!("{what} is not valid UTF-8: {err}"),
        )
    })
}

fn ensure_finite(values: &[f32], what: &str) -> Result<(), MementoFfiStatus> {
    for (index, value) in values.iter().copied().enumerate() {
        if !value.is_finite() {
            return Err(fail(
                MementoFfiStatus::Finite,
                format!("{what} contains non-finite value at index {index}"),
            ));
        }
    }
    Ok(())
}

fn map_gte_error(err: GteError) -> MementoFfiStatus {
    match err {
        GteError::Io(io_err) => fail(MementoFfiStatus::Io, io_err.to_string()),
        GteError::Cancelled(stage) => fail(
            MementoFfiStatus::Cancelled,
            format!("cancelled at checkpoint: {stage}"),
        ),
        GteError::OutputLen { got, expected } => fail(
            MementoFfiStatus::Bounds,
            format!("output length mismatch: got {got}, expected {expected}"),
        ),
        GteError::BatchTooLarge(size) => {
            fail(MementoFfiStatus::Bounds, format!("batch too large: {size}"))
        }
        GteError::InputTooLarge { index, len, max } => fail(
            MementoFfiStatus::Bounds,
            format!("input {index} too large: {len} chars > {max}"),
        ),
        GteError::InvalidMagic => fail(MementoFfiStatus::Model, "invalid model magic"),
        GteError::InvalidModel(message) => fail(MementoFfiStatus::Model, message),
    }
}

fn map_vector_error(err: &VectorError) -> MementoFfiStatus {
    match err {
        VectorError::NonFinite { index } => fail(
            MementoFfiStatus::Finite,
            format!("vector contains non-finite value at index {index}"),
        ),
        VectorError::DimensionMismatch { left, right } => fail(
            MementoFfiStatus::Bounds,
            format!("vector dimension mismatch: {left} vs {right}"),
        ),
        VectorError::ZeroNorm => fail(MementoFfiStatus::Vector, "zero-norm vector"),
        VectorError::InvalidByteLength(len) => fail(
            MementoFfiStatus::Bounds,
            format!("invalid byte length: {len}"),
        ),
        VectorError::InvalidMatrixShape { .. } | VectorError::ShapeOverflow { .. } => {
            fail(MementoFfiStatus::Bounds, err.to_string())
        }
    }
}

fn cancellation_checkpoint(
    token: Option<&MementoFfiCancelToken>,
) -> impl FnMut(&'static str) -> Result<(), GteError> + '_ {
    move |stage| {
        if let Some(token) = token {
            if token.cancelled.load(Ordering::Relaxed) {
                return Err(GteError::Cancelled(stage));
            }
        }
        Ok(())
    }
}

#[must_use]
fn model_info(model: &Model) -> MementoFfiModelInfo {
    let ModelConfig {
        vocab_size,
        hidden_size,
        num_layers,
        num_heads,
        intermediate,
        max_seq_len,
    } = model.config.clone();
    MementoFfiModelInfo {
        abi_version: ABI_VERSION,
        hidden_size,
        vocab_size,
        num_layers,
        num_heads,
        intermediate_size: intermediate,
        max_seq_len,
    }
}

#[no_mangle]
pub extern "C" fn memento_ffi_abi_version() -> u32 {
    with_ffi_status(|| Ok(ABI_VERSION)).unwrap_or_default()
}

#[no_mangle]
pub extern "C" fn memento_ffi_features() -> u64 {
    with_ffi_status(|| Ok(FEATURES)).unwrap_or_default()
}

#[no_mangle]
/// # Safety
///
/// `out_required_len` must be valid for writes. If `buffer_len > 0`, `buffer` must point to a
/// writable region of at least `buffer_len` bytes.
pub unsafe extern "C" fn memento_ffi_last_error_message(
    buffer: *mut u8,
    buffer_len: usize,
    out_required_len: *mut usize,
) -> MementoFfiStatus {
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
            Ok(MementoFfiStatus::Ok)
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
            MementoFfiStatus::Panic
        }
    }
}

#[no_mangle]
/// # Safety
///
/// If `path_len > 0`, `path_ptr` must reference `path_len` readable bytes. `out_handle` must be
/// valid for writes.
pub unsafe extern "C" fn memento_ffi_model_load_path(
    path_ptr: *const u8,
    path_len: usize,
    out_handle: *mut *mut MementoFfiModelHandle,
) -> MementoFfiStatus {
    with_ffi_status(|| {
        let out_handle = require_mut_ref(out_handle, "out_handle")?;
        let path_bytes = require_slice(path_ptr, path_len, "path")?;
        let path = std::str::from_utf8(path_bytes).map_err(|err| {
            fail(
                MementoFfiStatus::Utf8,
                format!("path is not valid UTF-8: {err}"),
            )
        })?;
        let model = Model::from_path(PathBuf::from(path)).map_err(map_gte_error)?;
        let handle = Box::new(MementoFfiModelHandle {
            model: RwLock::new(model),
        });
        *out_handle = Box::into_raw(handle);
        Ok(MementoFfiStatus::Ok)
    })
    .unwrap_or_else(|status| status)
}

#[no_mangle]
/// # Safety
///
/// `handle` must be a pointer previously returned by `memento_ffi_model_load_path` and must not
/// be freed more than once.
pub unsafe extern "C" fn memento_ffi_model_free(
    handle: *mut MementoFfiModelHandle,
) -> MementoFfiStatus {
    with_ffi_status(|| {
        if handle.is_null() {
            return Err(fail(MementoFfiStatus::Null, "handle is null"));
        }
        drop(Box::from_raw(handle));
        Ok(MementoFfiStatus::Ok)
    })
    .unwrap_or_else(|status| status)
}

#[no_mangle]
/// # Safety
///
/// `handle` must be a valid model handle returned by `memento_ffi_model_load_path`. `out_info`
/// must be valid for writes.
pub unsafe extern "C" fn memento_ffi_model_info(
    handle: *const MementoFfiModelHandle,
    out_info: *mut MementoFfiModelInfo,
) -> MementoFfiStatus {
    with_ffi_status(|| {
        let handle = require_ref(handle, "handle")?;
        let out_info = require_mut_ref(out_info, "out_info")?;
        let model = handle
            .model
            .read()
            .map_err(|_| fail(MementoFfiStatus::Model, "model lock poisoned"))?;
        *out_info = model_info(&model);
        Ok(MementoFfiStatus::Ok)
    })
    .unwrap_or_else(|status| status)
}

#[no_mangle]
/// # Safety
///
/// `out_token` must be valid for writes.
pub unsafe extern "C" fn memento_ffi_cancel_token_new(
    out_token: *mut *mut MementoFfiCancelToken,
) -> MementoFfiStatus {
    with_ffi_status(|| {
        let out_token = require_mut_ref(out_token, "out_token")?;
        *out_token = Box::into_raw(Box::new(MementoFfiCancelToken {
            cancelled: AtomicBool::new(false),
        }));
        Ok(MementoFfiStatus::Ok)
    })
    .unwrap_or_else(|status| status)
}

#[no_mangle]
/// # Safety
///
/// `token` must be a valid token pointer returned by `memento_ffi_cancel_token_new`.
pub unsafe extern "C" fn memento_ffi_cancel_token_cancel(
    token: *mut MementoFfiCancelToken,
) -> MementoFfiStatus {
    with_ffi_status(|| {
        let token = require_mut_ref(token, "token")?;
        token.cancelled.store(true, Ordering::Relaxed);
        Ok(MementoFfiStatus::Ok)
    })
    .unwrap_or_else(|status| status)
}

#[no_mangle]
/// # Safety
///
/// `token` must be a pointer previously returned by `memento_ffi_cancel_token_new` and must not
/// be freed more than once.
pub unsafe extern "C" fn memento_ffi_cancel_token_free(
    token: *mut MementoFfiCancelToken,
) -> MementoFfiStatus {
    with_ffi_status(|| {
        if token.is_null() {
            return Err(fail(MementoFfiStatus::Null, "token is null"));
        }
        drop(Box::from_raw(token));
        Ok(MementoFfiStatus::Ok)
    })
    .unwrap_or_else(|status| status)
}

#[no_mangle]
/// # Safety
///
/// `handle` must be a valid model handle. If `text.len > 0`, `text.ptr` must reference readable
/// bytes. If `cancel_token` is non-null it must be valid. `out_embedding` must reference
/// `out_embedding_len` writable `float`s.
pub unsafe extern "C" fn memento_ffi_embed(
    handle: *const MementoFfiModelHandle,
    text: MementoFfiStringView,
    cancel_token: *const MementoFfiCancelToken,
    max_chars: usize,
    out_embedding: *mut f32,
    out_embedding_len: usize,
) -> MementoFfiStatus {
    with_ffi_status(|| {
        let handle = require_ref(handle, "handle")?;
        let text = string_view_to_str(text, "text")?;
        if max_chars > 0 && text.len() > max_chars {
            return Err(fail(
                MementoFfiStatus::Bounds,
                format!("input too large: {} chars > {max_chars}", text.len()),
            ));
        }
        let out_embedding = require_mut_slice(out_embedding, out_embedding_len, "out_embedding")?;
        ensure_finite(out_embedding, "out_embedding")?;
        let token = if cancel_token.is_null() {
            None
        } else {
            Some(require_ref(cancel_token, "cancel_token")?)
        };
        let model = handle
            .model
            .read()
            .map_err(|_| fail(MementoFfiStatus::Model, "model lock poisoned"))?;
        if out_embedding.len() != model.dim() {
            return Err(fail(
                MementoFfiStatus::Bounds,
                format!("out_embedding_len must equal {}", model.dim()),
            ));
        }
        let mut checkpoint = cancellation_checkpoint(token);
        model
            .embed_to(text, out_embedding, Some(&mut checkpoint))
            .map_err(map_gte_error)?;
        ensure_finite(out_embedding, "out_embedding")?;
        Ok(MementoFfiStatus::Ok)
    })
    .unwrap_or_else(|status| status)
}

#[no_mangle]
/// # Safety
///
/// `handle` must be a valid model handle. If `texts_len > 0`, `texts_ptr` must reference
/// `texts_len` readable `MementoFfiStringView` values, and each non-empty view must reference
/// readable bytes. If `cancel_token` is non-null it must be valid. `out_embeddings` must
/// reference `out_embeddings_len` writable `float`s.
pub unsafe extern "C" fn memento_ffi_embed_batch(
    handle: *const MementoFfiModelHandle,
    texts_ptr: *const MementoFfiStringView,
    texts_len: usize,
    cancel_token: *const MementoFfiCancelToken,
    max_batch: usize,
    max_chars_per_input: usize,
    out_embeddings: *mut f32,
    out_embeddings_len: usize,
) -> MementoFfiStatus {
    with_ffi_status(|| {
        let handle = require_ref(handle, "handle")?;
        let views = require_slice(texts_ptr, texts_len, "texts_ptr")?;
        let token = if cancel_token.is_null() {
            None
        } else {
            Some(require_ref(cancel_token, "cancel_token")?)
        };
        let texts = views
            .iter()
            .copied()
            .map(|view| string_view_to_str(view, "text").map(ToOwned::to_owned))
            .collect::<Result<Vec<_>, _>>()?;
        let out_embeddings =
            require_mut_slice(out_embeddings, out_embeddings_len, "out_embeddings")?;
        ensure_finite(out_embeddings, "out_embeddings")?;
        let model = handle
            .model
            .read()
            .map_err(|_| fail(MementoFfiStatus::Model, "model lock poisoned"))?;
        let expected_len = texts.len().checked_mul(model.dim()).ok_or_else(|| {
            fail(
                MementoFfiStatus::Bounds,
                "texts_len * model.dim() overflowed output length computation",
            )
        })?;
        if out_embeddings.len() != expected_len {
            return Err(fail(
                MementoFfiStatus::Bounds,
                format!("out_embeddings_len must equal {expected_len}"),
            ));
        }
        let options = BatchOptions {
            max_batch: (max_batch > 0).then_some(max_batch),
            max_chars_per_input: (max_chars_per_input > 0).then_some(max_chars_per_input),
        };
        let mut checkpoint = cancellation_checkpoint(token);
        let embeddings = model
            .embed_batch(&texts, options, Some(&mut checkpoint))
            .map_err(map_gte_error)?;
        for (chunk, embedding) in out_embeddings.chunks_exact_mut(model.dim()).zip(embeddings) {
            chunk.copy_from_slice(&embedding);
        }
        ensure_finite(out_embeddings, "out_embeddings")?;
        Ok(MementoFfiStatus::Ok)
    })
    .unwrap_or_else(|status| status)
}

#[no_mangle]
/// # Safety
///
/// If `values_len > 0`, `values_ptr` must reference `values_len` readable `float`s.
pub unsafe extern "C" fn memento_ffi_vector_validate(
    values_ptr: *const f32,
    values_len: usize,
) -> MementoFfiStatus {
    with_ffi_status(|| {
        let values = require_slice(values_ptr, values_len, "values_ptr")?;
        ensure_finite(values, "values")?;
        Ok(MementoFfiStatus::Ok)
    })
    .unwrap_or_else(|status| status)
}

#[no_mangle]
/// # Safety
///
/// If `left_len > 0` or `right_len > 0`, the corresponding pointers must reference readable
/// `float`s. `out_cosine` must be valid for writes.
pub unsafe extern "C" fn memento_ffi_vector_cosine(
    left_ptr: *const f32,
    left_len: usize,
    right_ptr: *const f32,
    right_len: usize,
    out_cosine: *mut f32,
) -> MementoFfiStatus {
    with_ffi_status(|| {
        let left = require_slice(left_ptr, left_len, "left_ptr")?;
        let right = require_slice(right_ptr, right_len, "right_ptr")?;
        let out_cosine = require_mut_ref(out_cosine, "out_cosine")?;
        ensure_finite(left, "left")?;
        ensure_finite(right, "right")?;
        *out_cosine = cosine(left, right).map_err(|err| map_vector_error(&err))?;
        ensure_finite(std::slice::from_ref(out_cosine), "out_cosine")?;
        Ok(MementoFfiStatus::Ok)
    })
    .unwrap_or_else(|status| status)
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::env;
    use std::path::{Path, PathBuf};
    use std::process::Command;
    use std::time::{SystemTime, UNIX_EPOCH};

    fn push_u16(out: &mut Vec<u8>, value: u16) {
        out.extend_from_slice(&value.to_le_bytes());
    }

    fn push_u32(out: &mut Vec<u8>, value: u32) {
        out.extend_from_slice(&value.to_le_bytes());
    }

    fn push_f32s(out: &mut Vec<u8>, values: &[f32]) {
        for value in values {
            out.extend_from_slice(&value.to_le_bytes());
        }
    }

    #[allow(clippy::too_many_lines)]
    fn synthetic_model_bytes() -> Vec<u8> {
        let vocab = vec![
            "[PAD]",
            "[unused1]",
            "[unused2]",
            "[unused3]",
            "[unused4]",
            "[unused5]",
            "[unused6]",
            "[unused7]",
            "[unused8]",
            "[unused9]",
            "[unused10]",
            "[unused11]",
            "[unused12]",
            "[unused13]",
            "[unused14]",
            "[unused15]",
            "[unused16]",
            "[unused17]",
            "[unused18]",
            "[unused19]",
            "[unused20]",
            "[unused21]",
            "[unused22]",
            "[unused23]",
            "[unused24]",
            "[unused25]",
            "[unused26]",
            "[unused27]",
            "[unused28]",
            "[unused29]",
            "[unused30]",
            "[unused31]",
            "[unused32]",
            "[unused33]",
            "[unused34]",
            "[unused35]",
            "[unused36]",
            "[unused37]",
            "[unused38]",
            "[unused39]",
            "[unused40]",
            "[unused41]",
            "[unused42]",
            "[unused43]",
            "[unused44]",
            "[unused45]",
            "[unused46]",
            "[unused47]",
            "[unused48]",
            "[unused49]",
            "[unused50]",
            "[unused51]",
            "[unused52]",
            "[unused53]",
            "[unused54]",
            "[unused55]",
            "[unused56]",
            "[unused57]",
            "[unused58]",
            "[unused59]",
            "[unused60]",
            "[unused61]",
            "[unused62]",
            "[unused63]",
            "[unused64]",
            "[unused65]",
            "[unused66]",
            "[unused67]",
            "[unused68]",
            "[unused69]",
            "[unused70]",
            "[unused71]",
            "[unused72]",
            "[unused73]",
            "[unused74]",
            "[unused75]",
            "[unused76]",
            "[unused77]",
            "[unused78]",
            "[unused79]",
            "[unused80]",
            "[unused81]",
            "[unused82]",
            "[unused83]",
            "[unused84]",
            "[unused85]",
            "[unused86]",
            "[unused87]",
            "[unused88]",
            "[unused89]",
            "[unused90]",
            "[unused91]",
            "[unused92]",
            "[unused93]",
            "[unused94]",
            "[unused95]",
            "[unused96]",
            "[unused97]",
            "[unused98]",
            "[unused99]",
            "[UNK]",
            "[CLS]",
            "[SEP]",
            "[MASK]",
            "hello",
            "world",
            ",",
            "!",
        ];
        let hidden_size = 4_u32;
        let max_seq_len = 8_u32;
        let mut bytes = Vec::new();
        bytes.extend_from_slice(b"GTE1");
        push_u32(&mut bytes, vocab.len() as u32);
        push_u32(&mut bytes, hidden_size);
        push_u32(&mut bytes, 0);
        push_u32(&mut bytes, 1);
        push_u32(&mut bytes, hidden_size);
        push_u32(&mut bytes, max_seq_len);
        for token in &vocab {
            push_u16(&mut bytes, u16::try_from(token.len()).expect("token len"));
            bytes.extend_from_slice(token.as_bytes());
        }
        let hidden_size_usize = hidden_size as usize;
        for token_id in 0..vocab.len() {
            let base = (token_id as f32) + 1.0;
            push_f32s(&mut bytes, &[base, 0.0, 0.0, 0.0][..hidden_size_usize]);
        }
        push_f32s(
            &mut bytes,
            &vec![0.0; (max_seq_len as usize) * hidden_size_usize],
        );
        push_f32s(&mut bytes, &vec![0.0; 2 * hidden_size_usize]);
        push_f32s(&mut bytes, &vec![1.0; hidden_size_usize]);
        push_f32s(&mut bytes, &vec![0.0; hidden_size_usize]);
        push_f32s(
            &mut bytes,
            &vec![0.0; hidden_size_usize * hidden_size_usize],
        );
        push_f32s(&mut bytes, &vec![0.0; hidden_size_usize]);
        bytes
    }

    fn unique_path(name: &str) -> PathBuf {
        let nanos = SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .expect("time")
            .as_nanos();
        env::temp_dir().join(format!("{name}-{nanos}.gtemodel"))
    }

    fn write_fixture_model() -> PathBuf {
        let path = unique_path("memento-ffi-test-model");
        fs::write(&path, synthetic_model_bytes()).expect("write model");
        path
    }

    unsafe fn load_handle(path: &Path) -> *mut MementoFfiModelHandle {
        let bytes = path.as_os_str().to_string_lossy();
        let mut handle = ptr::null_mut();
        let status =
            memento_ffi_model_load_path(bytes.as_bytes().as_ptr(), bytes.len(), &raw mut handle);
        assert_eq!(status, MementoFfiStatus::Ok);
        assert!(!handle.is_null());
        handle
    }

    fn last_error_string() -> String {
        let mut required = 0;
        let status =
            unsafe { memento_ffi_last_error_message(ptr::null_mut(), 0, &raw mut required) };
        assert_eq!(status, MementoFfiStatus::Ok);
        let mut buffer = vec![0_u8; required];
        let status = unsafe {
            memento_ffi_last_error_message(buffer.as_mut_ptr(), buffer.len(), &raw mut required)
        };
        assert_eq!(status, MementoFfiStatus::Ok);
        String::from_utf8(buffer).expect("utf8")
    }

    #[test]
    fn exports_abi_and_model_info() {
        assert_eq!(memento_ffi_abi_version(), ABI_VERSION);
        assert_eq!(memento_ffi_features(), FEATURES);
        let path = write_fixture_model();
        let handle = unsafe { load_handle(&path) };
        let mut info = MementoFfiModelInfo {
            abi_version: 0,
            hidden_size: 0,
            vocab_size: 0,
            num_layers: 0,
            num_heads: 0,
            intermediate_size: 0,
            max_seq_len: 0,
        };
        let status = unsafe { memento_ffi_model_info(handle, &raw mut info) };
        assert_eq!(status, MementoFfiStatus::Ok);
        assert_eq!(info.abi_version, ABI_VERSION);
        assert_eq!(info.hidden_size, 4);
        assert_eq!(info.max_seq_len, 8);
        assert_eq!(info.num_layers, 0);
        let status = unsafe { memento_ffi_model_free(handle) };
        assert_eq!(status, MementoFfiStatus::Ok);
        fs::remove_file(path).expect("cleanup");
    }

    #[test]
    fn embed_and_batch_fill_caller_owned_buffers() {
        let path = write_fixture_model();
        let handle = unsafe { load_handle(&path) };
        let text = b"hello";
        let mut single = vec![7.0_f32; 4];
        let status = unsafe {
            memento_ffi_embed(
                handle,
                MementoFfiStringView {
                    ptr: text.as_ptr(),
                    len: text.len(),
                },
                ptr::null(),
                16,
                single.as_mut_ptr(),
                single.len(),
            )
        };
        assert_eq!(status, MementoFfiStatus::Ok);
        assert!(single.iter().all(|value| value.is_finite()));

        let items = [b"hello".as_slice(), b"world".as_slice()];
        let views = items
            .iter()
            .map(|item| MementoFfiStringView {
                ptr: item.as_ptr(),
                len: item.len(),
            })
            .collect::<Vec<_>>();
        let mut batch = vec![3.0_f32; 8];
        let status = unsafe {
            memento_ffi_embed_batch(
                handle,
                views.as_ptr(),
                views.len(),
                ptr::null(),
                views.len(),
                16,
                batch.as_mut_ptr(),
                batch.len(),
            )
        };
        assert_eq!(status, MementoFfiStatus::Ok);
        assert!(batch.iter().all(|value| value.is_finite()));
        assert_eq!(single, batch[..4]);

        let status = unsafe { memento_ffi_model_free(handle) };
        assert_eq!(status, MementoFfiStatus::Ok);
        fs::remove_file(path).expect("cleanup");
    }

    #[test]
    fn cancellation_and_errors_are_reported() {
        let path = write_fixture_model();
        let handle = unsafe { load_handle(&path) };
        let mut token = ptr::null_mut();
        let status = unsafe { memento_ffi_cancel_token_new(&raw mut token) };
        assert_eq!(status, MementoFfiStatus::Ok);
        let status = unsafe { memento_ffi_cancel_token_cancel(token) };
        assert_eq!(status, MementoFfiStatus::Ok);
        let mut output = vec![0.0_f32; 4];
        let status = unsafe {
            memento_ffi_embed(
                handle,
                MementoFfiStringView {
                    ptr: b"hello".as_ptr(),
                    len: 5,
                },
                token,
                16,
                output.as_mut_ptr(),
                output.len(),
            )
        };
        assert_eq!(status, MementoFfiStatus::Cancelled);
        assert!(last_error_string().contains("cancelled"));

        let status = unsafe { memento_ffi_vector_validate([1.0_f32, f32::NAN].as_ptr(), 2) };
        assert_eq!(status, MementoFfiStatus::Finite);
        assert!(last_error_string().contains("non-finite"));

        let status = unsafe { memento_ffi_cancel_token_free(token) };
        assert_eq!(status, MementoFfiStatus::Ok);
        let status = unsafe { memento_ffi_model_free(handle) };
        assert_eq!(status, MementoFfiStatus::Ok);
        fs::remove_file(path).expect("cleanup");
    }

    #[test]
    fn vector_cosine_reports_result() {
        let mut cosine_value = 0.0_f32;
        let status = unsafe {
            memento_ffi_vector_cosine(
                [1.0_f32, 0.0].as_ptr(),
                2,
                [0.5_f32, 0.0].as_ptr(),
                2,
                &raw mut cosine_value,
            )
        };
        assert_eq!(status, MementoFfiStatus::Ok);
        assert!((cosine_value - 1.0).abs() < 1e-6);
    }

    #[test]
    fn c_smoke_test_runs_when_compiler_exists() {
        let compiler = ["cc", "clang", "gcc"].into_iter().find(|candidate| {
            Command::new("sh")
                .arg("-c")
                .arg(format!("command -v {candidate}"))
                .status()
                .is_ok_and(|status| status.success())
        });
        let Some(compiler) = compiler else {
            eprintln!("skipping C smoke test; no C compiler found");
            return;
        };

        let workspace_root = PathBuf::from(env!("CARGO_MANIFEST_DIR")).join("../../..");
        let status = Command::new("cargo")
            .arg("build")
            .arg("-p")
            .arg("memento-ffi")
            .current_dir(workspace_root.join("rust"))
            .status()
            .expect("build ffi cdylib");
        assert!(status.success());

        let lib_dir = workspace_root.join("rust/target/debug");
        let header = workspace_root.join("rust/crates/memento-ffi/include/memento_ffi.h");
        let source = workspace_root.join("rust/target/memento_ffi_smoke.c");
        let binary = workspace_root.join("rust/target/memento_ffi_smoke");
        let model_path = write_fixture_model();
        let c_source = format!(
            "#include <math.h>\n#include <stdint.h>\n#include <stdio.h>\n#include <stdlib.h>\n#include \"{}\"\nint main(void) {{\n  const char *path = \"{}\";\n  MementoFfiModelHandle *handle = NULL;\n  if (memento_ffi_model_load_path((const uint8_t *)path, {}u, &handle) != MEMENTO_FFI_STATUS_OK) return 1;\n  float out[4] = {{0}};\n  MementoFfiStringView text = {{(const uint8_t *)\"hello\", 5u}};\n  if (memento_ffi_embed(handle, text, NULL, 16u, out, 4u) != MEMENTO_FFI_STATUS_OK) return 2;\n  if (!isfinite(out[0])) return 3;\n  if (memento_ffi_model_free(handle) != MEMENTO_FFI_STATUS_OK) return 4;\n  return 0;\n}}\n",
            header.display(),
            model_path.display(),
            model_path.as_os_str().to_string_lossy().len(),
        );
        fs::write(&source, c_source).expect("write C source");

        let link_arg = format!("-Wl,-rpath,{}", lib_dir.display());
        let status = Command::new(compiler)
            .arg(&source)
            .arg("-o")
            .arg(&binary)
            .arg("-L")
            .arg(&lib_dir)
            .arg("-lmemento_ffi")
            .arg(&link_arg)
            .status()
            .expect("compile C smoke test");
        assert!(status.success());

        let status = Command::new(&binary).status().expect("run C smoke test");
        assert!(status.success());

        let _ = fs::remove_file(source);
        let _ = fs::remove_file(binary);
        let _ = fs::remove_file(model_path);
    }
}
