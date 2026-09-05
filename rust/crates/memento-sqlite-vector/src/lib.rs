//! `SQLite` vector functions built on shared Memento kernels.
//! MIT-compatible implementation with attribution to `/tmp/go-gte` for the shared vector semantics.

use libc::{c_char, c_int, c_void};
use memento_vector::{cosine, decode_f32le, validate_f32le};
use std::ffi::CString;
use std::ptr;

#[allow(non_camel_case_types)]
type sqlite3 = c_void;
#[allow(non_camel_case_types)]
type sqlite3_context = c_void;
#[allow(non_camel_case_types)]
type sqlite3_value = c_void;

type Sqlite3Callback =
    Option<unsafe extern "C" fn(*mut sqlite3_context, c_int, *mut *mut sqlite3_value)>;

const SQLITE_UTF8: c_int = 1;
const SQLITE_OK: c_int = 0;
const SQLITE_BLOB: c_int = 4;
const SQLITE_NULL: c_int = 5;

extern "C" {
    fn memento_sqlite_api_init(api: *mut c_void) -> c_int;
    fn memento_sqlite_create_function_v2(
        db: *mut sqlite3,
        name: *const c_char,
        argc: c_int,
        flags: c_int,
        app: *mut c_void,
        function: Sqlite3Callback,
        step: Sqlite3Callback,
        final_callback: Option<unsafe extern "C" fn(*mut sqlite3_context)>,
        destroy: Option<unsafe extern "C" fn(*mut c_void)>,
    ) -> c_int;
    fn memento_sqlite_result_int(context: *mut sqlite3_context, value: c_int);
    fn memento_sqlite_result_double(context: *mut sqlite3_context, value: f64);
    fn memento_sqlite_result_null(context: *mut sqlite3_context);
    fn memento_sqlite_result_error(
        context: *mut sqlite3_context,
        message: *const c_char,
        length: c_int,
    );
    fn memento_sqlite_value_type(value: *mut sqlite3_value) -> c_int;
    fn memento_sqlite_value_blob(value: *mut sqlite3_value) -> *const c_void;
    fn memento_sqlite_value_bytes(value: *mut sqlite3_value) -> c_int;
}

unsafe fn blob_arg(value: *mut sqlite3_value) -> Option<&'static [u8]> {
    if memento_sqlite_value_type(value) == SQLITE_NULL {
        return None;
    }
    if memento_sqlite_value_type(value) != SQLITE_BLOB {
        // Deliberately invalid f32 byte length: SQL types are not empty vectors.
        return Some(&[0]);
    }
    let ptr = memento_sqlite_value_blob(value).cast::<u8>();
    let len = memento_sqlite_value_bytes(value) as usize;
    if len == 0 {
        return Some(&[]);
    }
    if ptr.is_null() {
        return None;
    }
    Some(std::slice::from_raw_parts(ptr, len))
}

unsafe extern "C" fn vector_is_valid_fn(
    ctx: *mut sqlite3_context,
    arg_count: c_int,
    values: *mut *mut sqlite3_value,
) {
    if arg_count != 1 {
        memento_sqlite_result_null(ctx);
        return;
    }
    let value = *values;
    match blob_arg(value) {
        None => memento_sqlite_result_null(ctx),
        Some(blob) => memento_sqlite_result_int(ctx, i32::from(validate_f32le(blob).is_ok())),
    }
}

unsafe extern "C" fn vector_dimensions_fn(
    ctx: *mut sqlite3_context,
    arg_count: c_int,
    values: *mut *mut sqlite3_value,
) {
    if arg_count != 1 {
        memento_sqlite_result_null(ctx);
        return;
    }
    match blob_arg(*values) {
        None => memento_sqlite_result_null(ctx),
        Some(blob) => match validate_f32le(blob) {
            Ok(dim) => match c_int::try_from(dim) {
                Ok(value) => memento_sqlite_result_int(ctx, value),
                Err(_) => memento_sqlite_result_null(ctx),
            },
            Err(_) => memento_sqlite_result_null(ctx),
        },
    }
}

unsafe extern "C" fn vector_cosine_fn(
    ctx: *mut sqlite3_context,
    arg_count: c_int,
    values: *mut *mut sqlite3_value,
) {
    if arg_count != 2 {
        memento_sqlite_result_null(ctx);
        return;
    }
    let value_slice = std::slice::from_raw_parts(values, 2);
    let Some(left_blob) = blob_arg(value_slice[0]) else {
        memento_sqlite_result_null(ctx);
        return;
    };
    let Some(right_blob) = blob_arg(value_slice[1]) else {
        memento_sqlite_result_null(ctx);
        return;
    };
    let result = decode_f32le(left_blob)
        .and_then(|left| decode_f32le(right_blob).and_then(|right| cosine(&left, &right)));
    match result {
        Ok(value) => memento_sqlite_result_double(ctx, f64::from(value)),
        Err(err) => {
            let msg = CString::new(err.to_string()).expect("cstring");
            memento_sqlite_result_error(ctx, msg.as_ptr(), -1);
        }
    }
}

#[no_mangle]
/// # Safety
///
/// `db` must be a valid `SQLite` connection pointer supplied by `SQLite` while loading
/// this extension. The remaining pointers are managed by `SQLite` and are not retained.
pub unsafe extern "C" fn sqlite3_mementosqlitevector_init(
    db: *mut sqlite3,
    _pz_err_msg: *mut *mut c_char,
    p_api: *mut c_void,
) -> c_int {
    let init_rc = memento_sqlite_api_init(p_api);
    if init_rc != SQLITE_OK {
        return init_rc;
    }
    for (name, argc, func) in [
        (
            "vector_is_valid",
            1,
            vector_is_valid_fn as unsafe extern "C" fn(_, _, _),
        ),
        (
            "vector_dimensions",
            1,
            vector_dimensions_fn as unsafe extern "C" fn(_, _, _),
        ),
        (
            "vector_cosine",
            2,
            vector_cosine_fn as unsafe extern "C" fn(_, _, _),
        ),
    ] {
        let cname = CString::new(name).expect("cstring");
        let rc = memento_sqlite_create_function_v2(
            db,
            cname.as_ptr(),
            argc,
            SQLITE_UTF8,
            ptr::null_mut(),
            Some(func),
            None,
            None,
            None,
        );
        if rc != SQLITE_OK {
            return rc;
        }
    }
    SQLITE_OK
}
