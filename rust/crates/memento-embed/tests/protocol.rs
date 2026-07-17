use memento_embed::{handle_request, read_request, write_frame, Request};
use memento_gte::Model;
use memento_vector::decode_f32le;
use std::io::Cursor;
use std::path::PathBuf;

fn fixture_model() -> Option<Model> {
    let path =
        PathBuf::from(env!("CARGO_MANIFEST_DIR")).join("../../tests/fixtures/gte-small.gtemodel");
    Model::from_path(path).ok()
}

#[test]
fn reads_and_writes_frames() {
    let request = Request::Info {
        id: Some("1".into()),
    };
    let encoded = serde_json::to_vec(&request).expect("json");
    let mut bytes = Vec::new();
    bytes.extend_from_slice(&(encoded.len() as u32).to_le_bytes());
    bytes.extend_from_slice(&encoded);
    let decoded = read_request(Cursor::new(bytes)).expect("read request");
    match decoded {
        Request::Info { id } => assert_eq!(id.as_deref(), Some("1")),
        _ => panic!("unexpected request"),
    }
}

#[test]
fn embed_response_contains_f32le_payload() {
    let Some(model) = fixture_model() else {
        eprintln!(
            "skipping fixture-dependent protocol test; run rust/tests/scripts/generate_golden.sh"
        );
        return;
    };
    let frame = handle_request(
        &model,
        Request::Embed {
            id: Some("e1".into()),
            text: "Hello world".into(),
        },
    )
    .expect("handle request");
    assert!(frame.header.ok);
    assert_eq!(frame.header.dimensions, Some(model.dim()));
    let decoded = decode_f32le(&frame.payload).expect("decode payload");
    assert_eq!(decoded.len(), model.dim());

    let mut wire = Vec::new();
    write_frame(&mut wire, &frame).expect("write frame");
    assert!(wire.len() > frame.payload.len());
}
