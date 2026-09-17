use memento_embed::{handle_request, read_request, write_frame, Request};
use memento_gte::Model;
use memento_vector::decode_f32le;
use std::io::Cursor;
use std::path::PathBuf;

fn fixture_model() -> Option<Model> {
    let path =
        PathBuf::from(env!("CARGO_MANIFEST_DIR")).join("../../../models/gte/gte-small.gtemodel");
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

#[test]
fn rejects_oversized_input_frame() {
    let frame = (4_u32 * 1024 * 1024 + 1).to_le_bytes();
    assert!(matches!(
        read_request(&frame[..]),
        Err(memento_embed::ProtocolError::FrameTooLarge(_))
    ));
}

#[test]
fn validates_backend_names() {
    use memento_embed::backend::Backend;
    assert_eq!("cpu".parse::<Backend>().unwrap(), Backend::Cpu);
    assert_eq!("auto".parse::<Backend>().unwrap(), Backend::Auto);
    assert!("cuda".parse::<Backend>().is_err());
}

#[test]
fn unavailable_backend_explicit_error_or_auto_cpu() {
    use memento_embed::backend::{Backend, Engine};
    let Some(model) = fixture_model() else {
        return;
    };
    assert!(Engine::new(
        model.clone(),
        Backend::Vulkan,
        Some("NONEXISTENT-GPU-MEMENTO")
    )
    .is_err());
    let engine = Engine::new(model, Backend::Auto, Some("NONEXISTENT-GPU-MEMENTO")).unwrap();
    assert_eq!(engine.info.selected, "cpu");
    assert!(engine.info.fallback_reason.is_some());
}

#[cfg(feature = "vulkan")]
#[test]
#[ignore = "requires actual hardware Vulkan and the pinned GTE1 model"]
fn hardware_same_model_parity_and_diagnostics() {
    use memento_embed::backend::{Backend, Engine};
    let model = fixture_model().expect("hardware test requires the real GTE1 model");
    let texts = vec![
        String::new(),
        "Review the shared project memory.".to_string(),
        "Olá 東京 café".to_string(),
    ];
    let reference = model
        .embed_batch(&texts, memento_gte::BatchOptions::default(), None)
        .unwrap();
    let mut engine = Engine::new(model, Backend::Vulkan, None).unwrap();
    assert_eq!(engine.info.selected, "vulkan");
    for _ in 0..2 {
        let got = engine.embed(&texts).unwrap();
        for (a, b) in reference.iter().zip(got) {
            assert!(a.iter().zip(b).all(|(x, y)| (x - y).abs() < 0.001));
        }
    }
}
