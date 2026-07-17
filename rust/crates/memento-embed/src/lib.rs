//! Persistent framed embedding protocol with f32le payloads.
//! Adapted for Memento from the MIT-licensed `/tmp/go-gte` reference model format.

use memento_gte::{BatchOptions, GteError, Model};
use memento_vector::encode_f32le;
use serde::{Deserialize, Serialize};
use std::io::{Read, Write};
use thiserror::Error;

#[derive(Debug, Error)]
pub enum ProtocolError {
    #[error("io error: {0}")]
    Io(#[from] std::io::Error),
    #[error("json error: {0}")]
    Json(#[from] serde_json::Error),
    #[error("gte error: {0}")]
    Gte(#[from] GteError),
    #[error("frame too large: {0}")]
    FrameTooLarge(usize),
    #[error("invalid method: {0}")]
    InvalidMethod(String),
}

#[derive(Debug, Serialize, Deserialize)]
#[serde(tag = "method", rename_all = "snake_case")]
pub enum Request {
    Info {
        id: Option<String>,
    },
    Embed {
        id: Option<String>,
        text: String,
    },
    EmbedBatch {
        id: Option<String>,
        texts: Vec<String>,
    },
}

#[derive(Debug, Serialize, Deserialize)]
pub struct ResponseHeader {
    pub id: Option<String>,
    pub ok: bool,
    pub method: String,
    pub dimensions: Option<usize>,
    pub count: Option<usize>,
    pub payload_len: usize,
    pub error: Option<String>,
}

#[derive(Debug)]
pub struct ResponseFrame {
    pub header: ResponseHeader,
    pub payload: Vec<u8>,
}

pub fn write_frame(mut writer: impl Write, frame: &ResponseFrame) -> Result<(), ProtocolError> {
    let header = serde_json::to_vec(&frame.header)?;
    let total_len = 4 + header.len() + frame.payload.len();
    if total_len > u32::MAX as usize {
        return Err(ProtocolError::FrameTooLarge(total_len));
    }
    writer.write_all(&(total_len as u32).to_le_bytes())?;
    writer.write_all(&(header.len() as u32).to_le_bytes())?;
    writer.write_all(&header)?;
    writer.write_all(&frame.payload)?;
    writer.flush()?;
    Ok(())
}

pub fn read_request(mut reader: impl Read) -> Result<Request, ProtocolError> {
    let mut len_buf = [0_u8; 4];
    reader.read_exact(&mut len_buf)?;
    let len = u32::from_le_bytes(len_buf) as usize;
    let mut data = vec![0_u8; len];
    reader.read_exact(&mut data)?;
    Ok(serde_json::from_slice(&data)?)
}

pub fn handle_request(model: &Model, request: Request) -> Result<ResponseFrame, ProtocolError> {
    match request {
        Request::Info { id } => Ok(ResponseFrame {
            header: ResponseHeader {
                id,
                ok: true,
                method: "info".into(),
                dimensions: Some(model.dim()),
                count: Some(0),
                payload_len: 0,
                error: None,
            },
            payload: vec![],
        }),
        Request::Embed { id, text } => {
            let embedding = model.embed(&text)?;
            let payload = encode_f32le(&embedding).expect("finite model output");
            Ok(ResponseFrame {
                header: ResponseHeader {
                    id,
                    ok: true,
                    method: "embed".into(),
                    dimensions: Some(model.dim()),
                    count: Some(1),
                    payload_len: payload.len(),
                    error: None,
                },
                payload,
            })
        }
        Request::EmbedBatch { id, texts } => {
            let embeddings = model.embed_batch(&texts, BatchOptions::default(), None)?;
            let mut payload = Vec::with_capacity(embeddings.len() * model.dim() * 4);
            for embedding in embeddings {
                payload.extend_from_slice(&encode_f32le(&embedding).expect("finite model output"));
            }
            Ok(ResponseFrame {
                header: ResponseHeader {
                    id,
                    ok: true,
                    method: "embed_batch".into(),
                    dimensions: Some(model.dim()),
                    count: Some(texts.len()),
                    payload_len: payload.len(),
                    error: None,
                },
                payload,
            })
        }
    }
}
