//! Rust FP32 `GTE1` loader and scalar inference path.
//! Faithful to the MIT-licensed `/tmp/go-gte` reference implementation.

use memmap2::{Mmap, MmapOptions};
use serde::{Deserialize, Serialize};
use std::fmt;
use std::fs::File;
use std::io::Cursor;
use std::ops::{Deref, Range};
use std::path::Path;
use std::sync::Arc;
use thiserror::Error;

const FILE_MAGIC: &[u8; 4] = b"GTE1";
const LAYER_NORM_EPS: f32 = 1e-12;
pub const TOKEN_PAD: u32 = 0;
pub const TOKEN_UNK: u32 = 100;
pub const TOKEN_CLS: u32 = 101;
pub const TOKEN_SEP: u32 = 102;
pub const TOKEN_MASK: u32 = 103;

#[derive(Debug, Error)]
pub enum GteError {
    #[error("io error: {0}")]
    Io(#[from] std::io::Error),
    #[error("invalid model magic")]
    InvalidMagic,
    #[error("invalid model: {0}")]
    InvalidModel(String),
    #[error("output buffer len {got} != hidden size {expected}")]
    OutputLen { got: usize, expected: usize },
    #[error("batch too large: {0}")]
    BatchTooLarge(usize),
    #[error("input too large at index {index}: {len} chars > {max}")]
    InputTooLarge {
        index: usize,
        len: usize,
        max: usize,
    },
    #[error("cancelled at checkpoint: {0}")]
    Cancelled(&'static str),
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ModelConfig {
    pub vocab_size: usize,
    pub hidden_size: usize,
    pub num_layers: usize,
    pub num_heads: usize,
    pub intermediate: usize,
    pub max_seq_len: usize,
}

#[derive(Clone)]
enum WeightData {
    Owned(Vec<f32>),
    Mapped {
        mmap: Arc<Mmap>,
        range: Range<usize>,
    },
}

impl fmt::Debug for WeightData {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Self::Owned(values) => f
                .debug_struct("WeightData")
                .field("storage", &"owned")
                .field("len", &values.len())
                .finish(),
            Self::Mapped { range, .. } => f
                .debug_struct("WeightData")
                .field("storage", &"mmap")
                .field(
                    "len",
                    &((range.end - range.start) / std::mem::size_of::<f32>()),
                )
                .field("range", range)
                .finish(),
        }
    }
}

#[derive(Clone)]
pub struct Weights {
    data: WeightData,
}

impl Weights {
    #[must_use]
    pub fn is_mmap_backed(&self) -> bool {
        matches!(self.data, WeightData::Mapped { .. })
    }

    #[must_use]
    pub fn len(&self) -> usize {
        self.as_slice().len()
    }

    #[must_use]
    pub fn is_empty(&self) -> bool {
        self.as_slice().is_empty()
    }

    #[must_use]
    pub fn as_slice(&self) -> &[f32] {
        match &self.data {
            WeightData::Owned(values) => values.as_slice(),
            WeightData::Mapped { mmap, range } => {
                let len = (range.end - range.start) / std::mem::size_of::<f32>();
                // Safety: construction validates that this range is in bounds, starts at
                // an address aligned for `f32`, contains complete values, and is only used
                // on little-endian targets. The `Arc<Mmap>` keeps the mapping alive.
                #[allow(clippy::cast_ptr_alignment)]
                let ptr = mmap.as_ptr().wrapping_add(range.start).cast::<f32>();
                unsafe { std::slice::from_raw_parts(ptr, len) }
            }
        }
    }

    fn from_le_bytes(bytes: &[u8]) -> Result<Self, GteError> {
        if !bytes.len().is_multiple_of(std::mem::size_of::<f32>()) {
            return Err(GteError::InvalidModel(
                "weight blob len must be a multiple of 4 bytes".to_string(),
            ));
        }
        Ok(Self {
            data: WeightData::Owned(
                bytes
                    .chunks_exact(std::mem::size_of::<f32>())
                    .map(|chunk| f32::from_le_bytes(chunk.try_into().expect("4-byte chunk")))
                    .collect(),
            ),
        })
    }

    fn from_mmap_or_copy(
        mmap: Arc<Mmap>,
        range: Range<usize>,
        bytes: &[u8],
    ) -> Result<Self, GteError> {
        validate_weight_range(mmap.len(), &range)?;
        if cfg!(target_endian = "little") && mapped_range_is_aligned(&mmap, &range) {
            return Ok(Self {
                data: WeightData::Mapped { mmap, range },
            });
        }
        Self::from_le_bytes(bytes)
    }
}

impl fmt::Debug for Weights {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        self.data.fmt(f)
    }
}

impl From<Vec<f32>> for Weights {
    fn from(values: Vec<f32>) -> Self {
        Self {
            data: WeightData::Owned(values),
        }
    }
}

impl AsRef<[f32]> for Weights {
    fn as_ref(&self) -> &[f32] {
        self.as_slice()
    }
}

impl Deref for Weights {
    type Target = [f32];

    fn deref(&self) -> &Self::Target {
        self.as_slice()
    }
}

#[derive(Debug, Clone)]
pub struct LayerWeights {
    pub query_weight: Weights,
    pub query_bias: Weights,
    pub key_weight: Weights,
    pub key_bias: Weights,
    pub value_weight: Weights,
    pub value_bias: Weights,
    pub attn_output_weight: Weights,
    pub attn_output_bias: Weights,
    pub attn_ln_weight: Weights,
    pub attn_ln_bias: Weights,
    pub ffn_inter_weight: Weights,
    pub ffn_inter_bias: Weights,
    pub ffn_output_weight: Weights,
    pub ffn_output_bias: Weights,
    pub ffn_ln_weight: Weights,
    pub ffn_ln_bias: Weights,
}

#[derive(Debug, Clone)]
pub struct Model {
    pub config: ModelConfig,
    pub vocab: Vec<String>,
    token_embeddings: Weights,
    position_embeddings: Weights,
    token_type_embeddings: Weights,
    embed_ln_weight: Weights,
    embed_ln_bias: Weights,
    pub layers: Vec<LayerWeights>,
    pub pooler_weight: Weights,
    pub pooler_bias: Weights,
}

#[derive(Debug, Clone, Copy, Default)]
pub struct BatchOptions {
    pub max_batch: Option<usize>,
    pub max_chars_per_input: Option<usize>,
}

pub type Checkpoint<'a> = &'a mut dyn FnMut(&'static str) -> Result<(), GteError>;

impl Model {
    pub fn from_path(path: impl AsRef<Path>) -> Result<Self, GteError> {
        let file = File::open(path)?;
        // Safety: the file descriptor remains valid for the duration of mapping creation,
        // and the returned `Mmap` owns the mapping independently of the `File`.
        let mmap = Arc::new(unsafe { MmapOptions::new().map(&file)? });
        Self::parse(&mmap[..], Some(&mmap))
    }

    pub fn from_bytes(bytes: &[u8]) -> Result<Self, GteError> {
        Self::parse(bytes, None)
    }

    #[allow(clippy::too_many_lines)]
    fn parse(bytes: &[u8], mmap: Option<&Arc<Mmap>>) -> Result<Self, GteError> {
        let mut cursor = Cursor::new(bytes);
        let magic = read_exact_slice(&mut cursor, FILE_MAGIC.len())?;
        if magic != FILE_MAGIC {
            return Err(GteError::InvalidMagic);
        }
        let config = ModelConfig {
            vocab_size: read_u32(&mut cursor)? as usize,
            hidden_size: read_u32(&mut cursor)? as usize,
            num_layers: read_u32(&mut cursor)? as usize,
            num_heads: read_u32(&mut cursor)? as usize,
            intermediate: read_u32(&mut cursor)? as usize,
            max_seq_len: read_u32(&mut cursor)? as usize,
        };
        if config.vocab_size <= TOKEN_MASK as usize {
            return Err(GteError::InvalidModel(format!(
                "vocab_size must include reserved token id {TOKEN_MASK}"
            )));
        }
        if config.hidden_size == 0 || config.intermediate == 0 {
            return Err(GteError::InvalidModel(
                "hidden_size and intermediate must be positive".to_string(),
            ));
        }
        if config.num_heads == 0 || !config.hidden_size.is_multiple_of(config.num_heads) {
            return Err(GteError::InvalidModel(format!(
                "num_heads must be positive and divide hidden_size: {} heads, {} hidden",
                config.num_heads, config.hidden_size
            )));
        }
        if config.max_seq_len < 2 {
            return Err(GteError::InvalidModel(
                "max_seq_len must be at least 2".to_string(),
            ));
        }

        let mut vocab = Vec::with_capacity(config.vocab_size);
        for _ in 0..config.vocab_size {
            let len = read_u16(&mut cursor)? as usize;
            let bytes = read_exact_slice(&mut cursor, len)?;
            vocab.push(
                String::from_utf8(bytes.to_vec())
                    .map_err(|e| GteError::InvalidModel(e.to_string()))?,
            );
        }

        let token_embeddings =
            read_weights(&mut cursor, config.vocab_size * config.hidden_size, mmap)?;
        let position_embeddings =
            read_weights(&mut cursor, config.max_seq_len * config.hidden_size, mmap)?;
        let token_type_embeddings = read_weights(&mut cursor, 2 * config.hidden_size, mmap)?;
        let embed_ln_weight = read_weights(&mut cursor, config.hidden_size, mmap)?;
        let embed_ln_bias = read_weights(&mut cursor, config.hidden_size, mmap)?;

        let mut layers = Vec::with_capacity(config.num_layers);
        for _ in 0..config.num_layers {
            layers.push(LayerWeights {
                query_weight: read_weights(
                    &mut cursor,
                    config.hidden_size * config.hidden_size,
                    mmap,
                )?,
                query_bias: read_weights(&mut cursor, config.hidden_size, mmap)?,
                key_weight: read_weights(
                    &mut cursor,
                    config.hidden_size * config.hidden_size,
                    mmap,
                )?,
                key_bias: read_weights(&mut cursor, config.hidden_size, mmap)?,
                value_weight: read_weights(
                    &mut cursor,
                    config.hidden_size * config.hidden_size,
                    mmap,
                )?,
                value_bias: read_weights(&mut cursor, config.hidden_size, mmap)?,
                attn_output_weight: read_weights(
                    &mut cursor,
                    config.hidden_size * config.hidden_size,
                    mmap,
                )?,
                attn_output_bias: read_weights(&mut cursor, config.hidden_size, mmap)?,
                attn_ln_weight: read_weights(&mut cursor, config.hidden_size, mmap)?,
                attn_ln_bias: read_weights(&mut cursor, config.hidden_size, mmap)?,
                ffn_inter_weight: read_weights(
                    &mut cursor,
                    config.intermediate * config.hidden_size,
                    mmap,
                )?,
                ffn_inter_bias: read_weights(&mut cursor, config.intermediate, mmap)?,
                ffn_output_weight: read_weights(
                    &mut cursor,
                    config.hidden_size * config.intermediate,
                    mmap,
                )?,
                ffn_output_bias: read_weights(&mut cursor, config.hidden_size, mmap)?,
                ffn_ln_weight: read_weights(&mut cursor, config.hidden_size, mmap)?,
                ffn_ln_bias: read_weights(&mut cursor, config.hidden_size, mmap)?,
            });
        }
        let pooler_weight =
            read_weights(&mut cursor, config.hidden_size * config.hidden_size, mmap)?;
        let pooler_bias = read_weights(&mut cursor, config.hidden_size, mmap)?;

        Ok(Self {
            config,
            vocab,
            token_embeddings,
            position_embeddings,
            token_type_embeddings,
            embed_ln_weight,
            embed_ln_bias,
            layers,
            pooler_weight,
            pooler_bias,
        })
    }

    #[must_use]
    pub fn dim(&self) -> usize {
        self.config.hidden_size
    }

    #[must_use]
    pub fn tokenize(&self, text: &str) -> Vec<u32> {
        let vocab_map = self.vocab_map();
        tokenize_with(text, &vocab_map, self.config.max_seq_len)
    }

    pub fn embed(&self, text: &str) -> Result<Vec<f32>, GteError> {
        let mut output = vec![0.0; self.dim()];
        self.embed_to(text, &mut output, None)?;
        Ok(output)
    }

    pub fn embed_to(
        &self,
        text: &str,
        out: &mut [f32],
        mut checkpoint: Option<Checkpoint<'_>>,
    ) -> Result<(), GteError> {
        if out.len() != self.dim() {
            return Err(GteError::OutputLen {
                got: out.len(),
                expected: self.dim(),
            });
        }
        let token_ids = self.tokenize(text);
        if let Some(cp) = checkpoint.as_mut() {
            cp("tokenized")?;
        }
        let outputs = if let Some(cp) = checkpoint.as_mut() {
            self.embed_token_batches(std::slice::from_ref(&token_ids), Some(&mut **cp))?
        } else {
            self.embed_token_batches(std::slice::from_ref(&token_ids), None)?
        };
        out.copy_from_slice(&outputs[0]);
        Ok(())
    }

    pub fn embed_batch(
        &self,
        texts: &[String],
        options: BatchOptions,
        mut checkpoint: Option<Checkpoint<'_>>,
    ) -> Result<Vec<Vec<f32>>, GteError> {
        if let Some(max_batch) = options.max_batch {
            if texts.len() > max_batch {
                return Err(GteError::BatchTooLarge(texts.len()));
            }
        }
        if texts.is_empty() {
            return Ok(Vec::new());
        }

        let vocab_map = self.vocab_map();
        let mut token_batches = Vec::with_capacity(texts.len());
        for (index, text) in texts.iter().enumerate() {
            if let Some(max) = options.max_chars_per_input {
                if text.len() > max {
                    return Err(GteError::InputTooLarge {
                        index,
                        len: text.len(),
                        max,
                    });
                }
            }
            if let Some(cp) = checkpoint.as_mut() {
                cp("batch_item_start")?;
            }
            token_batches.push(tokenize_with(text, &vocab_map, self.config.max_seq_len));
            if let Some(cp) = checkpoint.as_mut() {
                cp("tokenized")?;
            }
        }

        let outputs = if let Some(cp) = checkpoint.as_mut() {
            self.embed_token_batches(&token_batches, Some(&mut **cp))?
        } else {
            self.embed_token_batches(&token_batches, None)?
        };

        if let Some(cp) = checkpoint.as_mut() {
            for _ in texts {
                cp("batch_item_done")?;
            }
        }

        Ok(outputs)
    }

    fn vocab_map(&self) -> std::collections::HashMap<&str, u32> {
        self.vocab
            .iter()
            .enumerate()
            .map(|(i, s)| (s.as_str(), i as u32))
            .collect()
    }

    fn embed_token_batches(
        &self,
        token_batches: &[Vec<u32>],
        checkpoint: Option<Checkpoint<'_>>,
    ) -> Result<Vec<Vec<f32>>, GteError> {
        if token_batches.is_empty() {
            return Ok(Vec::new());
        }

        let batch_size = token_batches.len();
        let seq_len = token_batches.iter().map(Vec::len).max().unwrap_or(0);
        let rows = batch_size
            .checked_mul(seq_len)
            .expect("internal batch rows do not overflow");
        let hidden = self.config.hidden_size;

        let mut token_ids = vec![TOKEN_PAD; rows];
        let mut attn_mask = vec![false; rows];
        for (batch_index, batch_token_ids) in token_batches.iter().enumerate() {
            let row_start = batch_index * seq_len;
            let row_end = row_start + batch_token_ids.len();
            token_ids[row_start..row_end].copy_from_slice(batch_token_ids);
            attn_mask[row_start..row_end].fill(true);
        }

        let hidden_states = if let Some(cp) = checkpoint {
            self.transformer_forward_batch(&token_ids, &attn_mask, batch_size, seq_len, Some(cp))?
        } else {
            self.transformer_forward_batch(&token_ids, &attn_mask, batch_size, seq_len, None)?
        };

        let mut outputs = vec![vec![0.0; hidden]; batch_size];
        let hidden_rows_per_item = seq_len * hidden;
        for (batch_index, output) in outputs.iter_mut().enumerate() {
            let hidden_start = batch_index * hidden_rows_per_item;
            let hidden_end = hidden_start + hidden_rows_per_item;
            let mask_start = batch_index * seq_len;
            let mask_end = mask_start + seq_len;
            mean_pooling(
                output,
                &hidden_states[hidden_start..hidden_end],
                &attn_mask[mask_start..mask_end],
                hidden,
            );
            l2_normalize(output);
        }
        Ok(outputs)
    }

    #[allow(clippy::too_many_lines)]
    fn transformer_forward_batch(
        &self,
        token_ids: &[u32],
        attn_mask: &[bool],
        batch_size: usize,
        seq_len: usize,
        mut checkpoint: Option<Checkpoint<'_>>,
    ) -> Result<Vec<f32>, GteError> {
        let rows = batch_size
            .checked_mul(seq_len)
            .expect("internal batch rows do not overflow");
        debug_assert_eq!(token_ids.len(), rows);
        debug_assert_eq!(attn_mask.len(), rows);

        let hidden = self.config.hidden_size;
        let head_dim = hidden / self.config.num_heads;
        let scale = 1.0 / (head_dim as f32).sqrt();
        let mut hidden_states = vec![0.0; rows * hidden];
        for batch_index in 0..batch_size {
            let row_base = batch_index * seq_len;
            for position in 0..seq_len {
                let row = row_base + position;
                if !attn_mask[row] {
                    continue;
                }
                let token_id = token_ids[row] as usize;
                let hidden_base = row * hidden;
                let emb_offset = token_id * hidden;
                let pos_offset = position * hidden;
                for d in 0..hidden {
                    hidden_states[hidden_base + d] = self.token_embeddings[emb_offset + d]
                        + self.position_embeddings[pos_offset + d]
                        + self.token_type_embeddings[d];
                }
            }
        }
        layer_norm(
            &mut hidden_states,
            &self.embed_ln_weight,
            &self.embed_ln_bias,
            hidden,
        );
        if let Some(cp) = checkpoint.as_mut() {
            cp("embeddings_ready")?;
        }

        for layer in &self.layers {
            let q = linear(
                &hidden_states,
                &layer.query_weight,
                Some(&layer.query_bias),
                rows,
                hidden,
                hidden,
            );
            let k = linear(
                &hidden_states,
                &layer.key_weight,
                Some(&layer.key_bias),
                rows,
                hidden,
                hidden,
            );
            let v = linear(
                &hidden_states,
                &layer.value_weight,
                Some(&layer.value_bias),
                rows,
                hidden,
                hidden,
            );

            let mut attn_output = vec![0.0; rows * hidden];
            let mut scores = vec![0.0; seq_len];
            for batch_index in 0..batch_size {
                let batch_row_base = batch_index * seq_len;
                for head in 0..self.config.num_heads {
                    let head_offset = head * head_dim;
                    for query_position in 0..seq_len {
                        let query_row = batch_row_base + query_position;
                        if !attn_mask[query_row] {
                            continue;
                        }
                        let query_base = query_row * hidden + head_offset;
                        for (key_position, slot) in scores.iter_mut().enumerate() {
                            let key_row = batch_row_base + key_position;
                            if !attn_mask[key_row] {
                                *slot = -10_000.0;
                                continue;
                            }
                            let key_base = key_row * hidden + head_offset;
                            let mut score = 0.0;
                            for d in 0..head_dim {
                                score += q[query_base + d] * k[key_base + d];
                            }
                            *slot = score * scale;
                        }
                        softmax(&mut scores);
                        for d in 0..head_dim {
                            let value_offset = head_offset + d;
                            let mut sum = 0.0;
                            for (key_position, weight) in scores.iter().copied().enumerate() {
                                let value_row = batch_row_base + key_position;
                                sum += weight * v[value_row * hidden + value_offset];
                            }
                            attn_output[query_row * hidden + value_offset] = sum;
                        }
                    }
                }
            }

            let attn_projected = linear(
                &attn_output,
                &layer.attn_output_weight,
                Some(&layer.attn_output_bias),
                rows,
                hidden,
                hidden,
            );
            let mut after_attn = add_residual(&attn_projected, &hidden_states);
            layer_norm(
                &mut after_attn,
                &layer.attn_ln_weight,
                &layer.attn_ln_bias,
                hidden,
            );
            let mut ffn_hidden = linear(
                &after_attn,
                &layer.ffn_inter_weight,
                Some(&layer.ffn_inter_bias),
                rows,
                hidden,
                self.config.intermediate,
            );
            gelu(&mut ffn_hidden);
            let ffn_output = linear(
                &ffn_hidden,
                &layer.ffn_output_weight,
                Some(&layer.ffn_output_bias),
                rows,
                self.config.intermediate,
                hidden,
            );
            hidden_states = add_residual(&ffn_output, &after_attn);
            layer_norm(
                &mut hidden_states,
                &layer.ffn_ln_weight,
                &layer.ffn_ln_bias,
                hidden,
            );
            if let Some(cp) = checkpoint.as_mut() {
                cp("layer_done")?;
            }
        }
        Ok(hidden_states)
    }
}

fn read_weights(
    cursor: &mut Cursor<&[u8]>,
    len: usize,
    mmap: Option<&Arc<Mmap>>,
) -> Result<Weights, GteError> {
    let byte_len = len
        .checked_mul(std::mem::size_of::<f32>())
        .ok_or_else(|| GteError::InvalidModel("weight blob len overflow".to_string()))?;
    let start = cursor_position(cursor)?;
    let bytes = read_exact_slice(cursor, byte_len)?;
    if let Some(mmap) = mmap {
        Weights::from_mmap_or_copy(Arc::clone(mmap), start..start + byte_len, bytes)
    } else {
        Weights::from_le_bytes(bytes)
    }
}

fn read_u32(cursor: &mut Cursor<&[u8]>) -> Result<u32, GteError> {
    let buf = read_exact_slice(cursor, std::mem::size_of::<u32>())?;
    Ok(u32::from_le_bytes(buf.try_into().expect("4-byte chunk")))
}

fn read_u16(cursor: &mut Cursor<&[u8]>) -> Result<u16, GteError> {
    let buf = read_exact_slice(cursor, std::mem::size_of::<u16>())?;
    Ok(u16::from_le_bytes(buf.try_into().expect("2-byte chunk")))
}

fn read_exact_slice<'a>(cursor: &mut Cursor<&'a [u8]>, len: usize) -> Result<&'a [u8], GteError> {
    let start = cursor_position(cursor)?;
    let end = start
        .checked_add(len)
        .ok_or_else(|| GteError::InvalidModel("cursor position overflow".to_string()))?;
    if end > cursor.get_ref().len() {
        return Err(std::io::Error::from(std::io::ErrorKind::UnexpectedEof).into());
    }
    cursor.set_position(end as u64);
    Ok(&cursor.get_ref()[start..end])
}

fn cursor_position(cursor: &Cursor<&[u8]>) -> Result<usize, GteError> {
    usize::try_from(cursor.position())
        .map_err(|_| GteError::InvalidModel("cursor position overflow".to_string()))
}

fn validate_weight_range(mmap_len: usize, range: &Range<usize>) -> Result<(), GteError> {
    let Some(byte_len) = range.end.checked_sub(range.start) else {
        return Err(GteError::InvalidModel(
            "invalid mapped weight range".to_string(),
        ));
    };
    if !byte_len.is_multiple_of(std::mem::size_of::<f32>()) {
        return Err(GteError::InvalidModel(
            "mapped weight range len must be a multiple of 4 bytes".to_string(),
        ));
    }
    if range.end > mmap_len {
        return Err(GteError::InvalidModel(
            "mapped weight range exceeds file size".to_string(),
        ));
    }
    Ok(())
}

fn mapped_range_is_aligned(mmap: &Mmap, range: &Range<usize>) -> bool {
    let ptr = mmap.as_ptr().wrapping_add(range.start);
    (ptr as usize).is_multiple_of(std::mem::align_of::<f32>())
}

fn is_punctuation(b: u8) -> bool {
    (33..=47).contains(&b)
        || (58..=64).contains(&b)
        || (91..=96).contains(&b)
        || (123..=126).contains(&b)
}

fn is_whitespace(b: u8) -> bool {
    matches!(b, b' ' | b'\t' | b'\n' | b'\r')
}

fn basic_tokenize(text: &str) -> Vec<String> {
    let bytes = text.as_bytes();
    let mut tokens = Vec::new();
    let mut i = 0;
    while i < bytes.len() {
        while i < bytes.len() && is_whitespace(bytes[i]) {
            i += 1;
        }
        if i >= bytes.len() {
            break;
        }
        let start = i;
        if is_punctuation(bytes[i]) {
            i += 1;
        } else {
            while i < bytes.len() && !is_whitespace(bytes[i]) && !is_punctuation(bytes[i]) {
                i += 1;
            }
        }
        let src = &text[start..i];
        if src.bytes().any(|c| c.is_ascii_uppercase()) {
            tokens.push(src.to_ascii_lowercase());
        } else {
            tokens.push(src.to_string());
        }
    }
    tokens
}

fn wordpiece_tokenize(
    word: &str,
    vocab: &std::collections::HashMap<&str, u32>,
    out: &mut Vec<u32>,
) {
    if word.is_empty() {
        return;
    }
    let mut start = 0;
    while start < word.len() {
        let mut end = word.len();
        let mut found = None;
        while start < end {
            let candidate = if start > 0 {
                format!("##{}", &word[start..end])
            } else {
                word[start..end].to_string()
            };
            if let Some(id) = vocab.get(candidate.as_str()).copied() {
                found = Some((id, end));
                break;
            }
            end -= 1;
        }
        if let Some((id, end)) = found {
            out.push(id);
            start = end;
        } else {
            out.push(TOKEN_UNK);
            start += 1;
        }
    }
}

fn tokenize_with(
    text: &str,
    vocab: &std::collections::HashMap<&str, u32>,
    max_seq_len: usize,
) -> Vec<u32> {
    let basic = basic_tokenize(text);
    let mut tokens = Vec::with_capacity(max_seq_len);
    tokens.push(TOKEN_CLS);
    for token in basic {
        if tokens.len() >= max_seq_len - 1 {
            break;
        }
        let previous = tokens.len();
        wordpiece_tokenize(&token, vocab, &mut tokens);
        if tokens.len() > max_seq_len - 1 {
            tokens.truncate(previous);
            break;
        }
    }
    if tokens.len() < max_seq_len {
        tokens.push(TOKEN_SEP);
    }
    tokens
}

fn linear(
    x: &[f32],
    w: &[f32],
    b: Option<&[f32]>,
    rows: usize,
    in_dim: usize,
    out_dim: usize,
) -> Vec<f32> {
    memento_vector::linear_out_in(x, rows, in_dim, w, out_dim, b)
        .expect("internal GTE projection dimensions match")
}

fn layer_norm(x: &mut [f32], gamma: &[f32], beta: &[f32], hidden: usize) {
    for row in x.chunks_exact_mut(hidden) {
        let mean = row.iter().sum::<f32>() / hidden as f32;
        let variance = row
            .iter()
            .map(|v| {
                let diff = *v - mean;
                diff * diff
            })
            .sum::<f32>()
            / hidden as f32;
        let std_inv = 1.0 / (variance + LAYER_NORM_EPS).sqrt();
        for i in 0..hidden {
            row[i] = gamma[i] * (row[i] - mean) * std_inv + beta[i];
        }
    }
}

fn gelu(x: &mut [f32]) {
    const C: f32 = 0.797_884_6;
    for value in x {
        let v = *value;
        *value = 0.5 * v * (1.0 + (C * (v + 0.044_715 * v * v * v)).tanh());
    }
}

fn softmax(x: &mut [f32]) {
    let max = x.iter().copied().fold(f32::NEG_INFINITY, f32::max);
    let mut sum = 0.0;
    for value in x.iter_mut() {
        *value = (*value - max).exp();
        sum += *value;
    }
    for value in x.iter_mut() {
        *value /= sum;
    }
}

fn add_residual(x: &[f32], residual: &[f32]) -> Vec<f32> {
    x.iter().zip(residual).map(|(a, b)| a + b).collect()
}

fn mean_pooling(out: &mut [f32], hidden_states: &[f32], attn_mask: &[bool], hidden: usize) {
    out.fill(0.0);
    let mut count = 0_usize;
    for (s, mask) in attn_mask.iter().copied().enumerate() {
        if mask {
            let base = s * hidden;
            for d in 0..hidden {
                out[d] += hidden_states[base + d];
            }
            count += 1;
        }
    }
    if count > 0 {
        let inv = 1.0 / count as f32;
        for value in out {
            *value *= inv;
        }
    }
}

fn l2_normalize(x: &mut [f32]) {
    let norm = x.iter().map(|v| v * v).sum::<f32>().sqrt();
    if norm > 0.0 {
        for value in x {
            *value /= norm;
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::env;
    use std::fs;
    use std::path::PathBuf;
    use std::time::{SystemTime, UNIX_EPOCH};

    #[test]
    #[allow(clippy::too_many_lines)]
    fn tokenizer_matches_reference_behaviour() {
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
            "##s",
            ",",
            "world",
            "!",
        ]
        .into_iter()
        .map(str::to_string)
        .collect::<Vec<_>>();
        let model = Model {
            config: ModelConfig {
                vocab_size: vocab.len(),
                hidden_size: 4,
                num_layers: 0,
                num_heads: 1,
                intermediate: 4,
                max_seq_len: 8,
            },
            vocab,
            token_embeddings: vec![0.0; 109 * 4].into(),
            position_embeddings: vec![0.0; 8 * 4].into(),
            token_type_embeddings: vec![0.0; 8].into(),
            embed_ln_weight: vec![1.0; 4].into(),
            embed_ln_bias: vec![0.0; 4].into(),
            layers: vec![],
            pooler_weight: vec![0.0; 16].into(),
            pooler_bias: vec![0.0; 4].into(),
        };
        assert_eq!(
            model.tokenize("Hello, worlds!"),
            vec![TOKEN_CLS, 104, 106, 107, 105, 108, TOKEN_SEP]
        );
        assert_eq!(TOKEN_PAD, 0);
        assert_eq!(TOKEN_MASK, 103);
    }

    #[allow(clippy::too_many_lines)]
    fn synthetic_model_bytes_with_last_token(
        header_overrides: &[(usize, u32)],
        last_token: &str,
    ) -> Vec<u8> {
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
            last_token,
        ];
        let hidden_size = 4_u32;
        let max_seq_len = 8_u32;
        let mut header = [
            vocab.len() as u32,
            hidden_size,
            0,
            1,
            hidden_size,
            max_seq_len,
        ];
        for (index, value) in header_overrides {
            header[*index] = *value;
        }
        let mut bytes = Vec::new();
        bytes.extend_from_slice(b"GTE1");
        for value in header {
            bytes.extend_from_slice(&value.to_le_bytes());
        }
        for token in &vocab {
            let raw = token.as_bytes();
            bytes.extend_from_slice(&(raw.len() as u16).to_le_bytes());
            bytes.extend_from_slice(raw);
        }
        for token_id in 0..vocab.len() {
            let base = (token_id as f32) + 1.0;
            for value in [base, 0.0, 0.0, 0.0] {
                bytes.extend_from_slice(&value.to_le_bytes());
            }
        }
        bytes.extend_from_slice(&vec![
            0_u8;
            (max_seq_len as usize) * (hidden_size as usize) * 4
        ]);
        bytes.extend_from_slice(&vec![0_u8; 2 * (hidden_size as usize) * 4]);
        bytes.extend_from_slice(&vec![0_u8; hidden_size as usize * 4]);
        bytes.extend_from_slice(&vec![0_u8; hidden_size as usize * 4]);
        bytes.extend_from_slice(&vec![0_u8; hidden_size as usize * hidden_size as usize * 4]);
        bytes.extend_from_slice(&vec![0_u8; hidden_size as usize * 4]);
        bytes
    }

    fn synthetic_model_bytes(header_overrides: &[(usize, u32)]) -> Vec<u8> {
        synthetic_model_bytes_with_last_token(header_overrides, "!")
    }

    fn synthetic_model_bytes_aligned(header_overrides: &[(usize, u32)]) -> Vec<u8> {
        synthetic_model_bytes_with_last_token(header_overrides, "!!")
    }

    fn unique_path(name: &str) -> PathBuf {
        let nanos = SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .expect("time")
            .as_nanos();
        env::temp_dir().join(format!("{name}-{nanos}.gtemodel"))
    }

    #[test]
    fn from_path_matches_from_bytes_for_unaligned_weights() {
        let bytes = synthetic_model_bytes(&[]);
        let path = unique_path("memento-gte-unaligned");
        fs::write(&path, &bytes).expect("write model");
        let from_path = Model::from_path(&path).expect("load from path");
        let from_bytes = Model::from_bytes(&bytes).expect("load from bytes");
        fs::remove_file(&path).ok();

        assert_eq!(from_path.vocab, from_bytes.vocab);
        assert_eq!(
            from_path.token_embeddings.as_ref(),
            from_bytes.token_embeddings.as_ref()
        );
        assert_eq!(
            from_path.pooler_weight.as_ref(),
            from_bytes.pooler_weight.as_ref()
        );
        assert_eq!(
            from_path.tokenize("Hello, world!"),
            from_bytes.tokenize("Hello, world!")
        );
        assert_eq!(
            from_path.embed("hello world").expect("embed"),
            from_bytes.embed("hello world").expect("embed")
        );
        assert!(!from_path.token_embeddings.is_mmap_backed());
        assert!(!from_path.pooler_weight.is_mmap_backed());
    }

    #[test]
    fn from_path_uses_mmap_for_aligned_weights() {
        let bytes = synthetic_model_bytes_aligned(&[]);
        let path = unique_path("memento-gte-aligned");
        fs::write(&path, &bytes).expect("write model");
        let from_path = Model::from_path(&path).expect("load from path");
        let from_bytes = Model::from_bytes(&bytes).expect("load from bytes");
        fs::remove_file(&path).ok();

        assert_eq!(from_path.vocab, from_bytes.vocab);
        assert_eq!(
            from_path.token_embeddings.as_ref(),
            from_bytes.token_embeddings.as_ref()
        );
        assert_eq!(
            from_path.embed("hello world").expect("embed"),
            from_bytes.embed("hello world").expect("embed")
        );
        if cfg!(target_endian = "little") {
            assert!(from_path.token_embeddings.is_mmap_backed());
            assert!(from_path.pooler_weight.is_mmap_backed());
        } else {
            assert!(!from_path.token_embeddings.is_mmap_backed());
            assert!(!from_path.pooler_weight.is_mmap_backed());
        }
    }

    #[test]
    fn rejects_models_with_small_max_seq_len() {
        let err = Model::from_bytes(&synthetic_model_bytes(&[(5, 1)])).expect_err("invalid model");
        assert!(
            matches!(err, GteError::InvalidModel(message) if message.contains("max_seq_len must be at least 2"))
        );
    }

    #[test]
    fn rejects_models_with_reserved_vocab_missing() {
        let err = Model::from_bytes(&synthetic_model_bytes(&[(0, TOKEN_MASK)]))
            .expect_err("invalid model");
        assert!(
            matches!(err, GteError::InvalidModel(message) if message.contains("vocab_size must include reserved token id"))
        );
    }

    #[test]
    fn rejects_models_with_zero_heads() {
        let err = Model::from_bytes(&synthetic_model_bytes(&[(3, 0)])).expect_err("invalid model");
        assert!(
            matches!(err, GteError::InvalidModel(message) if message.contains("num_heads must be positive and divide hidden_size"))
        );
    }
}
