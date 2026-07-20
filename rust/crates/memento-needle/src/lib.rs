//! Rust `NDL1` parser plus pure-Rust scalar Needle router inference.

use sentencepiece_rust::{PieceType, SentencePieceProcessor};
use serde::{Deserialize, Serialize};
use sha2::{Digest, Sha256};
use std::collections::{BTreeMap, HashMap};
use std::fs;
use std::path::{Path, PathBuf};
use thiserror::Error;

const FILE_MAGIC: &[u8; 4] = b"NDL1";
const FILE_VERSION: u16 = 1;
const SECTION_CONFIG: &[u8; 4] = b"CONF";
const SECTION_TOKENIZER: &[u8; 4] = b"TOKN";
const SECTION_METADATA: &[u8; 4] = b"META";
const SECTION_TENSOR_DIRECTORY: &[u8; 4] = b"TDIR";
const SECTION_TENSOR_DATA: &[u8; 4] = b"DATA";
const DTYPE_BF16: u8 = 1;
const HEADER_PREFIX_LEN: usize = 12;
const SECTION_DESCRIPTOR_LEN: usize = 52;
const EPSILON: f32 = 1e-6;
const TOKEN_PAD: u32 = 0;
const TOKEN_EOS: u32 = 1;
const TOKEN_BOS: u32 = 2;
const TOKEN_UNK: u32 = 3;
const TOKEN_TOOL_CALL: u32 = 4;
const TOKEN_TOOLS: u32 = 5;

#[derive(Debug, Error)]
pub enum NeedleError {
    #[error("io error: {0}")]
    Io(#[from] std::io::Error),
    #[error("invalid model magic")]
    InvalidMagic,
    #[error("unsupported model version: {0}")]
    UnsupportedVersion(u16),
    #[error("invalid model: {0}")]
    InvalidModel(String),
    #[error("missing section: {0}")]
    MissingSection(&'static str),
    #[error("section {section} is out of bounds: offset {offset}, len {len}, file len {file_len}")]
    SectionOutOfBounds {
        section: String,
        offset: usize,
        len: usize,
        file_len: usize,
    },
    #[error("checksum mismatch for section {section}")]
    ChecksumMismatch { section: String },
    #[error("missing tensor: {0}")]
    MissingTensor(String),
    #[error("invalid tensor shape for {name}: expected {expected:?}, got {got:?}")]
    InvalidTensorShape {
        name: String,
        expected: Vec<u32>,
        got: Vec<u32>,
    },
    #[error("sentencepiece error: {0}")]
    SentencePiece(String),
    #[error("cancelled at checkpoint: {0}")]
    Cancelled(&'static str),
    #[error("generation exceeded max length {0}")]
    GenerationTooLong(usize),
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct NeedleConfig {
    pub activation: String,
    pub contrastive_dim: u32,
    pub d_ff: u32,
    pub d_model: u32,
    pub dropout_rate: f32,
    pub dtype: String,
    pub max_seq_len: u32,
    pub no_feedforward: bool,
    pub num_decoder_layers: u32,
    pub num_encoder_layers: u32,
    pub num_heads: u32,
    pub num_kv_heads: u32,
    pub num_memory_slots: u32,
    pub pad_token_id: u32,
    pub rope_theta: f32,
    pub vocab_size: u32,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub struct SourceHash {
    pub name: String,
    pub sha256: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub struct SourceMetadata {
    pub checkpoint: SourceHash,
    pub tokenizer_model: SourceHash,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub struct SectionHashes {
    pub config_sha256: String,
    pub tensor_data_sha256: String,
    pub tensor_directory_sha256: String,
    pub tokenizer_sha256: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub struct ConverterMetadata {
    pub format: String,
    pub tool: String,
    pub version: u16,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub struct NeedleMetadata {
    pub converter: ConverterMetadata,
    pub section_hashes: SectionHashes,
    pub source: SourceMetadata,
    pub tensor_count: u32,
    pub tokenizer_piece_count: u32,
}

#[derive(Debug, Clone, PartialEq)]
pub struct TokenizerPiece {
    pub piece: String,
    pub piece_type: u8,
    pub score: f32,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub enum TensorDType {
    Bf16,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct TensorDescriptor {
    pub name: String,
    pub dtype: TensorDType,
    pub shape: Vec<u32>,
    pub data_offset: usize,
    pub byte_len: usize,
    pub sha256: [u8; 32],
}

#[derive(Debug, Clone)]
pub struct TensorView<'a> {
    descriptor: &'a TensorDescriptor,
    data: &'a [u8],
}

impl<'a> TensorView<'a> {
    #[must_use]
    pub fn name(&self) -> &str {
        &self.descriptor.name
    }
    #[must_use]
    pub fn dtype(&self) -> &TensorDType {
        &self.descriptor.dtype
    }
    #[must_use]
    pub fn shape(&self) -> &[u32] {
        &self.descriptor.shape
    }
    #[must_use]
    pub fn raw_bf16(&self) -> &'a [u8] {
        self.data
    }
    #[must_use]
    pub fn element_count(&self) -> usize {
        shape_elements(&self.descriptor.shape)
    }
    #[must_use]
    pub fn to_f32_vec(&self) -> Vec<f32> {
        self.data
            .chunks_exact(2)
            .map(|chunk| {
                let word = u16::from_le_bytes(chunk.try_into().expect("2-byte chunk"));
                f32::from_bits(u32::from(word) << 16)
            })
            .collect()
    }
}

#[derive(Debug, Clone)]
pub struct Model {
    bytes: Vec<u8>,
    config: NeedleConfig,
    metadata: NeedleMetadata,
    tokenizer: Vec<TokenizerPiece>,
    tensors: BTreeMap<String, TensorDescriptor>,
    tensor_data_range: std::ops::Range<usize>,
}

impl Model {
    pub fn from_path(path: impl AsRef<Path>) -> Result<Self, NeedleError> {
        Self::from_bytes(&fs::read(path)?)
    }

    #[allow(clippy::too_many_lines)]
    pub fn from_bytes(bytes: &[u8]) -> Result<Self, NeedleError> {
        if bytes.len() < HEADER_PREFIX_LEN || &bytes[..4] != FILE_MAGIC {
            return Err(NeedleError::InvalidMagic);
        }
        let version = read_u16(bytes, 4)?;
        if version != FILE_VERSION {
            return Err(NeedleError::UnsupportedVersion(version));
        }
        let section_count = usize::from(read_u16(bytes, 6)?);
        let header_len = HEADER_PREFIX_LEN
            .checked_add(
                section_count
                    .checked_mul(SECTION_DESCRIPTOR_LEN)
                    .ok_or_else(|| {
                        NeedleError::InvalidModel("section descriptor table overflows".to_string())
                    })?,
            )
            .ok_or_else(|| NeedleError::InvalidModel("header length overflows".to_string()))?;
        if header_len > bytes.len() {
            return Err(NeedleError::InvalidModel(
                "section descriptor table extends past file".to_string(),
            ));
        }
        let mut sections = BTreeMap::new();
        for index in 0..section_count {
            let base = HEADER_PREFIX_LEN + index * SECTION_DESCRIPTOR_LEN;
            let kind = read_array_4(bytes, base)?;
            let offset = read_u64(bytes, base + 4)? as usize;
            let len = read_u64(bytes, base + 12)? as usize;
            let checksum = read_array_32(bytes, base + 20)?;
            let end = offset.checked_add(len).ok_or_else(|| {
                NeedleError::InvalidModel(format!(
                    "section {} length overflows",
                    String::from_utf8_lossy(&kind)
                ))
            })?;
            if end > bytes.len() {
                return Err(NeedleError::SectionOutOfBounds {
                    section: String::from_utf8_lossy(&kind).into_owned(),
                    offset,
                    len,
                    file_len: bytes.len(),
                });
            }
            let payload = &bytes[offset..end];
            if sha256_bytes(payload) != checksum {
                return Err(NeedleError::ChecksumMismatch {
                    section: String::from_utf8_lossy(&kind).into_owned(),
                });
            }
            sections.insert(kind, (offset, len));
        }
        let config_bytes = section_payload(bytes, &sections, *SECTION_CONFIG, "CONF")?;
        let tokenizer_bytes = section_payload(bytes, &sections, *SECTION_TOKENIZER, "TOKN")?;
        let metadata_bytes = section_payload(bytes, &sections, *SECTION_METADATA, "META")?;
        let tensor_dir_bytes =
            section_payload(bytes, &sections, *SECTION_TENSOR_DIRECTORY, "TDIR")?;
        let (tensor_data_range, tensor_data_bytes) =
            section_payload_with_range(bytes, &sections, *SECTION_TENSOR_DATA, "DATA")?;
        let config: NeedleConfig = serde_json::from_slice(config_bytes)
            .map_err(|exc| NeedleError::InvalidModel(format!("invalid config json: {exc}")))?;
        let metadata: NeedleMetadata = serde_json::from_slice(metadata_bytes)
            .map_err(|exc| NeedleError::InvalidModel(format!("invalid metadata json: {exc}")))?;
        let tokenizer = parse_tokenizer(tokenizer_bytes)?;
        let tensors = parse_tensor_directory(tensor_dir_bytes, tensor_data_bytes.len())?;
        if usize::try_from(metadata.tensor_count).ok() != Some(tensors.len()) {
            return Err(NeedleError::InvalidModel(format!(
                "metadata tensor_count {} != parsed {}",
                metadata.tensor_count,
                tensors.len()
            )));
        }
        if usize::try_from(metadata.tokenizer_piece_count).ok() != Some(tokenizer.len()) {
            return Err(NeedleError::InvalidModel(format!(
                "metadata tokenizer_piece_count {} != parsed {}",
                metadata.tokenizer_piece_count,
                tokenizer.len()
            )));
        }
        validate_hash(
            &metadata.section_hashes.config_sha256,
            config_bytes,
            "config",
        )?;
        validate_hash(
            &metadata.section_hashes.tokenizer_sha256,
            tokenizer_bytes,
            "tokenizer",
        )?;
        validate_hash(
            &metadata.section_hashes.tensor_directory_sha256,
            tensor_dir_bytes,
            "tensor_directory",
        )?;
        validate_hash(
            &metadata.section_hashes.tensor_data_sha256,
            tensor_data_bytes,
            "tensor_data",
        )?;
        Ok(Self {
            bytes: bytes.to_vec(),
            config,
            metadata,
            tokenizer,
            tensors,
            tensor_data_range,
        })
    }

    #[must_use]
    pub fn config(&self) -> &NeedleConfig {
        &self.config
    }
    #[must_use]
    pub fn metadata(&self) -> &NeedleMetadata {
        &self.metadata
    }
    #[must_use]
    pub fn tokenizer_pieces(&self) -> &[TokenizerPiece] {
        &self.tokenizer
    }
    #[must_use]
    pub fn tensor_names(&self) -> Vec<&str> {
        self.tensors.keys().map(String::as_str).collect()
    }
    #[must_use]
    pub fn tensor_descriptor(&self, name: &str) -> Option<&TensorDescriptor> {
        self.tensors.get(name)
    }
    #[must_use]
    pub fn tensor(&self, name: &str) -> Option<TensorView<'_>> {
        let descriptor = self.tensors.get(name)?;
        let start = self.tensor_data_range.start + descriptor.data_offset;
        let end = start + descriptor.byte_len;
        Some(TensorView {
            descriptor,
            data: &self.bytes[start..end],
        })
    }

    pub fn tensor_f32(&self, name: &str, expected_shape: &[u32]) -> Result<Vec<f32>, NeedleError> {
        let tensor = self
            .tensor(name)
            .ok_or_else(|| NeedleError::MissingTensor(name.to_string()))?;
        if tensor.shape() != expected_shape {
            return Err(NeedleError::InvalidTensorShape {
                name: name.to_string(),
                expected: expected_shape.to_vec(),
                got: tensor.shape().to_vec(),
            });
        }
        Ok(tensor.to_f32_vec())
    }
}

pub struct NeedleTokenizer {
    sp: SentencePieceProcessor,
}

impl NeedleTokenizer {
    pub fn from_model_path(path: impl AsRef<Path>) -> Result<Self, NeedleError> {
        let sp = SentencePieceProcessor::open(path)
            .map_err(|err| NeedleError::SentencePiece(err.to_string()))?;
        Ok(Self { sp })
    }

    pub fn from_model_bytes(bytes: &[u8]) -> Result<Self, NeedleError> {
        let sp = SentencePieceProcessor::from_bytes(bytes)
            .map_err(|err| NeedleError::SentencePiece(err.to_string()))?;
        Ok(Self { sp })
    }

    pub fn from_repo_default() -> Result<Self, NeedleError> {
        Self::from_model_path(
            PathBuf::from(env!("CARGO_MANIFEST_DIR")).join("../../../models/needle/needle.model"),
        )
    }

    #[must_use]
    pub fn pad_token_id(&self) -> u32 {
        TOKEN_PAD
    }
    #[must_use]
    pub fn eos_token_id(&self) -> u32 {
        TOKEN_EOS
    }
    #[must_use]
    pub fn bos_token_id(&self) -> u32 {
        TOKEN_BOS
    }
    #[must_use]
    pub fn unk_token_id(&self) -> u32 {
        TOKEN_UNK
    }
    #[must_use]
    pub fn tool_call_token_id(&self) -> u32 {
        TOKEN_TOOL_CALL
    }
    #[must_use]
    pub fn tools_token_id(&self) -> u32 {
        TOKEN_TOOLS
    }
    #[must_use]
    pub fn vocab_size(&self) -> usize {
        self.sp.piece_size()
    }
    pub fn encode(&self, text: &str) -> Result<Vec<u32>, NeedleError> {
        encode_with_special_tokens(&self.sp, text)
    }
    pub fn encode_pieces(&self, text: &str) -> Result<Vec<String>, NeedleError> {
        self.encode(text)?
            .into_iter()
            .map(|id| {
                self.sp
                    .id_to_piece(i32::try_from(id).unwrap_or(-1))
                    .map(str::to_string)
                    .ok_or_else(|| NeedleError::SentencePiece("unknown token id".to_string()))
            })
            .collect()
    }
    pub fn decode(&self, ids: &[u32]) -> Result<String, NeedleError> {
        let ids: Vec<i32> = ids
            .iter()
            .map(|&id| {
                i32::try_from(id)
                    .map_err(|_| NeedleError::SentencePiece("token id overflow".to_string()))
            })
            .collect::<Result<_, _>>()?;
        self.sp
            .decode(&ids)
            .map_err(|err| NeedleError::SentencePiece(err.to_string()))
    }
    fn token_string(&self, id: u32) -> String {
        let Some(piece) = self.sp.id_to_piece(i32::try_from(id).unwrap_or(-1)) else {
            return String::new();
        };
        match self
            .sp
            .vocab()
            .kind(i32::try_from(id).unwrap_or(-1))
            .unwrap_or(PieceType::Normal)
        {
            PieceType::Control | PieceType::Unknown => String::new(),
            PieceType::Byte => {
                let bytes = piece.as_bytes();
                if bytes.len() == 6 && &bytes[..3] == b"<0x" && bytes[5] == b'>' {
                    u8::from_str_radix(&piece[3..5], 16)
                        .map(|byte| char::from(byte).to_string())
                        .unwrap_or_default()
                } else {
                    String::new()
                }
            }
            _ => piece.replace('\u{2581}', " "),
        }
    }
}

fn encode_with_special_tokens(
    sp: &SentencePieceProcessor,
    text: &str,
) -> Result<Vec<u32>, NeedleError> {
    const SPECIALS: [(&str, u32); 2] = [("<tool_call>", TOKEN_TOOL_CALL), ("<tools>", TOKEN_TOOLS)];
    let mut out = Vec::new();
    let mut cursor = 0;
    let mut after_special = false;
    while cursor < text.len() {
        let next = SPECIALS
            .iter()
            .filter_map(|(needle, id)| {
                text[cursor..]
                    .find(needle)
                    .map(|pos| (cursor + pos, *needle, *id))
            })
            .min_by_key(|(pos, _, _)| *pos);
        if let Some((pos, needle, id)) = next {
            if pos > cursor {
                out.extend(encode_fragment(sp, &text[cursor..pos], after_special)?);
            } else if cursor == 0 {
                if let Some(prefix) = sp.vocab().piece_to_id("▁".as_bytes()) {
                    out.push(u32::try_from(prefix).map_err(|_| {
                        NeedleError::SentencePiece("negative prefix token id".to_string())
                    })?);
                }
            }
            out.push(id);
            cursor = pos + needle.len();
            after_special = true;
        } else {
            out.extend(encode_fragment(sp, &text[cursor..], after_special)?);
            break;
        }
    }
    Ok(out)
}

fn encode_fragment(
    sp: &SentencePieceProcessor,
    text: &str,
    suppress_dummy_prefix: bool,
) -> Result<Vec<u32>, NeedleError> {
    let mut tokens = sp
        .encode(text)
        .map_err(|err| NeedleError::SentencePiece(err.to_string()))?;
    if suppress_dummy_prefix {
        if let Some(first) = tokens.first_mut() {
            if let Some(piece) = sp.id_to_piece(*first) {
                if let Some(unprefixed) = piece.strip_prefix('▁') {
                    if let Some(replacement) = sp.vocab().piece_to_id(unprefixed.as_bytes()) {
                        *first = replacement;
                    } else if piece == "▁" {
                        tokens.remove(0);
                    }
                }
            }
        }
    }
    tokens
        .into_iter()
        .map(|token| {
            u32::try_from(token)
                .map_err(|_| NeedleError::SentencePiece("negative token id".to_string()))
        })
        .collect()
}

pub type Checkpoint<'a> = &'a mut dyn FnMut(&'static str) -> Result<(), NeedleError>;

#[derive(Debug, Clone, Copy)]
pub struct GenerationOptions {
    pub max_gen_len: usize,
    pub max_enc_len: usize,
    pub constrained: bool,
}

impl Default for GenerationOptions {
    fn default() -> Self {
        Self {
            max_gen_len: 512,
            max_enc_len: 1024,
            constrained: true,
        }
    }
}

#[derive(Debug, Clone)]
pub struct RouterModel {
    config: NeedleConfig,
    embedding: Vec<f32>,
    encoder_final_norm: Vec<f32>,
    decoder_final_norm: Vec<f32>,
    encoder_norm0: Vec<f32>,
    encoder_attn_gate: Vec<f32>,
    encoder_q_norm: Vec<f32>,
    encoder_k_norm: Vec<f32>,
    encoder_q_proj: Vec<f32>,
    encoder_k_proj: Vec<f32>,
    encoder_v_proj: Vec<f32>,
    encoder_out_proj: Vec<f32>,
    decoder_norm0: Vec<f32>,
    decoder_norm1: Vec<f32>,
    decoder_self_gate: Vec<f32>,
    decoder_cross_gate: Vec<f32>,
    decoder_self_q_norm: Vec<f32>,
    decoder_self_k_norm: Vec<f32>,
    decoder_self_q_proj: Vec<f32>,
    decoder_self_k_proj: Vec<f32>,
    decoder_self_v_proj: Vec<f32>,
    decoder_self_out_proj: Vec<f32>,
    decoder_cross_q_norm: Vec<f32>,
    decoder_cross_k_norm: Vec<f32>,
    decoder_cross_q_proj: Vec<f32>,
    decoder_cross_k_proj: Vec<f32>,
    decoder_cross_v_proj: Vec<f32>,
    decoder_cross_out_proj: Vec<f32>,
}

impl RouterModel {
    #[allow(clippy::too_many_lines)]
    pub fn from_ndl(model: &Model) -> Result<Self, NeedleError> {
        let c = model.config();
        let dm = c.d_model;
        let nh = c.num_heads;
        let nkv = c.num_kv_heads;
        let nl_enc = c.num_encoder_layers;
        let nl_dec = c.num_decoder_layers;
        let hd = dm / nh;
        let kvd = nkv * hd;
        Ok(Self {
            config: c.clone(),
            embedding: model.tensor_f32("embedding.embedding", &[c.vocab_size, dm])?,
            encoder_final_norm: model.tensor_f32("encoder.final_norm.scale", &[dm])?,
            decoder_final_norm: model.tensor_f32("decoder.ZCRMSNorm_0.scale", &[dm])?,
            encoder_norm0: model.tensor_f32(
                "encoder.layers.EncoderBlock_0.ZCRMSNorm_0.scale",
                &[nl_enc, dm],
            )?,
            encoder_attn_gate: model
                .tensor_f32("encoder.layers.EncoderBlock_0.attn_gate", &[nl_enc])?,
            encoder_q_norm: model.tensor_f32(
                "encoder.layers.EncoderBlock_0.self_attn.q_norm.scale",
                &[nl_enc, hd],
            )?,
            encoder_k_norm: model.tensor_f32(
                "encoder.layers.EncoderBlock_0.self_attn.k_norm.scale",
                &[nl_enc, hd],
            )?,
            encoder_q_proj: model.tensor_f32(
                "encoder.layers.EncoderBlock_0.self_attn.q_proj.kernel",
                &[nl_enc, dm, dm],
            )?,
            encoder_k_proj: model.tensor_f32(
                "encoder.layers.EncoderBlock_0.self_attn.k_proj.kernel",
                &[nl_enc, dm, kvd],
            )?,
            encoder_v_proj: model.tensor_f32(
                "encoder.layers.EncoderBlock_0.self_attn.v_proj.kernel",
                &[nl_enc, dm, kvd],
            )?,
            encoder_out_proj: model.tensor_f32(
                "encoder.layers.EncoderBlock_0.self_attn.out_proj.kernel",
                &[nl_enc, dm, dm],
            )?,
            decoder_norm0: model.tensor_f32(
                "decoder.layers.DecoderBlock_0.ZCRMSNorm_0.scale",
                &[nl_dec, dm],
            )?,
            decoder_norm1: model.tensor_f32(
                "decoder.layers.DecoderBlock_0.ZCRMSNorm_1.scale",
                &[nl_dec, dm],
            )?,
            decoder_self_gate: model
                .tensor_f32("decoder.layers.DecoderBlock_0.self_attn_gate", &[nl_dec])?,
            decoder_cross_gate: model
                .tensor_f32("decoder.layers.DecoderBlock_0.cross_attn_gate", &[nl_dec])?,
            decoder_self_q_norm: model.tensor_f32(
                "decoder.layers.DecoderBlock_0.self_attn.q_norm.scale",
                &[nl_dec, hd],
            )?,
            decoder_self_k_norm: model.tensor_f32(
                "decoder.layers.DecoderBlock_0.self_attn.k_norm.scale",
                &[nl_dec, hd],
            )?,
            decoder_self_q_proj: model.tensor_f32(
                "decoder.layers.DecoderBlock_0.self_attn.q_proj.kernel",
                &[nl_dec, dm, dm],
            )?,
            decoder_self_k_proj: model.tensor_f32(
                "decoder.layers.DecoderBlock_0.self_attn.k_proj.kernel",
                &[nl_dec, dm, kvd],
            )?,
            decoder_self_v_proj: model.tensor_f32(
                "decoder.layers.DecoderBlock_0.self_attn.v_proj.kernel",
                &[nl_dec, dm, kvd],
            )?,
            decoder_self_out_proj: model.tensor_f32(
                "decoder.layers.DecoderBlock_0.self_attn.out_proj.kernel",
                &[nl_dec, dm, dm],
            )?,
            decoder_cross_q_norm: model.tensor_f32(
                "decoder.layers.DecoderBlock_0.cross_attn.q_norm.scale",
                &[nl_dec, hd],
            )?,
            decoder_cross_k_norm: model.tensor_f32(
                "decoder.layers.DecoderBlock_0.cross_attn.k_norm.scale",
                &[nl_dec, hd],
            )?,
            decoder_cross_q_proj: model.tensor_f32(
                "decoder.layers.DecoderBlock_0.cross_attn.q_proj.kernel",
                &[nl_dec, dm, dm],
            )?,
            decoder_cross_k_proj: model.tensor_f32(
                "decoder.layers.DecoderBlock_0.cross_attn.k_proj.kernel",
                &[nl_dec, dm, kvd],
            )?,
            decoder_cross_v_proj: model.tensor_f32(
                "decoder.layers.DecoderBlock_0.cross_attn.v_proj.kernel",
                &[nl_dec, dm, kvd],
            )?,
            decoder_cross_out_proj: model.tensor_f32(
                "decoder.layers.DecoderBlock_0.cross_attn.out_proj.kernel",
                &[nl_dec, dm, dm],
            )?,
        })
    }

    pub fn generate(
        &self,
        tokenizer: &NeedleTokenizer,
        query: &str,
        tools_json: &str,
        options: GenerationOptions,
        mut checkpoint: Option<Checkpoint<'_>>,
    ) -> Result<String, NeedleError> {
        let (tools_json, name_map) = normalize_tools_json(tools_json);
        let enc_tokens = build_encoder_input(tokenizer, query, &tools_json, options.max_enc_len)?;
        poll(&mut checkpoint, "tokenized")?;
        let encoder_out = self.encode(&enc_tokens, &mut checkpoint)?;
        poll(&mut checkpoint, "encoded")?;
        let mut decoder = ConstrainedDecoder::new(&tools_json, tokenizer);
        let mut state = self.build_decoder_state(&encoder_out, &enc_tokens, &mut checkpoint)?;
        let mut token = tokenizer.eos_token_id();
        let mut generated = Vec::new();
        for _ in 0..options.max_gen_len {
            let hidden = self.decode_step(token, generated.len(), &mut state, &mut checkpoint)?;
            let next = if options.constrained {
                decoder
                    .allowed_token_ids()
                    .filter(|allowed| !allowed.is_empty())
                    .map_or_else(
                        || argmax_logits(&hidden, &self.embedding, self.config.d_model as usize),
                        |allowed| {
                            argmax_logits_allowed(
                                &hidden,
                                &self.embedding,
                                self.config.d_model as usize,
                                allowed,
                            )
                        },
                    )
            } else {
                argmax_logits(&hidden, &self.embedding, self.config.d_model as usize)
            } as u32;
            if options.constrained {
                decoder.update(next);
            }
            if next == tokenizer.eos_token_id() {
                break;
            }
            generated.push(next);
            token = next;
        }
        if generated.len() == options.max_gen_len {
            return Err(NeedleError::GenerationTooLong(options.max_gen_len));
        }
        let mut text = tokenizer.decode(&generated)?;
        if let Some(stripped) = text.strip_prefix("<tool_call>") {
            text = stripped.to_string();
        }
        Ok(restore_tool_names(&text, &name_map))
    }

    fn encode(
        &self,
        tokens: &[u32],
        checkpoint: &mut Option<Checkpoint<'_>>,
    ) -> Result<Vec<f32>, NeedleError> {
        let dm = self.config.d_model as usize;
        let layers = self.config.num_encoder_layers as usize;
        let mut x = vec![0.0; tokens.len() * dm];
        let scale = (dm as f32).sqrt();
        for (t, &id) in tokens.iter().enumerate() {
            let src = &self.embedding[id as usize * dm..(id as usize + 1) * dm];
            let dst = &mut x[t * dm..(t + 1) * dm];
            for i in 0..dm {
                dst[i] = src[i] * scale;
            }
        }
        let rope = precompute_rope(
            dm / self.config.num_heads as usize,
            tokens.len(),
            self.config.rope_theta,
        );
        for layer in 0..layers {
            poll(checkpoint, "encoder_layer")?;
            let norm = layer_slice(&self.encoder_norm0, layer, dm);
            let xn = apply_zcrmsnorm_rows(&x, dm, norm);
            let attn = self.attend_full(&xn, &xn, tokens, layer, &rope, true, false);
            let gate = sigmoid(self.encoder_attn_gate[layer]);
            for i in 0..x.len() {
                x[i] += gate * attn[i];
            }
        }
        Ok(apply_zcrmsnorm_rows(&x, dm, &self.encoder_final_norm))
    }

    fn build_decoder_state(
        &self,
        encoder_out: &[f32],
        enc_tokens: &[u32],
        checkpoint: &mut Option<Checkpoint<'_>>,
    ) -> Result<DecoderState, NeedleError> {
        let dm = self.config.d_model as usize;
        let layers = self.config.num_decoder_layers as usize;
        let kv_heads = self.config.num_kv_heads as usize;
        let head_dim = dm / self.config.num_heads as usize;
        let mut cross = Vec::with_capacity(layers);
        for layer in 0..layers {
            poll(checkpoint, "decoder_cross_prep")?;
            let kn = layer_slice(&self.decoder_cross_k_norm, layer, head_dim);
            let k_proj = layer_slice(&self.decoder_cross_k_proj, layer, dm * kv_heads * head_dim);
            let v_proj = layer_slice(&self.decoder_cross_v_proj, layer, dm * kv_heads * head_dim);
            let mut k = project_rows(encoder_out, dm, kv_heads * head_dim, k_proj);
            let v = project_rows(encoder_out, dm, kv_heads * head_dim, v_proj);
            apply_head_norm_rows(&mut k, enc_tokens.len(), kv_heads, head_dim, kn);
            cross.push(CrossCache { k, v });
        }
        let cache_capacity = self.config.max_seq_len as usize * kv_heads * head_dim;
        Ok(DecoderState {
            self_k: (0..layers)
                .map(|_| Vec::with_capacity(cache_capacity))
                .collect(),
            self_v: (0..layers)
                .map(|_| Vec::with_capacity(cache_capacity))
                .collect(),
            cross,
            rope: precompute_rope(
                head_dim,
                self.config.max_seq_len as usize,
                self.config.rope_theta,
            ),
        })
    }

    fn decode_step(
        &self,
        token: u32,
        pos: usize,
        state: &mut DecoderState,
        checkpoint: &mut Option<Checkpoint<'_>>,
    ) -> Result<Vec<f32>, NeedleError> {
        let dm = self.config.d_model as usize;
        let heads = self.config.num_heads as usize;
        let kv_heads = self.config.num_kv_heads as usize;
        let head_dim = dm / heads;
        let seq_scale = (dm as f32).sqrt();
        let mut x = self.embedding[token as usize * dm..(token as usize + 1) * dm]
            .iter()
            .map(|v| v * seq_scale)
            .collect::<Vec<_>>();
        for layer in 0..self.config.num_decoder_layers as usize {
            poll(checkpoint, "decoder_layer")?;
            let norm = layer_slice(&self.decoder_norm0, layer, dm);
            let xn = apply_zcrmsnorm_vec(&x, norm);
            let q_proj = layer_slice(&self.decoder_self_q_proj, layer, dm * dm);
            let k_proj = layer_slice(&self.decoder_self_k_proj, layer, dm * kv_heads * head_dim);
            let v_proj = layer_slice(&self.decoder_self_v_proj, layer, dm * kv_heads * head_dim);
            let out_proj = layer_slice(&self.decoder_self_out_proj, layer, dm * dm);
            let mut q = project_vec(&xn, dm, dm, q_proj);
            let mut k = project_vec(&xn, dm, kv_heads * head_dim, k_proj);
            let v = project_vec(&xn, dm, kv_heads * head_dim, v_proj);
            apply_head_norm_vec(
                &mut q,
                heads,
                head_dim,
                layer_slice(&self.decoder_self_q_norm, layer, head_dim),
            );
            apply_head_norm_vec(
                &mut k,
                kv_heads,
                head_dim,
                layer_slice(&self.decoder_self_k_norm, layer, head_dim),
            );
            apply_rope_position(&mut q, heads, head_dim, &state.rope, pos);
            apply_rope_position(&mut k, kv_heads, head_dim, &state.rope, pos);
            state.self_k[layer].extend_from_slice(&k);
            state.self_v[layer].extend_from_slice(&v);
            let self_ctx = attend_single(
                &q,
                &state.self_k[layer],
                &state.self_v[layer],
                heads,
                kv_heads,
                head_dim,
            );
            let self_out = project_vec(&self_ctx, dm, dm, out_proj);
            let gate = sigmoid(self.decoder_self_gate[layer]);
            for i in 0..dm {
                x[i] += gate * self_out[i];
            }

            let xn = apply_zcrmsnorm_vec(&x, layer_slice(&self.decoder_norm1, layer, dm));
            let q_proj = layer_slice(&self.decoder_cross_q_proj, layer, dm * dm);
            let out_proj = layer_slice(&self.decoder_cross_out_proj, layer, dm * dm);
            let mut q = project_vec(&xn, dm, dm, q_proj);
            apply_head_norm_vec(
                &mut q,
                heads,
                head_dim,
                layer_slice(&self.decoder_cross_q_norm, layer, head_dim),
            );
            let cross_ctx = attend_single(
                &q,
                &state.cross[layer].k,
                &state.cross[layer].v,
                heads,
                kv_heads,
                head_dim,
            );
            let cross_out = project_vec(&cross_ctx, dm, dm, out_proj);
            let gate = sigmoid(self.decoder_cross_gate[layer]);
            for i in 0..dm {
                x[i] += gate * cross_out[i];
            }
        }
        Ok(apply_zcrmsnorm_vec(&x, &self.decoder_final_norm))
    }

    #[allow(clippy::too_many_arguments)]
    fn attend_full(
        &self,
        q_input: &[f32],
        kv_input: &[f32],
        tokens: &[u32],
        layer: usize,
        rope: &Rope,
        encoder: bool,
        cross: bool,
    ) -> Vec<f32> {
        let dm = self.config.d_model as usize;
        let heads = self.config.num_heads as usize;
        let kv_heads = self.config.num_kv_heads as usize;
        let head_dim = dm / heads;
        let q_kernel = if encoder {
            layer_slice(&self.encoder_q_proj, layer, dm * dm)
        } else if cross {
            layer_slice(&self.decoder_cross_q_proj, layer, dm * dm)
        } else {
            layer_slice(&self.decoder_self_q_proj, layer, dm * dm)
        };
        let k_kernel = if encoder {
            layer_slice(&self.encoder_k_proj, layer, dm * kv_heads * head_dim)
        } else if cross {
            layer_slice(&self.decoder_cross_k_proj, layer, dm * kv_heads * head_dim)
        } else {
            layer_slice(&self.decoder_self_k_proj, layer, dm * kv_heads * head_dim)
        };
        let v_kernel = if encoder {
            layer_slice(&self.encoder_v_proj, layer, dm * kv_heads * head_dim)
        } else if cross {
            layer_slice(&self.decoder_cross_v_proj, layer, dm * kv_heads * head_dim)
        } else {
            layer_slice(&self.decoder_self_v_proj, layer, dm * kv_heads * head_dim)
        };
        let out_kernel = if encoder {
            layer_slice(&self.encoder_out_proj, layer, dm * dm)
        } else if cross {
            layer_slice(&self.decoder_cross_out_proj, layer, dm * dm)
        } else {
            layer_slice(&self.decoder_self_out_proj, layer, dm * dm)
        };
        let q_norm = if encoder {
            layer_slice(&self.encoder_q_norm, layer, head_dim)
        } else if cross {
            layer_slice(&self.decoder_cross_q_norm, layer, head_dim)
        } else {
            layer_slice(&self.decoder_self_q_norm, layer, head_dim)
        };
        let k_norm = if encoder {
            layer_slice(&self.encoder_k_norm, layer, head_dim)
        } else if cross {
            layer_slice(&self.decoder_cross_k_norm, layer, head_dim)
        } else {
            layer_slice(&self.decoder_self_k_norm, layer, head_dim)
        };
        let mut q = project_rows(q_input, dm, dm, q_kernel);
        let mut k = project_rows(kv_input, dm, kv_heads * head_dim, k_kernel);
        let v = project_rows(kv_input, dm, kv_heads * head_dim, v_kernel);
        apply_head_norm_rows(&mut q, tokens.len(), heads, head_dim, q_norm);
        apply_head_norm_rows(&mut k, tokens.len(), kv_heads, head_dim, k_norm);
        apply_rope_rows(&mut q, tokens.len(), heads, head_dim, rope);
        apply_rope_rows(&mut k, tokens.len(), kv_heads, head_dim, rope);
        let mut contexts = vec![0.0; tokens.len() * dm];
        for t in 0..tokens.len() {
            let ctx = attend_single(&q[t * dm..(t + 1) * dm], &k, &v, heads, kv_heads, head_dim);
            contexts[t * dm..(t + 1) * dm].copy_from_slice(&ctx);
        }
        project_rows(&contexts, dm, dm, out_kernel)
    }
}

#[derive(Debug, Clone)]
struct Rope {
    cos: Vec<f32>,
    sin: Vec<f32>,
}
#[derive(Debug, Clone)]
struct CrossCache {
    k: Vec<f32>,
    v: Vec<f32>,
}
#[derive(Debug, Clone)]
struct DecoderState {
    self_k: Vec<Vec<f32>>,
    self_v: Vec<Vec<f32>>,
    cross: Vec<CrossCache>,
    rope: Rope,
}

#[derive(Debug, Clone)]
pub struct NameMap(HashMap<String, String>);

fn poll(checkpoint: &mut Option<Checkpoint<'_>>, label: &'static str) -> Result<(), NeedleError> {
    if let Some(cp) = checkpoint.as_mut() {
        cp(label)?;
    }
    Ok(())
}

fn build_encoder_input(
    tokenizer: &NeedleTokenizer,
    query: &str,
    tools: &str,
    max_enc_len: usize,
) -> Result<Vec<u32>, NeedleError> {
    let mut q = tokenizer.encode(query)?;
    let mut t = tokenizer.encode(tools)?;
    let max_query = max_enc_len.saturating_sub(2);
    if q.len() > max_query {
        q.truncate(max_query);
    }
    let remaining = max_enc_len.saturating_sub(q.len() + 1);
    if t.len() > remaining {
        t.truncate(remaining);
    }
    q.push(tokenizer.tools_token_id());
    q.extend(t);
    Ok(q)
}

#[must_use]
pub fn to_snake_case(name: &str) -> String {
    let mut out = String::new();
    let mut prev_lower = false;
    let mut prev_upper = false;
    for ch in name.chars() {
        if ch.is_ascii_alphanumeric() || ch == '_' {
            if ch.is_ascii_uppercase() {
                if !out.is_empty()
                    && (prev_lower
                        || prev_upper && out.chars().last().is_some_and(|c| c.is_ascii_lowercase()))
                {
                    out.push('_');
                }
                out.push(ch.to_ascii_lowercase());
                prev_lower = false;
                prev_upper = true;
            } else {
                out.push(ch.to_ascii_lowercase());
                prev_lower = ch.is_ascii_lowercase() || ch.is_ascii_digit();
                prev_upper = false;
            }
        } else if !out.ends_with('_') {
            out.push('_');
            prev_lower = false;
            prev_upper = false;
        }
    }
    out.trim_matches('_')
        .split('_')
        .filter(|s| !s.is_empty())
        .collect::<Vec<_>>()
        .join("_")
}

pub fn normalize_tools_json(tools_json: &str) -> (String, NameMap) {
    let mut map = HashMap::new();
    let Ok(mut value) = serde_json::from_str::<serde_json::Value>(tools_json) else {
        return (tools_json.to_string(), NameMap(map));
    };
    if let Some(items) = value.as_array_mut() {
        for item in items {
            let name = item
                .get("name")
                .and_then(serde_json::Value::as_str)
                .map(str::to_string);
            if let Some(name) = name {
                let snake = to_snake_case(&name);
                map.insert(snake.clone(), name);
                if let Some(obj) = item.as_object_mut() {
                    obj.insert("name".to_string(), serde_json::Value::String(snake));
                }
            }
        }
    }
    (
        serde_json::to_string(&value).unwrap_or_else(|_| tools_json.to_string()),
        NameMap(map),
    )
}

#[must_use]
pub fn restore_tool_names(text: &str, name_map: &NameMap) -> String {
    if name_map.0.is_empty() {
        return text.to_string();
    }
    if let Ok(mut value) = serde_json::from_str::<serde_json::Value>(text) {
        match &mut value {
            serde_json::Value::Array(items) => {
                for item in items {
                    restore_name_in_value(item, name_map);
                }
            }
            other => restore_name_in_value(other, name_map),
        }
        return serde_json::to_string(&value).unwrap_or_else(|_| text.to_string());
    }
    let mut out = text.to_string();
    let mut keys = name_map.0.keys().collect::<Vec<_>>();
    keys.sort_by_key(|k| std::cmp::Reverse(k.len()));
    for snake in keys {
        if let Some(orig) = name_map.0.get(snake) {
            out = out.replace(snake, orig);
        }
    }
    out
}

fn restore_name_in_value(value: &mut serde_json::Value, name_map: &NameMap) {
    if let Some(obj) = value.as_object_mut() {
        if let Some(name) = obj.get_mut("name") {
            if let Some(s) = name.as_str() {
                if let Some(orig) = name_map.0.get(s) {
                    *name = serde_json::Value::String(orig.clone());
                }
            }
        }
    }
}

struct ToolConstraints {
    names: Trie,
    params: HashMap<String, Trie>,
}
struct ConstrainedDecoder {
    machine: JsonStateMachine,
    token_strings: Vec<String>,
    constraints: ToolConstraints,
}
#[derive(Clone, Copy, PartialEq, Eq, Default)]
enum JsonState {
    #[default]
    Free,
    InName,
    InArgKey,
}
#[derive(Default)]
struct TrieNode {
    children: BTreeMap<char, TrieNode>,
    terminal: bool,
}
#[derive(Default)]
struct Trie {
    root: TrieNode,
}

impl Trie {
    fn insert(&mut self, word: &str) {
        let mut node = &mut self.root;
        for ch in word.chars() {
            node = node.children.entry(ch).or_default();
        }
        node.terminal = true;
    }
    fn node<'a>(&'a self, prefix: &str) -> Option<&'a TrieNode> {
        let mut node = &self.root;
        for ch in prefix.chars() {
            node = node.children.get(&ch)?;
        }
        Some(node)
    }
}
impl ConstrainedDecoder {
    fn new(tools_json: &str, tokenizer: &NeedleTokenizer) -> Self {
        let mut names = Trie::default();
        let mut params = HashMap::new();
        if let Ok(value) = serde_json::from_str::<serde_json::Value>(tools_json) {
            if let Some(items) = value.as_array() {
                for item in items {
                    let Some(name) = item.get("name").and_then(serde_json::Value::as_str) else {
                        continue;
                    };
                    names.insert(name);
                    let mut trie = Trie::default();
                    if let Some(props) = item
                        .get("parameters")
                        .and_then(serde_json::Value::as_object)
                    {
                        for (key, value) in props {
                            if value.is_object() {
                                trie.insert(key);
                            }
                        }
                    }
                    params.insert(name.to_string(), trie);
                }
            }
        }
        Self {
            machine: JsonStateMachine::default(),
            token_strings: (0..tokenizer.vocab_size())
                .map(|id| tokenizer.token_string(id as u32))
                .collect(),
            constraints: ToolConstraints { names, params },
        }
    }
    fn update(&mut self, token: u32) {
        if let Some(text) = self.token_strings.get(token as usize) {
            self.machine.feed(text);
        }
    }
    fn allowed_token_ids(&self) -> Option<Vec<usize>> {
        let trie = match self.machine.state {
            JsonState::Free => return None,
            JsonState::InName => &self.constraints.names,
            JsonState::InArgKey => self
                .constraints
                .params
                .get(&self.machine.current_function)?,
        };
        let node = trie.node(&self.machine.constrained_buf)?;
        let mut allowed = Vec::new();
        let mut starts = node.children.keys().copied().collect::<Vec<_>>();
        if node.terminal {
            starts.push('"');
        }
        for first in starts {
            for (id, text) in self.token_strings.iter().enumerate() {
                if text.starts_with(first) && token_valid(text, node) {
                    allowed.push(id);
                }
            }
        }
        allowed.sort_unstable();
        allowed.dedup();
        Some(allowed)
    }
}

#[derive(Default)]
struct JsonStateMachine {
    state: JsonState,
    buffer: String,
    constrained_buf: String,
    current_function: String,
    in_arguments: bool,
    arguments_depth: usize,
    nesting_depth: usize,
    in_string: bool,
    prev_escape: bool,
}
impl JsonStateMachine {
    fn feed(&mut self, text: &str) {
        for ch in text.chars() {
            self.feed_char(ch);
        }
    }
    fn feed_char(&mut self, ch: char) {
        if matches!(self.state, JsonState::InName | JsonState::InArgKey) {
            if ch == '"' {
                if self.state == JsonState::InName {
                    self.current_function = self.constrained_buf.clone();
                }
                self.constrained_buf.clear();
                self.state = JsonState::Free;
            } else {
                self.constrained_buf.push(ch);
            }
            self.buffer.push(ch);
            return;
        }
        self.buffer.push(ch);
        if self.in_string {
            if self.prev_escape {
                self.prev_escape = false;
                return;
            }
            if ch == '\\' {
                self.prev_escape = true;
                return;
            }
            if ch == '"' {
                self.in_string = false;
            }
            return;
        }
        if ch == '{' || ch == '[' {
            self.nesting_depth += 1;
        }
        if ch == '}' || ch == ']' {
            self.nesting_depth = self.nesting_depth.saturating_sub(1);
            if ch == '}' && self.in_arguments && self.nesting_depth < self.arguments_depth {
                self.in_arguments = false;
            }
            return;
        }
        if self.buffer.ends_with("\"name\":\"") && !self.in_arguments {
            self.state = JsonState::InName;
            return;
        }
        if self.buffer.ends_with("\"arguments\":{") {
            self.in_arguments = true;
            self.arguments_depth = self.nesting_depth;
            return;
        }
        if self.in_arguments
            && self.nesting_depth == self.arguments_depth
            && (self.buffer.ends_with("{\"") || self.buffer.ends_with(",\""))
        {
            self.state = JsonState::InArgKey;
            return;
        }
        if ch == '"'
            && self.buffer[..self.buffer.len().saturating_sub(1)]
                .chars()
                .rev()
                .find(|c| !c.is_whitespace())
                .is_some_and(|c| c == ':')
        {
            self.in_string = true;
        }
    }
}
fn token_valid(token_text: &str, node: &TrieNode) -> bool {
    let mut node = node;
    for ch in token_text.chars() {
        if ch == '"' {
            return node.terminal;
        }
        let Some(next) = node.children.get(&ch) else {
            return false;
        };
        node = next;
    }
    true
}

fn precompute_rope(head_dim: usize, seq_len: usize, theta: f32) -> Rope {
    let half = head_dim / 2;
    let mut cos = vec![0.0; seq_len * half];
    let mut sin = vec![0.0; seq_len * half];
    for t in 0..seq_len {
        for i in 0..half {
            let freq = 1.0 / theta.powf((2 * i) as f32 / head_dim as f32);
            let angle = t as f32 * freq;
            cos[t * half + i] = angle.cos();
            sin[t * half + i] = angle.sin();
        }
    }
    Rope { cos, sin }
}
#[allow(clippy::many_single_char_names)]
fn apply_rope_position(x: &mut [f32], heads: usize, head_dim: usize, rope: &Rope, pos: usize) {
    let half = head_dim / 2;
    for h in 0..heads {
        let base = h * head_dim;
        for i in 0..half {
            let c = rope.cos[pos * half + i];
            let s = rope.sin[pos * half + i];
            let a = x[base + i];
            let b = x[base + half + i];
            x[base + i] = a * c - b * s;
            x[base + half + i] = b * c + a * s;
        }
    }
}
fn apply_rope_rows(x: &mut [f32], rows: usize, heads: usize, head_dim: usize, rope: &Rope) {
    for r in 0..rows {
        apply_rope_position(
            &mut x[r * heads * head_dim..(r + 1) * heads * head_dim],
            heads,
            head_dim,
            rope,
            r,
        );
    }
}
fn apply_zcrmsnorm_vec(x: &[f32], scale: &[f32]) -> Vec<f32> {
    let rms = ((x.iter().map(|v| v * v).sum::<f32>() / x.len() as f32) + EPSILON).sqrt();
    x.iter()
        .zip(scale)
        .map(|(v, s)| (1.0 + s) * v / rms)
        .collect()
}
fn apply_zcrmsnorm_rows(x: &[f32], width: usize, scale: &[f32]) -> Vec<f32> {
    let mut out = vec![0.0; x.len()];
    for row in 0..x.len() / width {
        out[row * width..(row + 1) * width].copy_from_slice(&apply_zcrmsnorm_vec(
            &x[row * width..(row + 1) * width],
            scale,
        ));
    }
    out
}
fn apply_head_norm_vec(x: &mut [f32], heads: usize, head_dim: usize, scale: &[f32]) {
    for h in 0..heads {
        let base = h * head_dim;
        let rms = ((x[base..base + head_dim].iter().map(|v| v * v).sum::<f32>() / head_dim as f32)
            + EPSILON)
            .sqrt();
        for i in 0..head_dim {
            x[base + i] = (1.0 + scale[i]) * x[base + i] / rms;
        }
    }
}
fn apply_head_norm_rows(x: &mut [f32], rows: usize, heads: usize, head_dim: usize, scale: &[f32]) {
    for row in 0..rows {
        apply_head_norm_vec(
            &mut x[row * heads * head_dim..(row + 1) * heads * head_dim],
            heads,
            head_dim,
            scale,
        );
    }
}
fn project_vec(input: &[f32], in_dim: usize, out_dim: usize, kernel: &[f32]) -> Vec<f32> {
    let mut out = vec![0.0; out_dim];
    for i in 0..in_dim {
        let row = &kernel[i * out_dim..(i + 1) * out_dim];
        memento_vector::axpy(input[i], row, &mut out)
            .expect("internal projection dimensions match");
    }
    out
}
fn project_rows(input: &[f32], in_dim: usize, out_dim: usize, kernel: &[f32]) -> Vec<f32> {
    let rows = input.len() / in_dim;
    if rows == 1 {
        return project_vec(input, in_dim, out_dim, kernel);
    }
    memento_vector::linear_in_out(input, rows, in_dim, kernel, out_dim, None)
        .expect("internal Needle projection dimensions match")
}
fn attend_single(
    q: &[f32],
    k_all: &[f32],
    v_all: &[f32],
    heads: usize,
    kv_heads: usize,
    head_dim: usize,
) -> Vec<f32> {
    let kv_tokens = if kv_heads == 0 || head_dim == 0 {
        0
    } else {
        k_all.len() / (kv_heads * head_dim)
    };
    let repeats = heads / kv_heads;
    let mut out = vec![0.0; heads * head_dim];
    let mut scores = vec![0.0; kv_tokens];
    let scale = (head_dim as f32).sqrt();
    for h in 0..heads {
        let qh = &q[h * head_dim..(h + 1) * head_dim];
        let kh = h / repeats;
        scores.fill(0.0);
        let mut max = f32::NEG_INFINITY;
        for (token_index, score_slot) in scores.iter_mut().enumerate() {
            let base = token_index * kv_heads * head_dim + kh * head_dim;
            let score = dot(qh, &k_all[base..base + head_dim]) / scale;
            *score_slot = score;
            max = max.max(score);
        }
        let mut sum = 0.0;
        for s in &mut scores {
            *s = (*s - max).exp();
            sum += *s;
        }
        if sum == 0.0 {
            continue;
        }
        for (token_index, score) in scores.iter().enumerate() {
            let weight = score / sum;
            let base = token_index * kv_heads * head_dim + kh * head_dim;
            for i in 0..head_dim {
                out[h * head_dim + i] += weight * v_all[base + i];
            }
        }
    }
    out
}
fn argmax_logits(hidden: &[f32], embedding: &[f32], dim: usize) -> usize {
    argmax_logits_allowed(hidden, embedding, dim, 0..embedding.len() / dim)
}

fn argmax_logits_allowed(
    hidden: &[f32],
    embedding: &[f32],
    dim: usize,
    token_ids: impl IntoIterator<Item = usize>,
) -> usize {
    let mut best = 0;
    let mut best_score = f32::NEG_INFINITY;
    for id in token_ids {
        let score = dot(hidden, &embedding[id * dim..(id + 1) * dim]);
        if score > best_score {
            best_score = score;
            best = id;
        }
    }
    best
}
fn sigmoid(x: f32) -> f32 {
    1.0 / (1.0 + (-x).exp())
}
fn dot(left: &[f32], right: &[f32]) -> f32 {
    memento_vector::dot(left, right).expect("internal vector dimensions match")
}
fn layer_slice(values: &[f32], layer: usize, len: usize) -> &[f32] {
    &values[layer * len..(layer + 1) * len]
}

fn read_u16(bytes: &[u8], offset: usize) -> Result<u16, NeedleError> {
    Ok(u16::from_le_bytes(
        bytes
            .get(offset..offset + 2)
            .ok_or_else(|| NeedleError::InvalidModel("unexpected end of file".to_string()))?
            .try_into()
            .expect("2-byte slice"),
    ))
}
fn read_u32(bytes: &[u8], offset: usize) -> Result<u32, NeedleError> {
    Ok(u32::from_le_bytes(
        bytes
            .get(offset..offset + 4)
            .ok_or_else(|| NeedleError::InvalidModel("unexpected end of file".to_string()))?
            .try_into()
            .expect("4-byte slice"),
    ))
}
fn read_u64(bytes: &[u8], offset: usize) -> Result<u64, NeedleError> {
    Ok(u64::from_le_bytes(
        bytes
            .get(offset..offset + 8)
            .ok_or_else(|| NeedleError::InvalidModel("unexpected end of file".to_string()))?
            .try_into()
            .expect("8-byte slice"),
    ))
}
fn read_array_4(bytes: &[u8], offset: usize) -> Result<[u8; 4], NeedleError> {
    Ok(bytes
        .get(offset..offset + 4)
        .ok_or_else(|| NeedleError::InvalidModel("unexpected end of file".to_string()))?
        .try_into()
        .expect("4-byte slice"))
}
fn read_array_32(bytes: &[u8], offset: usize) -> Result<[u8; 32], NeedleError> {
    Ok(bytes
        .get(offset..offset + 32)
        .ok_or_else(|| NeedleError::InvalidModel("unexpected end of file".to_string()))?
        .try_into()
        .expect("32-byte slice"))
}
fn section_payload<'a>(
    bytes: &'a [u8],
    sections: &BTreeMap<[u8; 4], (usize, usize)>,
    key: [u8; 4],
    label: &'static str,
) -> Result<&'a [u8], NeedleError> {
    let (offset, len) = sections
        .get(&key)
        .copied()
        .ok_or(NeedleError::MissingSection(label))?;
    Ok(&bytes[offset..offset + len])
}
fn section_payload_with_range<'a>(
    bytes: &'a [u8],
    sections: &BTreeMap<[u8; 4], (usize, usize)>,
    key: [u8; 4],
    label: &'static str,
) -> Result<(std::ops::Range<usize>, &'a [u8]), NeedleError> {
    let (offset, len) = sections
        .get(&key)
        .copied()
        .ok_or(NeedleError::MissingSection(label))?;
    Ok((offset..offset + len, &bytes[offset..offset + len]))
}
fn parse_tokenizer(bytes: &[u8]) -> Result<Vec<TokenizerPiece>, NeedleError> {
    let count = read_u32(bytes, 0)? as usize;
    let mut offset = 4;
    let mut pieces = Vec::with_capacity(count);
    for _ in 0..count {
        let piece_len = read_u32(bytes, offset)? as usize;
        offset += 4;
        let piece_end = offset + piece_len;
        let piece_bytes = bytes.get(offset..piece_end).ok_or_else(|| {
            NeedleError::InvalidModel("tokenizer piece out of bounds".to_string())
        })?;
        offset = piece_end;
        let piece_type = *bytes
            .get(offset)
            .ok_or_else(|| NeedleError::InvalidModel("tokenizer piece type missing".to_string()))?;
        offset += 1;
        let score_bits = read_u32(bytes, offset)?;
        offset += 4;
        pieces.push(TokenizerPiece {
            piece: String::from_utf8(piece_bytes.to_vec())
                .map_err(|exc| NeedleError::InvalidModel(format!("tokenizer piece utf8: {exc}")))?,
            piece_type,
            score: f32::from_bits(score_bits),
        });
    }
    if offset != bytes.len() {
        return Err(NeedleError::InvalidModel(
            "tokenizer section has trailing bytes".to_string(),
        ));
    }
    Ok(pieces)
}
fn parse_tensor_directory(
    bytes: &[u8],
    tensor_data_len: usize,
) -> Result<BTreeMap<String, TensorDescriptor>, NeedleError> {
    let count = read_u32(bytes, 0)? as usize;
    let mut offset = 4;
    let mut tensors = BTreeMap::new();
    for _ in 0..count {
        let name_len = read_u32(bytes, offset)? as usize;
        offset += 4;
        let name_end = offset + name_len;
        let name_bytes = bytes
            .get(offset..name_end)
            .ok_or_else(|| NeedleError::InvalidModel("tensor name out of bounds".to_string()))?;
        offset = name_end;
        let dtype = *bytes
            .get(offset)
            .ok_or_else(|| NeedleError::InvalidModel("tensor dtype missing".to_string()))?;
        offset += 1;
        let rank = usize::from(
            *bytes
                .get(offset)
                .ok_or_else(|| NeedleError::InvalidModel("tensor rank missing".to_string()))?,
        );
        offset += 1;
        let mut shape = Vec::with_capacity(rank);
        for _ in 0..rank {
            shape.push(read_u32(bytes, offset)?);
            offset += 4;
        }
        let data_offset = read_u64(bytes, offset)? as usize;
        offset += 8;
        let byte_len = read_u64(bytes, offset)? as usize;
        offset += 8;
        let sha256 = read_array_32(bytes, offset)?;
        offset += 32;
        let name = String::from_utf8(name_bytes.to_vec())
            .map_err(|exc| NeedleError::InvalidModel(format!("tensor name utf8: {exc}")))?;
        if dtype != DTYPE_BF16 {
            return Err(NeedleError::InvalidModel(format!(
                "tensor {name} has unsupported dtype tag {dtype}"
            )));
        }
        let expected_bytes = shape_elements(&shape)
            .checked_mul(2)
            .ok_or_else(|| NeedleError::InvalidModel(format!("tensor {name} size overflows")))?;
        if expected_bytes != byte_len {
            return Err(NeedleError::InvalidModel(format!(
                "tensor {name} byte_len {byte_len} != expected bf16 bytes {expected_bytes}"
            )));
        }
        if data_offset + byte_len > tensor_data_len {
            return Err(NeedleError::InvalidModel(format!(
                "tensor {name} range exceeds tensor data section"
            )));
        }
        tensors.insert(
            name.clone(),
            TensorDescriptor {
                name,
                dtype: TensorDType::Bf16,
                shape,
                data_offset,
                byte_len,
                sha256,
            },
        );
    }
    if offset != bytes.len() {
        return Err(NeedleError::InvalidModel(
            "tensor directory has trailing bytes".to_string(),
        ));
    }
    Ok(tensors)
}
fn validate_hash(expected_hex: &str, payload: &[u8], label: &str) -> Result<(), NeedleError> {
    let actual = hex_lower(&sha256_bytes(payload));
    if actual != expected_hex {
        return Err(NeedleError::InvalidModel(format!(
            "metadata hash mismatch for {label}: {expected_hex} != {actual}"
        )));
    }
    Ok(())
}
fn sha256_bytes(payload: &[u8]) -> [u8; 32] {
    Sha256::digest(payload).into()
}
#[allow(clippy::format_collect)]
fn hex_lower(bytes: &[u8]) -> String {
    bytes.iter().map(|byte| format!("{byte:02x}")).collect()
}
fn shape_elements(shape: &[u32]) -> usize {
    if shape.is_empty() {
        1
    } else {
        shape
            .iter()
            .fold(1_usize, |acc, &dim| acc.saturating_mul(dim as usize))
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use serde::Deserialize;

    #[test]
    fn tokenizer_matches_existing_parity_fixture() {
        #[derive(Deserialize)]
        struct Fixture {
            input: String,
            ids: Vec<u32>,
            pieces: Vec<String>,
            decoded: String,
        }
        #[derive(Deserialize)]
        struct DecodeOnly {
            ids: Vec<u32>,
            decoded: String,
        }
        #[derive(Deserialize)]
        struct Parity {
            fixtures: Vec<Fixture>,
            decode_only: Vec<DecodeOnly>,
        }
        let parity: Parity = serde_json::from_str(
            &std::fs::read_to_string(
                PathBuf::from(env!("CARGO_MANIFEST_DIR"))
                    .join("tests/fixtures/needle_sentencepiece_parity.json"),
            )
            .expect("fixture"),
        )
        .expect("json");
        let tok = NeedleTokenizer::from_repo_default().expect("tokenizer");
        for case in parity.fixtures {
            assert_eq!(
                tok.encode(&case.input).expect("encode"),
                case.ids,
                "input {}",
                case.input
            );
            assert_eq!(
                tok.encode_pieces(&case.input).expect("pieces"),
                case.pieces,
                "input {}",
                case.input
            );
            assert_eq!(
                tok.decode(&case.ids).expect("decode"),
                case.decoded,
                "input {}",
                case.input
            );
        }
        for case in parity.decode_only {
            assert_eq!(tok.decode(&case.ids).expect("decode only"), case.decoded);
        }
    }

    #[test]
    fn snake_case_roundtrip() {
        let src = r#"[{"name":"memorySearch","parameters":{"query":{"type":"string"}}}]"#;
        let (norm, map) = normalize_tools_json(src);
        assert!(norm.contains("memory_search"));
        let restored = restore_tool_names(
            r#"[{"name":"memory_search","arguments":{"query":"x"}}]"#,
            &map,
        );
        let left: serde_json::Value = serde_json::from_str(&restored).expect("restored json");
        let right: serde_json::Value =
            serde_json::from_str(r#"[{"name":"memorySearch","arguments":{"query":"x"}}]"#)
                .expect("expected json");
        assert_eq!(left, right);
    }

    #[test]
    fn rope_and_norm_ops_are_stable() {
        let rope = precompute_rope(4, 2, 10000.0);
        let mut x = vec![1.0, 2.0, 3.0, 4.0];
        apply_rope_position(&mut x, 1, 4, &rope, 1);
        assert!(x.iter().all(|v| v.is_finite()));
        let y = apply_zcrmsnorm_vec(&[1.0, -1.0, 2.0, -2.0], &[0.0; 4]);
        assert!(y.iter().all(|v| v.is_finite()));
    }
}
