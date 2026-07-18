use memento_needle::Model;
use std::path::PathBuf;

fn model_path() -> PathBuf {
    PathBuf::from(env!("CARGO_MANIFEST_DIR")).join("../../../models/needle/memento-router.ndl")
}

#[test]
fn loads_converted_router_model_when_present() {
    let path = model_path();
    if !path.exists() {
        eprintln!("skipping model load test; generate or fetch models/needle/memento-router.ndl");
        return;
    }
    let bytes = std::fs::read(&path).expect("read ndl");
    if bytes.starts_with(b"version https://git-lfs.github.com/spec/v1\n") {
        eprintln!("skipping model load test; git lfs pull models/needle/memento-router.ndl");
        return;
    }
    let model = Model::from_bytes(&bytes).expect("parse ndl");
    assert_eq!(model.config().dtype, "bfloat16");
    assert_eq!(model.config().num_encoder_layers, 12);
    assert_eq!(model.metadata().tensor_count, 31);
    assert_eq!(model.tokenizer_pieces().len(), 8192);
    let tensor = model
        .tensor("embedding.embedding")
        .expect("embedding tensor");
    assert_eq!(tensor.shape(), &[8192, 512]);
    assert_eq!(tensor.raw_bf16().len(), 8192 * 512 * 2);
}
