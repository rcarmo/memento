use memento_gte::{BatchOptions, GteError, Model};
use serde::Deserialize;
use std::fs;
use std::path::PathBuf;

#[derive(Deserialize)]
struct Fixture {
    items: Vec<Item>,
}

#[derive(Deserialize)]
struct Item {
    text: String,
    tokens: Vec<u32>,
    embedding: Vec<f32>,
}

fn fixture_root() -> PathBuf {
    PathBuf::from(env!("CARGO_MANIFEST_DIR")).join("../../tests/fixtures")
}

fn model_path() -> PathBuf {
    PathBuf::from(env!("CARGO_MANIFEST_DIR")).join("../../../models/gte/gte-small.gtemodel")
}

#[test]
fn parses_fixture_model_and_matches_go_parity() {
    let root = fixture_root();
    let model_path = model_path();
    let fixture_path = root.join("go_parity.json");
    if !(model_path.exists() && fixture_path.exists()) {
        eprintln!("skipping parity test; run rust/tests/scripts/generate_golden.sh");
        return;
    }
    let model = Model::from_path(&model_path).expect("load model");
    let fixture: Fixture = serde_json::from_slice(&fs::read(fixture_path).expect("read fixture"))
        .expect("parse fixture");
    for item in fixture.items {
        assert_eq!(
            model.tokenize(&item.text),
            item.tokens,
            "token mismatch for {:?}",
            item.text
        );
        let got = model.embed(&item.text).expect("embed");
        assert_eq!(got.len(), item.embedding.len());
        for (index, (g, w)) in got.iter().zip(item.embedding.iter()).enumerate() {
            assert!(
                (g - w).abs() < 1e-4,
                "embedding mismatch for {:?} at {}: {} vs {}",
                item.text,
                index,
                g,
                w
            );
        }
    }
}

#[test]
fn batch_limits_and_cancellation_work() {
    let model_path = model_path();
    if !model_path.exists() {
        eprintln!("skipping fixture-dependent test; run rust/tests/scripts/generate_golden.sh");
        return;
    }
    let model = Model::from_path(&model_path).expect("load model");
    let texts = vec!["hello".to_string(), "world".to_string()];
    let err = model
        .embed_batch(
            &texts,
            BatchOptions {
                max_batch: Some(1),
                max_chars_per_input: None,
            },
            None,
        )
        .expect_err("max_batch error");
    assert!(matches!(err, GteError::BatchTooLarge(2)));

    let mut checkpoints = 0;
    let mut cp = |stage| {
        checkpoints += 1;
        if stage == "layer_done" && checkpoints > 2 {
            Err(GteError::Cancelled("test"))
        } else {
            Ok(())
        }
    };
    let err = model
        .embed_batch(&texts[..1], BatchOptions::default(), Some(&mut cp))
        .expect_err("cancelled");
    assert!(matches!(err, GteError::Cancelled("test")));
}
