mod sentencepiece;
use serde_json::json;
use sha2::{Digest, Sha256};
fn hex(bytes: &[u8]) -> String { bytes.iter().map(|b| format!("{b:02x}")).collect() }
fn main() {
    let path=std::env::args().nth(1).expect("model path required");
    if path == "synthetic-tokenizers" { println!("{}", serde_json::to_string_pretty(&sentencepiece::fixtures()).unwrap()); return; }
    let bytes=std::fs::read(path).unwrap();
    if std::env::args().nth(2).as_deref() == Some("tokenizer") {
        let tokenizer=memento_needle::NeedleTokenizer::from_model_bytes(&bytes).unwrap();
        let mut cases=Vec::new();
        for text in ["", "hello", " hello world  ", "Find project notes", "café 界 ☃ 😀", "{\"name\":\"memory_search\",\"arguments\":{}}", "<tools>[]<tool_call>", "before<tools>after", "<tool_call>word", "<tools> hello", "<tools><tool_call>", "\t\nhello\r", " \u{00a0}a\u{200b}b", "..........----", "<tool_call>界"] {
            let ids=tokenizer.encode(text).unwrap();
            let decoded=tokenizer.decode(&ids).unwrap();
            cases.push(json!({"text":text,"ids":ids,"decoded":decoded}));
        }
        let strings:Vec<String>=(0..tokenizer.vocab_size()).map(|i|tokenizer.decode(&[i as u32]).unwrap()).collect();
        println!("{}",serde_json::to_string_pretty(&json!({"model_sha256":hex(&Sha256::digest(&bytes)),"cases":cases,"decoded_tokens":strings})).unwrap());
        return;
    }
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
