//! Rust FP32 `GTE1` loader and scalar inference path.
//! Faithful to the MIT-licensed `/tmp/go-gte` reference implementation.

use serde::{Deserialize, Serialize};
use std::fs;
use std::io::{Cursor, Read};
use std::path::Path;
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

#[derive(Debug, Clone)]
pub struct LayerWeights {
    pub query_weight: Vec<f32>,
    pub query_bias: Vec<f32>,
    pub key_weight: Vec<f32>,
    pub key_bias: Vec<f32>,
    pub value_weight: Vec<f32>,
    pub value_bias: Vec<f32>,
    pub attn_output_weight: Vec<f32>,
    pub attn_output_bias: Vec<f32>,
    pub attn_ln_weight: Vec<f32>,
    pub attn_ln_bias: Vec<f32>,
    pub ffn_inter_weight: Vec<f32>,
    pub ffn_inter_bias: Vec<f32>,
    pub ffn_output_weight: Vec<f32>,
    pub ffn_output_bias: Vec<f32>,
    pub ffn_ln_weight: Vec<f32>,
    pub ffn_ln_bias: Vec<f32>,
}

#[derive(Debug, Clone)]
pub struct Model {
    pub config: ModelConfig,
    pub vocab: Vec<String>,
    token_embeddings: Vec<f32>,
    position_embeddings: Vec<f32>,
    token_type_embeddings: Vec<f32>,
    embed_ln_weight: Vec<f32>,
    embed_ln_bias: Vec<f32>,
    pub layers: Vec<LayerWeights>,
    pub pooler_weight: Vec<f32>,
    pub pooler_bias: Vec<f32>,
}

#[derive(Debug, Clone, Copy, Default)]
pub struct BatchOptions {
    pub max_batch: Option<usize>,
    pub max_chars_per_input: Option<usize>,
}

pub type Checkpoint<'a> = &'a mut dyn FnMut(&'static str) -> Result<(), GteError>;

impl Model {
    pub fn from_path(path: impl AsRef<Path>) -> Result<Self, GteError> {
        Self::from_bytes(&fs::read(path)?)
    }

    pub fn from_bytes(bytes: &[u8]) -> Result<Self, GteError> {
        let mut cursor = Cursor::new(bytes);
        let mut magic = [0_u8; 4];
        cursor.read_exact(&mut magic)?;
        if &magic != FILE_MAGIC {
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
            let mut buf = vec![0_u8; len];
            cursor.read_exact(&mut buf)?;
            vocab.push(String::from_utf8(buf).map_err(|e| GteError::InvalidModel(e.to_string()))?);
        }

        let read_vec = |cursor: &mut Cursor<&[u8]>, len: usize| -> Result<Vec<f32>, GteError> {
            let mut bytes = vec![0_u8; len * 4];
            cursor.read_exact(&mut bytes)?;
            Ok(bytes
                .chunks_exact(4)
                .map(|chunk| f32::from_le_bytes(chunk.try_into().expect("4-byte chunk")))
                .collect())
        };

        let token_embeddings = read_vec(&mut cursor, config.vocab_size * config.hidden_size)?;
        let position_embeddings = read_vec(&mut cursor, config.max_seq_len * config.hidden_size)?;
        let token_type_embeddings = read_vec(&mut cursor, 2 * config.hidden_size)?;
        let embed_ln_weight = read_vec(&mut cursor, config.hidden_size)?;
        let embed_ln_bias = read_vec(&mut cursor, config.hidden_size)?;

        let mut layers = Vec::with_capacity(config.num_layers);
        for _ in 0..config.num_layers {
            layers.push(LayerWeights {
                query_weight: read_vec(&mut cursor, config.hidden_size * config.hidden_size)?,
                query_bias: read_vec(&mut cursor, config.hidden_size)?,
                key_weight: read_vec(&mut cursor, config.hidden_size * config.hidden_size)?,
                key_bias: read_vec(&mut cursor, config.hidden_size)?,
                value_weight: read_vec(&mut cursor, config.hidden_size * config.hidden_size)?,
                value_bias: read_vec(&mut cursor, config.hidden_size)?,
                attn_output_weight: read_vec(&mut cursor, config.hidden_size * config.hidden_size)?,
                attn_output_bias: read_vec(&mut cursor, config.hidden_size)?,
                attn_ln_weight: read_vec(&mut cursor, config.hidden_size)?,
                attn_ln_bias: read_vec(&mut cursor, config.hidden_size)?,
                ffn_inter_weight: read_vec(&mut cursor, config.intermediate * config.hidden_size)?,
                ffn_inter_bias: read_vec(&mut cursor, config.intermediate)?,
                ffn_output_weight: read_vec(&mut cursor, config.hidden_size * config.intermediate)?,
                ffn_output_bias: read_vec(&mut cursor, config.hidden_size)?,
                ffn_ln_weight: read_vec(&mut cursor, config.hidden_size)?,
                ffn_ln_bias: read_vec(&mut cursor, config.hidden_size)?,
            });
        }
        let pooler_weight = read_vec(&mut cursor, config.hidden_size * config.hidden_size)?;
        let pooler_bias = read_vec(&mut cursor, config.hidden_size)?;

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
        let attn_mask = vec![true; token_ids.len()];
        let hidden = if let Some(cp) = checkpoint.as_mut() {
            self.transformer_forward(&token_ids, &attn_mask, Some(cp))?
        } else {
            self.transformer_forward(&token_ids, &attn_mask, None)?
        };
        mean_pooling(out, &hidden, &attn_mask, self.config.hidden_size);
        l2_normalize(out);
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
        let mut result = Vec::with_capacity(texts.len());
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
            let mut output = vec![0.0; self.dim()];
            if let Some(cp) = checkpoint.as_mut() {
                self.embed_to(text, &mut output, Some(cp))?;
            } else {
                self.embed_to(text, &mut output, None)?;
            }
            result.push(output);
            if let Some(cp) = checkpoint.as_mut() {
                cp("batch_item_done")?;
            }
        }
        Ok(result)
    }

    fn vocab_map(&self) -> std::collections::HashMap<&str, u32> {
        self.vocab
            .iter()
            .enumerate()
            .map(|(i, s)| (s.as_str(), i as u32))
            .collect()
    }

    #[allow(clippy::too_many_lines)]
    fn transformer_forward(
        &self,
        token_ids: &[u32],
        attn_mask: &[bool],
        mut checkpoint: Option<Checkpoint<'_>>,
    ) -> Result<Vec<f32>, GteError> {
        let seq_len = token_ids.len();
        let hidden = self.config.hidden_size;
        let head_dim = hidden / self.config.num_heads;
        let mut hidden_states = vec![0.0; seq_len * hidden];
        for (s, token_id) in token_ids.iter().copied().enumerate().take(seq_len) {
            let token_id = token_id as usize;
            let base = s * hidden;
            let emb_offset = token_id * hidden;
            let pos_offset = s * hidden;
            for d in 0..hidden {
                hidden_states[base + d] = self.token_embeddings[emb_offset + d]
                    + self.position_embeddings[pos_offset + d]
                    + self.token_type_embeddings[d];
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
                seq_len,
                hidden,
                hidden,
            );
            let k = linear(
                &hidden_states,
                &layer.key_weight,
                Some(&layer.key_bias),
                seq_len,
                hidden,
                hidden,
            );
            let v = linear(
                &hidden_states,
                &layer.value_weight,
                Some(&layer.value_bias),
                seq_len,
                hidden,
                hidden,
            );
            let mut attn_output = vec![0.0; seq_len * hidden];
            let scale = 1.0 / (head_dim as f32).sqrt();
            for h in 0..self.config.num_heads {
                for i in 0..seq_len {
                    let mut scores = vec![0.0; seq_len];
                    for j in 0..seq_len {
                        let mut score = 0.0;
                        for d in 0..head_dim {
                            let offset = h * head_dim + d;
                            score += q[i * hidden + offset] * k[j * hidden + offset];
                        }
                        scores[j] = if attn_mask[j] {
                            score * scale
                        } else {
                            -10_000.0
                        };
                    }
                    softmax(&mut scores);
                    for d in 0..head_dim {
                        let offset = h * head_dim + d;
                        let mut sum = 0.0;
                        for j in 0..seq_len {
                            sum += scores[j] * v[j * hidden + offset];
                        }
                        attn_output[i * hidden + offset] = sum;
                    }
                }
            }
            let attn_projected = linear(
                &attn_output,
                &layer.attn_output_weight,
                Some(&layer.attn_output_bias),
                seq_len,
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
                seq_len,
                hidden,
                self.config.intermediate,
            );
            gelu(&mut ffn_hidden);
            let ffn_output = linear(
                &ffn_hidden,
                &layer.ffn_output_weight,
                Some(&layer.ffn_output_bias),
                seq_len,
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

fn read_u32(cursor: &mut Cursor<&[u8]>) -> Result<u32, GteError> {
    let mut buf = [0_u8; 4];
    cursor.read_exact(&mut buf)?;
    Ok(u32::from_le_bytes(buf))
}

fn read_u16(cursor: &mut Cursor<&[u8]>) -> Result<u16, GteError> {
    let mut buf = [0_u8; 2];
    cursor.read_exact(&mut buf)?;
    Ok(u16::from_le_bytes(buf))
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
    seq_len: usize,
    in_dim: usize,
    out_dim: usize,
) -> Vec<f32> {
    let mut y = vec![0.0; seq_len * out_dim];
    for s in 0..seq_len {
        for o in 0..out_dim {
            let mut sum = 0.0;
            for i in 0..in_dim {
                sum += x[s * in_dim + i] * w[o * in_dim + i];
            }
            y[s * out_dim + o] = sum + b.map_or(0.0, |bias| bias[o]);
        }
    }
    y
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
            token_embeddings: vec![0.0; 109 * 4],
            position_embeddings: vec![0.0; 8 * 4],
            token_type_embeddings: vec![0.0; 8],
            embed_ln_weight: vec![1.0; 4],
            embed_ln_bias: vec![0.0; 4],
            layers: vec![],
            pooler_weight: vec![0.0; 16],
            pooler_bias: vec![0.0; 4],
        };
        assert_eq!(
            model.tokenize("Hello, worlds!"),
            vec![TOKEN_CLS, 104, 106, 107, 105, 108, TOKEN_SEP]
        );
        assert_eq!(TOKEN_PAD, 0);
        assert_eq!(TOKEN_MASK, 103);
    }

    #[allow(clippy::too_many_lines)]
    fn synthetic_model_bytes(header_overrides: &[(usize, u32)]) -> Vec<u8> {
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
