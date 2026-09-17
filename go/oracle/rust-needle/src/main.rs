use serde_json::json;
use sha2::{Digest, Sha256};
fn hex(bytes: &[u8]) -> String { bytes.iter().map(|b| format!("{b:02x}")).collect() }
fn main() {
    let path=std::env::args().nth(1).expect("model path required");
    let bytes=std::fs::read(path).unwrap();
    let model=memento_needle::Model::from_bytes(&bytes).unwrap();
    let mut tensors=Vec::new();
    for name in model.tensor_names() {
        let t=model.tensor(name).unwrap();
        let floats=t.to_f32_vec();
        let encoded:Vec<u8>=floats.iter().flat_map(|v|v.to_le_bytes()).collect();
        tensors.push(json!({"name":name,"shape":t.shape(),"elements":t.element_count(),"raw_sha256":hex(&Sha256::digest(t.raw_bf16())),"float32_sha256":hex(&Sha256::digest(&encoded))}));
    }
    let pieces:Vec<_>=model.tokenizer_pieces().iter().map(|p|json!({"piece":p.piece,"piece_type":p.piece_type,"score":p.score})).collect();
    println!("{}",serde_json::to_string_pretty(&json!({"model_sha256":hex(&Sha256::digest(&bytes)),"config":model.config(),"metadata":model.metadata(),"pieces":pieces,"tensors":tensors})).unwrap());
}
