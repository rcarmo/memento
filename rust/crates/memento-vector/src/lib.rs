//! Shared vector helpers for Memento semantic search.
//! Adapted from the MIT-licensed `/tmp/go-gte` reference implementation.

use thiserror::Error;

const ROW_BLOCK: usize = 4;
const SCALAR_OUT_BLOCK: usize = 8;

#[derive(Debug, Error, Clone, PartialEq, Eq)]
pub enum VectorError {
    #[error("f32le blob length {0} is not a multiple of 4")]
    InvalidByteLength(usize),
    #[error("vector contains non-finite value at index {index}")]
    NonFinite { index: usize },
    #[error("vector dimension mismatch: {left} vs {right}")]
    DimensionMismatch { left: usize, right: usize },
    #[error("{name} length {len} does not match shape {rows}x{cols}")]
    InvalidMatrixShape {
        name: &'static str,
        len: usize,
        rows: usize,
        cols: usize,
    },
    #[error("{name} shape {rows}x{cols} overflows usize")]
    ShapeOverflow {
        name: &'static str,
        rows: usize,
        cols: usize,
    },
    #[error("zero-norm vector")]
    ZeroNorm,
}

pub fn validate_f32le(blob: &[u8]) -> Result<usize, VectorError> {
    if !blob.len().is_multiple_of(4) {
        return Err(VectorError::InvalidByteLength(blob.len()));
    }
    for (index, chunk) in blob.chunks_exact(4).enumerate() {
        let value = f32::from_le_bytes(chunk.try_into().expect("4-byte chunk"));
        if !value.is_finite() {
            return Err(VectorError::NonFinite { index });
        }
    }
    Ok(blob.len() / 4)
}

pub fn decode_f32le(blob: &[u8]) -> Result<Vec<f32>, VectorError> {
    validate_f32le(blob)?;
    Ok(blob
        .chunks_exact(4)
        .map(|chunk| f32::from_le_bytes(chunk.try_into().expect("4-byte chunk")))
        .collect())
}

pub fn encode_f32le(values: &[f32]) -> Result<Vec<u8>, VectorError> {
    let mut out = Vec::with_capacity(values.len() * 4);
    for (index, value) in values.iter().copied().enumerate() {
        if !value.is_finite() {
            return Err(VectorError::NonFinite { index });
        }
        out.extend_from_slice(&value.to_le_bytes());
    }
    Ok(out)
}

pub fn dot(left: &[f32], right: &[f32]) -> Result<f32, VectorError> {
    if left.len() != right.len() {
        return Err(VectorError::DimensionMismatch {
            left: left.len(),
            right: right.len(),
        });
    }
    Ok(dot_runtime(left, right))
}

pub fn axpy(alpha: f32, values: &[f32], output: &mut [f32]) -> Result<(), VectorError> {
    if values.len() != output.len() {
        return Err(VectorError::DimensionMismatch {
            left: values.len(),
            right: output.len(),
        });
    }
    axpy_runtime(alpha, values, output);
    Ok(())
}

pub fn linear_out_in(
    input: &[f32],
    rows: usize,
    in_features: usize,
    weights: &[f32],
    out_features: usize,
    bias: Option<&[f32]>,
) -> Result<Vec<f32>, VectorError> {
    validate_linear_out_in_shapes(input, rows, in_features, weights, out_features, bias)?;
    let mut output = vec![0.0; checked_shape_len("output", rows, out_features)?];
    linear_out_in_runtime(
        input,
        rows,
        in_features,
        weights,
        out_features,
        bias,
        &mut output,
    );
    Ok(output)
}

pub fn linear_in_out(
    input: &[f32],
    rows: usize,
    in_features: usize,
    weights: &[f32],
    out_features: usize,
    bias: Option<&[f32]>,
) -> Result<Vec<f32>, VectorError> {
    validate_linear_in_out_shapes(input, rows, in_features, weights, out_features, bias)?;
    let mut output = vec![0.0; checked_shape_len("output", rows, out_features)?];
    linear_in_out_runtime(
        input,
        rows,
        in_features,
        weights,
        out_features,
        bias,
        &mut output,
    );
    Ok(output)
}

pub fn cosine(left: &[f32], right: &[f32]) -> Result<f32, VectorError> {
    if left.len() != right.len() {
        return Err(VectorError::DimensionMismatch {
            left: left.len(),
            right: right.len(),
        });
    }
    let ll = dot_runtime(left, left);
    let rr = dot_runtime(right, right);
    if ll <= 0.0 || rr <= 0.0 {
        return Err(VectorError::ZeroNorm);
    }
    Ok(dot_runtime(left, right) / (ll.sqrt() * rr.sqrt()))
}

fn checked_shape_len(name: &'static str, rows: usize, cols: usize) -> Result<usize, VectorError> {
    rows.checked_mul(cols)
        .ok_or(VectorError::ShapeOverflow { name, rows, cols })
}

fn validate_matrix_shape(
    name: &'static str,
    values: &[f32],
    rows: usize,
    cols: usize,
) -> Result<(), VectorError> {
    let expected = checked_shape_len(name, rows, cols)?;
    if values.len() != expected {
        return Err(VectorError::InvalidMatrixShape {
            name,
            len: values.len(),
            rows,
            cols,
        });
    }
    Ok(())
}

fn validate_linear_out_in_shapes(
    input: &[f32],
    rows: usize,
    in_features: usize,
    weights: &[f32],
    out_features: usize,
    bias: Option<&[f32]>,
) -> Result<(), VectorError> {
    validate_matrix_shape("input", input, rows, in_features)?;
    validate_matrix_shape("weights", weights, out_features, in_features)?;
    validate_bias_shape(bias, out_features)
}

fn validate_linear_in_out_shapes(
    input: &[f32],
    rows: usize,
    in_features: usize,
    weights: &[f32],
    out_features: usize,
    bias: Option<&[f32]>,
) -> Result<(), VectorError> {
    validate_matrix_shape("input", input, rows, in_features)?;
    validate_matrix_shape("weights", weights, in_features, out_features)?;
    validate_bias_shape(bias, out_features)
}

fn validate_bias_shape(bias: Option<&[f32]>, out_features: usize) -> Result<(), VectorError> {
    if let Some(bias) = bias {
        if bias.len() != out_features {
            return Err(VectorError::DimensionMismatch {
                left: bias.len(),
                right: out_features,
            });
        }
    }
    Ok(())
}

fn dot_scalar(left: &[f32], right: &[f32]) -> f32 {
    left.iter().zip(right).map(|(a, b)| a * b).sum()
}

fn axpy_scalar(alpha: f32, values: &[f32], output: &mut [f32]) {
    for (dst, value) in output.iter_mut().zip(values) {
        *dst += alpha * value;
    }
}

fn linear_out_in_scalar(
    input: &[f32],
    rows: usize,
    in_features: usize,
    weights: &[f32],
    out_features: usize,
    bias: Option<&[f32]>,
    output: &mut [f32],
) {
    for row_base in (0..rows).step_by(ROW_BLOCK) {
        let block_rows = (rows - row_base).min(ROW_BLOCK);
        linear_out_in_row_block_scalar(
            input,
            row_base,
            block_rows,
            in_features,
            weights,
            out_features,
            bias,
            output,
        );
    }
}

#[allow(
    clippy::too_many_arguments,
    reason = "this scalar microkernel mirrors the hot-path matrix slices and bounds; bundling them would obscure the fallback implementation"
)]
fn linear_out_in_row_block_scalar(
    input: &[f32],
    row_base: usize,
    block_rows: usize,
    in_features: usize,
    weights: &[f32],
    out_features: usize,
    bias: Option<&[f32]>,
    output: &mut [f32],
) {
    for out in 0..out_features {
        let mut sums = [0.0; ROW_BLOCK];
        if let Some(bias) = bias {
            sums[..block_rows].fill(bias[out]);
        }
        let weight_row = &weights[out * in_features..(out + 1) * in_features];
        for (k, &weight) in weight_row.iter().enumerate() {
            for (row_offset, sum) in sums.iter_mut().enumerate().take(block_rows) {
                *sum += input[(row_base + row_offset) * in_features + k] * weight;
            }
        }
        for (row_offset, &sum) in sums.iter().enumerate().take(block_rows) {
            output[(row_base + row_offset) * out_features + out] = sum;
        }
    }
}

fn linear_in_out_scalar(
    input: &[f32],
    rows: usize,
    in_features: usize,
    weights: &[f32],
    out_features: usize,
    bias: Option<&[f32]>,
    output: &mut [f32],
) {
    for row_base in (0..rows).step_by(ROW_BLOCK) {
        let block_rows = (rows - row_base).min(ROW_BLOCK);
        linear_in_out_row_block_scalar_range(
            input,
            row_base,
            block_rows,
            in_features,
            weights,
            out_features,
            bias,
            output,
            0,
            out_features,
        );
    }
}

#[allow(
    clippy::too_many_arguments,
    reason = "this scalar microkernel mirrors the SIMD fallback inputs; grouping parameters would make the hot-path call sites less direct"
)]
fn linear_in_out_row_block_scalar_range(
    input: &[f32],
    row_base: usize,
    block_rows: usize,
    in_features: usize,
    weights: &[f32],
    out_features: usize,
    bias: Option<&[f32]>,
    output: &mut [f32],
    out_start: usize,
    out_end: usize,
) {
    let mut out_base = out_start;
    while out_base < out_end {
        let width = (out_end - out_base).min(SCALAR_OUT_BLOCK);
        let mut acc = [[0.0; SCALAR_OUT_BLOCK]; ROW_BLOCK];
        if let Some(bias) = bias {
            let bias_chunk = &bias[out_base..out_base + width];
            for row_acc in acc.iter_mut().take(block_rows) {
                row_acc[..width].copy_from_slice(bias_chunk);
            }
        }
        for k in 0..in_features {
            let weight_offset = k * out_features + out_base;
            let weight_chunk = &weights[weight_offset..weight_offset + width];
            for (row_offset, row_acc) in acc.iter_mut().enumerate().take(block_rows) {
                let value = input[(row_base + row_offset) * in_features + k];
                for (acc_lane, &weight) in row_acc[..width].iter_mut().zip(weight_chunk.iter()) {
                    *acc_lane += value * weight;
                }
            }
        }
        for (row_offset, row_acc) in acc.iter().enumerate().take(block_rows) {
            let dst_offset = (row_base + row_offset) * out_features + out_base;
            output[dst_offset..dst_offset + width].copy_from_slice(&row_acc[..width]);
        }
        out_base += width;
    }
}

#[cfg(target_arch = "x86_64")]
fn axpy_runtime(alpha: f32, values: &[f32], output: &mut [f32]) {
    if std::is_x86_feature_detected!("avx2") && std::is_x86_feature_detected!("fma") {
        // SAFETY: feature detection guarantees required instructions.
        unsafe { return axpy_x86_avx2_fma(alpha, values, output) }
    }
    axpy_scalar(alpha, values, output);
}

#[cfg(target_arch = "aarch64")]
fn axpy_runtime(alpha: f32, values: &[f32], output: &mut [f32]) {
    if std::arch::is_aarch64_feature_detected!("neon") {
        // SAFETY: feature detection guarantees required instructions.
        unsafe { return axpy_neon(alpha, values, output) }
    }
    axpy_scalar(alpha, values, output);
}

#[cfg(not(any(target_arch = "x86_64", target_arch = "aarch64")))]
fn axpy_runtime(alpha: f32, values: &[f32], output: &mut [f32]) {
    axpy_scalar(alpha, values, output);
}

#[cfg(target_arch = "x86_64")]
fn dot_runtime(left: &[f32], right: &[f32]) -> f32 {
    if std::is_x86_feature_detected!("avx2") && std::is_x86_feature_detected!("fma") {
        // SAFETY: feature detection guarantees required instructions.
        unsafe { return dot_x86_avx2_fma(left, right) }
    }
    dot_scalar(left, right)
}

#[cfg(target_arch = "aarch64")]
fn dot_runtime(left: &[f32], right: &[f32]) -> f32 {
    if std::arch::is_aarch64_feature_detected!("neon") {
        // SAFETY: feature detection guarantees required instructions.
        unsafe { return dot_neon(left, right) }
    }
    dot_scalar(left, right)
}

#[cfg(not(any(target_arch = "x86_64", target_arch = "aarch64")))]
fn dot_runtime(left: &[f32], right: &[f32]) -> f32 {
    dot_scalar(left, right)
}

#[cfg(target_arch = "x86_64")]
fn linear_out_in_runtime(
    input: &[f32],
    rows: usize,
    in_features: usize,
    weights: &[f32],
    out_features: usize,
    bias: Option<&[f32]>,
    output: &mut [f32],
) {
    if std::is_x86_feature_detected!("avx2") && std::is_x86_feature_detected!("fma") {
        // SAFETY: feature detection guarantees required instructions.
        unsafe {
            return linear_out_in_x86_avx2_fma(
                input,
                rows,
                in_features,
                weights,
                out_features,
                bias,
                output,
            );
        }
    }
    linear_out_in_scalar(
        input,
        rows,
        in_features,
        weights,
        out_features,
        bias,
        output,
    );
}

#[cfg(target_arch = "aarch64")]
fn linear_out_in_runtime(
    input: &[f32],
    rows: usize,
    in_features: usize,
    weights: &[f32],
    out_features: usize,
    bias: Option<&[f32]>,
    output: &mut [f32],
) {
    if std::arch::is_aarch64_feature_detected!("neon") {
        // SAFETY: feature detection guarantees required instructions.
        unsafe {
            return linear_out_in_neon(
                input,
                rows,
                in_features,
                weights,
                out_features,
                bias,
                output,
            );
        }
    }
    linear_out_in_scalar(
        input,
        rows,
        in_features,
        weights,
        out_features,
        bias,
        output,
    );
}

#[cfg(not(any(target_arch = "x86_64", target_arch = "aarch64")))]
fn linear_out_in_runtime(
    input: &[f32],
    rows: usize,
    in_features: usize,
    weights: &[f32],
    out_features: usize,
    bias: Option<&[f32]>,
    output: &mut [f32],
) {
    linear_out_in_scalar(
        input,
        rows,
        in_features,
        weights,
        out_features,
        bias,
        output,
    );
}

#[cfg(target_arch = "x86_64")]
fn linear_in_out_runtime(
    input: &[f32],
    rows: usize,
    in_features: usize,
    weights: &[f32],
    out_features: usize,
    bias: Option<&[f32]>,
    output: &mut [f32],
) {
    if std::is_x86_feature_detected!("avx2") && std::is_x86_feature_detected!("fma") {
        // SAFETY: feature detection guarantees required instructions.
        unsafe {
            return linear_in_out_x86_avx2_fma(
                input,
                rows,
                in_features,
                weights,
                out_features,
                bias,
                output,
            );
        }
    }
    linear_in_out_scalar(
        input,
        rows,
        in_features,
        weights,
        out_features,
        bias,
        output,
    );
}

#[cfg(target_arch = "aarch64")]
fn linear_in_out_runtime(
    input: &[f32],
    rows: usize,
    in_features: usize,
    weights: &[f32],
    out_features: usize,
    bias: Option<&[f32]>,
    output: &mut [f32],
) {
    if std::arch::is_aarch64_feature_detected!("neon") {
        // SAFETY: feature detection guarantees required instructions.
        unsafe {
            return linear_in_out_neon(
                input,
                rows,
                in_features,
                weights,
                out_features,
                bias,
                output,
            );
        }
    }
    linear_in_out_scalar(
        input,
        rows,
        in_features,
        weights,
        out_features,
        bias,
        output,
    );
}

#[cfg(not(any(target_arch = "x86_64", target_arch = "aarch64")))]
fn linear_in_out_runtime(
    input: &[f32],
    rows: usize,
    in_features: usize,
    weights: &[f32],
    out_features: usize,
    bias: Option<&[f32]>,
    output: &mut [f32],
) {
    linear_in_out_scalar(
        input,
        rows,
        in_features,
        weights,
        out_features,
        bias,
        output,
    );
}

#[cfg(target_arch = "x86_64")]
#[target_feature(enable = "avx2")]
#[target_feature(enable = "fma")]
unsafe fn axpy_x86_avx2_fma(alpha: f32, values: &[f32], output: &mut [f32]) {
    use std::arch::x86_64::{_mm256_fmadd_ps, _mm256_loadu_ps, _mm256_set1_ps, _mm256_storeu_ps};
    let factor = _mm256_set1_ps(alpha);
    let chunks = values.len() / 8;
    for i in 0..chunks {
        let offset = i * 8;
        let value = _mm256_loadu_ps(values.as_ptr().add(offset));
        let current = _mm256_loadu_ps(output.as_ptr().add(offset));
        let next = _mm256_fmadd_ps(factor, value, current);
        _mm256_storeu_ps(output.as_mut_ptr().add(offset), next);
    }
    axpy_scalar(alpha, &values[chunks * 8..], &mut output[chunks * 8..]);
}

#[cfg(target_arch = "aarch64")]
#[target_feature(enable = "neon")]
unsafe fn axpy_neon(alpha: f32, values: &[f32], output: &mut [f32]) {
    use std::arch::aarch64::{vdupq_n_f32, vfmaq_f32, vld1q_f32, vst1q_f32};
    let factor = vdupq_n_f32(alpha);
    let chunks = values.len() / 4;
    for i in 0..chunks {
        let offset = i * 4;
        let value = vld1q_f32(values.as_ptr().add(offset));
        let current = vld1q_f32(output.as_ptr().add(offset));
        let next = vfmaq_f32(current, factor, value);
        vst1q_f32(output.as_mut_ptr().add(offset), next);
    }
    axpy_scalar(alpha, &values[chunks * 4..], &mut output[chunks * 4..]);
}

#[cfg(target_arch = "x86_64")]
#[target_feature(enable = "avx2")]
#[target_feature(enable = "fma")]
unsafe fn dot_x86_avx2_fma(left: &[f32], right: &[f32]) -> f32 {
    use std::arch::x86_64::{
        _mm256_fmadd_ps, _mm256_loadu_ps, _mm256_setzero_ps, _mm256_storeu_ps,
    };
    let mut sum = _mm256_setzero_ps();
    let chunks = left.len() / 8;
    for i in 0..chunks {
        let offset = i * 8;
        let a = _mm256_loadu_ps(left.as_ptr().add(offset));
        let b = _mm256_loadu_ps(right.as_ptr().add(offset));
        sum = _mm256_fmadd_ps(a, b, sum);
    }
    let mut lanes = [0f32; 8];
    _mm256_storeu_ps(lanes.as_mut_ptr(), sum);
    let mut total: f32 = lanes.iter().sum();
    for i in (chunks * 8)..left.len() {
        total += left[i] * right[i];
    }
    total
}

#[cfg(target_arch = "aarch64")]
#[target_feature(enable = "neon")]
unsafe fn dot_neon(left: &[f32], right: &[f32]) -> f32 {
    use std::arch::aarch64::{vdupq_n_f32, vfmaq_f32, vld1q_f32, vst1q_f32};
    let mut acc = vdupq_n_f32(0.0);
    let chunks = left.len() / 4;
    for i in 0..chunks {
        let offset = i * 4;
        let a = vld1q_f32(left.as_ptr().add(offset));
        let b = vld1q_f32(right.as_ptr().add(offset));
        acc = vfmaq_f32(acc, a, b);
    }
    let mut lanes = [0f32; 4];
    vst1q_f32(lanes.as_mut_ptr(), acc);
    let mut total: f32 = lanes.iter().sum();
    for i in (chunks * 4)..left.len() {
        total += left[i] * right[i];
    }
    total
}

#[cfg(target_arch = "x86_64")]
#[target_feature(enable = "avx2")]
#[target_feature(enable = "fma")]
unsafe fn linear_out_in_x86_avx2_fma(
    input: &[f32],
    rows: usize,
    in_features: usize,
    weights: &[f32],
    out_features: usize,
    bias: Option<&[f32]>,
    output: &mut [f32],
) {
    use std::arch::x86_64::{_mm_fmadd_ps, _mm_set1_ps, _mm_set_ps, _mm_setzero_ps, _mm_storeu_ps};

    let row_limit = rows / ROW_BLOCK * ROW_BLOCK;
    for row_base in (0..row_limit).step_by(ROW_BLOCK) {
        let input_ptr0 = input.as_ptr().add(row_base * in_features);
        let input_ptr1 = input_ptr0.add(in_features);
        let input_ptr2 = input_ptr1.add(in_features);
        let input_ptr3 = input_ptr2.add(in_features);
        for out in 0..out_features {
            let weight_row = weights.as_ptr().add(out * in_features);
            let mut acc = if let Some(bias) = bias {
                _mm_set1_ps(bias[out])
            } else {
                _mm_setzero_ps()
            };
            for k in 0..in_features {
                let input_lanes = _mm_set_ps(
                    *input_ptr3.add(k),
                    *input_ptr2.add(k),
                    *input_ptr1.add(k),
                    *input_ptr0.add(k),
                );
                let weight_lanes = _mm_set1_ps(*weight_row.add(k));
                acc = _mm_fmadd_ps(input_lanes, weight_lanes, acc);
            }
            let mut lanes = [0.0; ROW_BLOCK];
            _mm_storeu_ps(lanes.as_mut_ptr(), acc);
            for row_offset in 0..ROW_BLOCK {
                output[(row_base + row_offset) * out_features + out] = lanes[row_offset];
            }
        }
    }
    if row_limit < rows {
        linear_out_in_row_block_scalar(
            input,
            row_limit,
            rows - row_limit,
            in_features,
            weights,
            out_features,
            bias,
            output,
        );
    }
}

#[cfg(target_arch = "aarch64")]
#[target_feature(enable = "neon")]
unsafe fn linear_out_in_neon(
    input: &[f32],
    rows: usize,
    in_features: usize,
    weights: &[f32],
    out_features: usize,
    bias: Option<&[f32]>,
    output: &mut [f32],
) {
    use std::arch::aarch64::{vdupq_n_f32, vfmaq_f32, vld1q_f32, vst1q_f32};

    let row_limit = rows / ROW_BLOCK * ROW_BLOCK;
    for row_base in (0..row_limit).step_by(ROW_BLOCK) {
        let input_ptr0 = input.as_ptr().add(row_base * in_features);
        let input_ptr1 = input_ptr0.add(in_features);
        let input_ptr2 = input_ptr1.add(in_features);
        let input_ptr3 = input_ptr2.add(in_features);
        for out in 0..out_features {
            let weight_row = weights.as_ptr().add(out * in_features);
            let mut acc = if let Some(bias) = bias {
                vdupq_n_f32(bias[out])
            } else {
                vdupq_n_f32(0.0)
            };
            for k in 0..in_features {
                let input_lanes = [
                    *input_ptr0.add(k),
                    *input_ptr1.add(k),
                    *input_ptr2.add(k),
                    *input_ptr3.add(k),
                ];
                let inputs = vld1q_f32(input_lanes.as_ptr());
                let weight_lanes = vdupq_n_f32(*weight_row.add(k));
                acc = vfmaq_f32(acc, inputs, weight_lanes);
            }
            let mut lanes = [0.0; ROW_BLOCK];
            vst1q_f32(lanes.as_mut_ptr(), acc);
            for row_offset in 0..ROW_BLOCK {
                output[(row_base + row_offset) * out_features + out] = lanes[row_offset];
            }
        }
    }
    if row_limit < rows {
        linear_out_in_row_block_scalar(
            input,
            row_limit,
            rows - row_limit,
            in_features,
            weights,
            out_features,
            bias,
            output,
        );
    }
}

#[cfg(target_arch = "x86_64")]
#[target_feature(enable = "avx2")]
#[target_feature(enable = "fma")]
unsafe fn linear_in_out_x86_avx2_fma(
    input: &[f32],
    rows: usize,
    in_features: usize,
    weights: &[f32],
    out_features: usize,
    bias: Option<&[f32]>,
    output: &mut [f32],
) {
    use std::arch::x86_64::{
        _mm256_fmadd_ps, _mm256_loadu_ps, _mm256_set1_ps, _mm256_setzero_ps, _mm256_storeu_ps,
    };

    let row_limit = rows / ROW_BLOCK * ROW_BLOCK;
    for row_base in (0..row_limit).step_by(ROW_BLOCK) {
        let input_ptr0 = input.as_ptr().add(row_base * in_features);
        let input_ptr1 = input_ptr0.add(in_features);
        let input_ptr2 = input_ptr1.add(in_features);
        let input_ptr3 = input_ptr2.add(in_features);
        let mut out_base = 0;
        while out_base + 8 <= out_features {
            let bias_lanes = if let Some(bias) = bias {
                _mm256_loadu_ps(bias.as_ptr().add(out_base))
            } else {
                _mm256_setzero_ps()
            };
            let mut acc0 = bias_lanes;
            let mut acc1 = bias_lanes;
            let mut acc2 = bias_lanes;
            let mut acc3 = bias_lanes;
            for k in 0..in_features {
                let weight_lanes =
                    _mm256_loadu_ps(weights.as_ptr().add(k * out_features + out_base));
                acc0 = _mm256_fmadd_ps(_mm256_set1_ps(*input_ptr0.add(k)), weight_lanes, acc0);
                acc1 = _mm256_fmadd_ps(_mm256_set1_ps(*input_ptr1.add(k)), weight_lanes, acc1);
                acc2 = _mm256_fmadd_ps(_mm256_set1_ps(*input_ptr2.add(k)), weight_lanes, acc2);
                acc3 = _mm256_fmadd_ps(_mm256_set1_ps(*input_ptr3.add(k)), weight_lanes, acc3);
            }
            let dst0 = output.as_mut_ptr().add(row_base * out_features + out_base);
            let dst1 = output
                .as_mut_ptr()
                .add((row_base + 1) * out_features + out_base);
            let dst2 = output
                .as_mut_ptr()
                .add((row_base + 2) * out_features + out_base);
            let dst3 = output
                .as_mut_ptr()
                .add((row_base + 3) * out_features + out_base);
            _mm256_storeu_ps(dst0, acc0);
            _mm256_storeu_ps(dst1, acc1);
            _mm256_storeu_ps(dst2, acc2);
            _mm256_storeu_ps(dst3, acc3);
            out_base += 8;
        }
        if out_base < out_features {
            linear_in_out_row_block_scalar_range(
                input,
                row_base,
                ROW_BLOCK,
                in_features,
                weights,
                out_features,
                bias,
                output,
                out_base,
                out_features,
            );
        }
    }
    for row_base in (row_limit..rows).step_by(ROW_BLOCK) {
        let block_rows = (rows - row_base).min(ROW_BLOCK);
        linear_in_out_row_block_scalar_range(
            input,
            row_base,
            block_rows,
            in_features,
            weights,
            out_features,
            bias,
            output,
            0,
            out_features,
        );
    }
}

#[cfg(target_arch = "aarch64")]
#[target_feature(enable = "neon")]
unsafe fn linear_in_out_neon(
    input: &[f32],
    rows: usize,
    in_features: usize,
    weights: &[f32],
    out_features: usize,
    bias: Option<&[f32]>,
    output: &mut [f32],
) {
    use std::arch::aarch64::{vdupq_n_f32, vfmaq_f32, vld1q_f32, vst1q_f32};

    let row_limit = rows / ROW_BLOCK * ROW_BLOCK;
    for row_base in (0..row_limit).step_by(ROW_BLOCK) {
        let input_ptr0 = input.as_ptr().add(row_base * in_features);
        let input_ptr1 = input_ptr0.add(in_features);
        let input_ptr2 = input_ptr1.add(in_features);
        let input_ptr3 = input_ptr2.add(in_features);
        let mut out_base = 0;
        while out_base + 4 <= out_features {
            let bias_lanes = if let Some(bias) = bias {
                vld1q_f32(bias.as_ptr().add(out_base))
            } else {
                vdupq_n_f32(0.0)
            };
            let mut acc0 = bias_lanes;
            let mut acc1 = bias_lanes;
            let mut acc2 = bias_lanes;
            let mut acc3 = bias_lanes;
            for k in 0..in_features {
                let weight_lanes = vld1q_f32(weights.as_ptr().add(k * out_features + out_base));
                acc0 = vfmaq_f32(acc0, vdupq_n_f32(*input_ptr0.add(k)), weight_lanes);
                acc1 = vfmaq_f32(acc1, vdupq_n_f32(*input_ptr1.add(k)), weight_lanes);
                acc2 = vfmaq_f32(acc2, vdupq_n_f32(*input_ptr2.add(k)), weight_lanes);
                acc3 = vfmaq_f32(acc3, vdupq_n_f32(*input_ptr3.add(k)), weight_lanes);
            }
            let dst0 = output.as_mut_ptr().add(row_base * out_features + out_base);
            let dst1 = output
                .as_mut_ptr()
                .add((row_base + 1) * out_features + out_base);
            let dst2 = output
                .as_mut_ptr()
                .add((row_base + 2) * out_features + out_base);
            let dst3 = output
                .as_mut_ptr()
                .add((row_base + 3) * out_features + out_base);
            vst1q_f32(dst0, acc0);
            vst1q_f32(dst1, acc1);
            vst1q_f32(dst2, acc2);
            vst1q_f32(dst3, acc3);
            out_base += 4;
        }
        if out_base < out_features {
            linear_in_out_row_block_scalar_range(
                input,
                row_base,
                ROW_BLOCK,
                in_features,
                weights,
                out_features,
                bias,
                output,
                out_base,
                out_features,
            );
        }
    }
    for row_base in (row_limit..rows).step_by(ROW_BLOCK) {
        let block_rows = (rows - row_base).min(ROW_BLOCK);
        linear_in_out_row_block_scalar_range(
            input,
            row_base,
            block_rows,
            in_features,
            weights,
            out_features,
            bias,
            output,
            0,
            out_features,
        );
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn reference_linear_out_in(
        input: &[f32],
        rows: usize,
        in_features: usize,
        weights: &[f32],
        out_features: usize,
        bias: Option<&[f32]>,
    ) -> Vec<f32> {
        let mut output = vec![0.0; rows * out_features];
        for row in 0..rows {
            for out in 0..out_features {
                let mut sum = bias.map_or(0.0, |bias| bias[out]);
                for k in 0..in_features {
                    sum += input[row * in_features + k] * weights[out * in_features + k];
                }
                output[row * out_features + out] = sum;
            }
        }
        output
    }

    fn reference_linear_in_out(
        input: &[f32],
        rows: usize,
        in_features: usize,
        weights: &[f32],
        out_features: usize,
        bias: Option<&[f32]>,
    ) -> Vec<f32> {
        let mut output = vec![0.0; rows * out_features];
        for row in 0..rows {
            for out in 0..out_features {
                let mut sum = bias.map_or(0.0, |bias| bias[out]);
                for k in 0..in_features {
                    sum += input[row * in_features + k] * weights[k * out_features + out];
                }
                output[row * out_features + out] = sum;
            }
        }
        output
    }

    fn transpose_out_in(weights: &[f32], out_features: usize, in_features: usize) -> Vec<f32> {
        let mut transposed = vec![0.0; weights.len()];
        for out in 0..out_features {
            for k in 0..in_features {
                transposed[k * out_features + out] = weights[out * in_features + k];
            }
        }
        transposed
    }

    fn approx_eq(left: &[f32], right: &[f32], tolerance: f32) {
        assert_eq!(left.len(), right.len());
        for (index, (&lhs, &rhs)) in left.iter().zip(right).enumerate() {
            let delta = (lhs - rhs).abs();
            assert!(
                delta <= tolerance,
                "index {index}: left={lhs}, right={rhs}, delta={delta}"
            );
        }
    }

    fn generated_values(len: usize, scale: f32) -> Vec<f32> {
        (0..len)
            .map(|index| (((index * 17 + 5) % 29) as f32 - 14.0) * scale)
            .collect()
    }

    #[test]
    fn validates_f32le() {
        let blob = encode_f32le(&[1.0, 2.0, 3.5]).expect("encode");
        assert_eq!(validate_f32le(&blob), Ok(3));
    }

    #[test]
    fn rejects_non_finite() {
        let mut blob = Vec::new();
        blob.extend_from_slice(&1.0f32.to_le_bytes());
        blob.extend_from_slice(&f32::NAN.to_le_bytes());
        assert!(matches!(
            validate_f32le(&blob),
            Err(VectorError::NonFinite { index: 1 })
        ));
    }

    #[test]
    fn cosine_works() {
        let sim = cosine(&[1.0, 0.0], &[0.5, 0.0]).expect("cosine");
        assert!((sim - 1.0).abs() < 1e-6);
    }

    #[test]
    fn linear_out_in_exact() {
        let input = [1.0, 2.0, 3.0, 4.0, 5.0, 6.0];
        let weights = [10.0, 20.0, 30.0, 1.0, 2.0, 3.0];
        let bias = [1.0, -1.0];
        let output = linear_out_in(&input, 2, 3, &weights, 2, Some(&bias)).expect("linear_out_in");
        assert_eq!(output, vec![141.0, 13.0, 321.0, 31.0]);
    }

    #[test]
    fn linear_in_out_exact() {
        let input = [1.0, 2.0, 3.0, 4.0, 5.0, 6.0];
        let weights_out_in = [10.0, 20.0, 30.0, 1.0, 2.0, 3.0];
        let weights = transpose_out_in(&weights_out_in, 2, 3);
        let bias = [1.0, -1.0];
        let output = linear_in_out(&input, 2, 3, &weights, 2, Some(&bias)).expect("linear_in_out");
        assert_eq!(output, vec![141.0, 13.0, 321.0, 31.0]);
    }

    #[test]
    fn linear_out_in_matches_reference_with_bias_and_row_tail() {
        let rows = 5;
        let in_features = 7;
        let out_features = 3;
        let input = generated_values(rows * in_features, 0.25);
        let weights = generated_values(out_features * in_features, -0.125);
        let bias = [0.5, -1.0, 1.5];
        let expected = reference_linear_out_in(
            &input,
            rows,
            in_features,
            &weights,
            out_features,
            Some(&bias),
        );
        let actual = linear_out_in(
            &input,
            rows,
            in_features,
            &weights,
            out_features,
            Some(&bias),
        )
        .expect("linear_out_in");
        approx_eq(&actual, &expected, 1e-5);
    }

    #[test]
    fn linear_in_out_matches_reference_with_bias_and_output_tail() {
        let rows = 5;
        let in_features = 7;
        let out_features = 10;
        let input = generated_values(rows * in_features, 0.125);
        let weights = generated_values(in_features * out_features, 0.0625);
        let bias = generated_values(out_features, 0.5);
        let expected = reference_linear_in_out(
            &input,
            rows,
            in_features,
            &weights,
            out_features,
            Some(&bias),
        );
        let actual = linear_in_out(
            &input,
            rows,
            in_features,
            &weights,
            out_features,
            Some(&bias),
        )
        .expect("linear_in_out");
        approx_eq(&actual, &expected, 1e-5);
    }

    #[test]
    fn linear_out_in_matches_reference_for_single_row() {
        let rows = 1;
        let in_features = 9;
        let out_features = 5;
        let input = generated_values(rows * in_features, -0.2);
        let weights = generated_values(out_features * in_features, 0.15);
        let bias = generated_values(out_features, -0.3);
        let expected = reference_linear_out_in(
            &input,
            rows,
            in_features,
            &weights,
            out_features,
            Some(&bias),
        );
        let actual = linear_out_in(
            &input,
            rows,
            in_features,
            &weights,
            out_features,
            Some(&bias),
        )
        .expect("linear_out_in");
        approx_eq(&actual, &expected, 1e-5);
    }

    #[test]
    fn linear_in_out_matches_reference_for_single_row() {
        let rows = 1;
        let in_features = 9;
        let out_features = 10;
        let input = generated_values(rows * in_features, 0.2);
        let weights = generated_values(in_features * out_features, -0.1);
        let bias = generated_values(out_features, 0.3);
        let expected = reference_linear_in_out(
            &input,
            rows,
            in_features,
            &weights,
            out_features,
            Some(&bias),
        );
        let actual = linear_in_out(
            &input,
            rows,
            in_features,
            &weights,
            out_features,
            Some(&bias),
        )
        .expect("linear_in_out");
        approx_eq(&actual, &expected, 1e-5);
    }

    #[test]
    fn linear_projection_rejects_bad_shapes() {
        assert!(matches!(
            linear_out_in(&[0.0; 5], 2, 3, &[0.0; 6], 2, None),
            Err(VectorError::InvalidMatrixShape {
                name: "input",
                len: 5,
                rows: 2,
                cols: 3,
            })
        ));
        assert!(matches!(
            linear_out_in(&[0.0; 6], 2, 3, &[0.0; 5], 2, None),
            Err(VectorError::InvalidMatrixShape {
                name: "weights",
                len: 5,
                rows: 2,
                cols: 3,
            })
        ));
        assert!(matches!(
            linear_in_out(&[0.0; 6], 2, 3, &[0.0; 5], 2, None),
            Err(VectorError::InvalidMatrixShape {
                name: "weights",
                len: 5,
                rows: 3,
                cols: 2,
            })
        ));
        assert!(matches!(
            linear_out_in(&[0.0; 6], 2, 3, &[0.0; 6], 2, Some(&[0.0; 3])),
            Err(VectorError::DimensionMismatch { left: 3, right: 2 })
        ));
        assert!(matches!(
            linear_out_in(&[], usize::MAX, 2, &[], 0, None),
            Err(VectorError::ShapeOverflow {
                name: "input",
                rows,
                cols: 2,
            }) if rows == usize::MAX
        ));
    }
}
