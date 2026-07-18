//! Shared vector helpers for Memento semantic search.
//! Adapted from the MIT-licensed `/tmp/go-gte` reference implementation.

use thiserror::Error;

#[derive(Debug, Error, Clone, PartialEq, Eq)]
pub enum VectorError {
    #[error("f32le blob length {0} is not a multiple of 4")]
    InvalidByteLength(usize),
    #[error("vector contains non-finite value at index {index}")]
    NonFinite { index: usize },
    #[error("vector dimension mismatch: {left} vs {right}")]
    DimensionMismatch { left: usize, right: usize },
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

fn dot_scalar(left: &[f32], right: &[f32]) -> f32 {
    left.iter().zip(right).map(|(a, b)| a * b).sum()
}

fn axpy_scalar(alpha: f32, values: &[f32], output: &mut [f32]) {
    for (dst, value) in output.iter_mut().zip(values) {
        *dst += alpha * value;
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

#[cfg(test)]
mod tests {
    use super::*;

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
}
