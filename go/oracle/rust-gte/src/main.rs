use serde_json::json;

fn model_bytes(layers: u32, max_sequence: u32) -> (Vec<String>, Vec<u8>) {
    let mut vocab: Vec<String> = (0..110).map(|i| format!("t{i}")).collect();
    for (i, w) in ["hello", "world", "!", "##s", "界", "##界"].iter().enumerate() {
        vocab[104 + i] = (*w).to_string();
    }
    let mut bytes = b"GTE1".to_vec();
    for value in [110u32, 2, layers, 1, 3, max_sequence] {
        bytes.extend(value.to_le_bytes());
    }
    for word in &vocab {
        bytes.extend((word.len() as u16).to_le_bytes());
        bytes.extend(word.as_bytes());
    }
    let floats = 110 * 2 + max_sequence as usize * 2 + 2 * 2 + 2 + 2
        + layers as usize * (4 * (2 * 2 + 2) + 2 * (2 + 2) + 3 * 2 + 3 + 2 * 3 + 2)
        + 2 * 2 + 2;
    for i in 0..floats {
        bytes.extend(((i % 7) as f32 / 10.0).to_le_bytes());
    }
    (vocab, bytes)
}

fn main() {
    let mode = std::env::args().nth(1).unwrap_or_else(|| "tokenizer".into());
    let mut cases = Vec::new();
    if mode == "tokenizer" {
        for max_sequence in [2u32, 3, 4, 5, 16] {
            let (vocab, bytes) = model_bytes(0, max_sequence);
            let model = memento_gte::Model::from_bytes(&bytes).unwrap();
            for text in ["", " \t\n\r ", "HELLO world!", "hellos", "界界", "☃", "☃☃", "world☃", "hello unknown world", "[HELLO]:world!{x}|~", "a\0b", "é", "HELLO WORLD HELLO WORLD"] {
                cases.push(json!({"vocab":vocab,"max_sequence":max_sequence,"text":text,"tokens":model.tokenize(text)}));
            }
        }
    } else if mode == "inference" {
        for layers in [0u32, 1, 2] {
            let (_, bytes) = model_bytes(layers, 8);
            let model = memento_gte::Model::from_bytes(&bytes).unwrap();
            for texts in [vec![""], vec!["hello"], vec!["HELLO world!", "界", "☃☃", ""], vec!["hello world hello world", "hellos"]] {
                let texts: Vec<String> = texts.into_iter().map(str::to_string).collect();
                let mut checkpoints = Vec::new();
                let mut cp = |label: &'static str| { checkpoints.push(label); Ok(()) };
                let outputs = model.embed_batch(&texts, memento_gte::BatchOptions::default(), Some(&mut cp)).unwrap();
                cases.push(json!({"layers":layers,"texts":texts,"outputs":outputs,"checkpoints":checkpoints}));
            }
        }
    } else if mode == "real" {
        let path = std::env::args().nth(2).expect("real mode requires model path");
        let model = memento_gte::Model::from_path(path).unwrap();
        let texts: Vec<String> = ["", "hello", "Where is the service configuration?", "Unicode: café 界 ☃", "Keep the original proposal and its assets."]
            .into_iter().map(str::to_string).collect();
        let outputs = model.embed_batch(&texts, memento_gte::BatchOptions::default(), None).unwrap();
        let tokens: Vec<Vec<u32>> = texts.iter().map(|s| model.tokenize(s)).collect();
        cases.push(json!({"model_sha256":"06d049fc4f67208665b05d840cc307c04d46770654a8fe25afb040f360abf171", "texts":texts,"tokens":tokens,"outputs":outputs}));
    } else { panic!("unsupported oracle mode"); }
    println!("{}", serde_json::to_string_pretty(&cases).unwrap());
}
