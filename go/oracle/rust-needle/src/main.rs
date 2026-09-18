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
    if std::env::args().nth(2).as_deref() == Some("corpus") {
        let tokenizer_path=std::env::args().nth(3).expect("tokenizer required");
        let corpus_path=std::env::args().nth(4).expect("corpus required");
        let corpus=std::fs::read(corpus_path).unwrap();
        assert_eq!(hex(&Sha256::digest(&corpus)),"9ffeb303574fa6bd24718adc42c7a3d8c4632e3cf78685d14886c7b24b2ddca9");
        let tokenizer=memento_needle::NeedleTokenizer::from_model_path(tokenizer_path).unwrap();
        let router=memento_needle::RouterModel::from_ndl(&model).unwrap();
        let mut cases=Vec::new();
        let mut tools=String::new();
        for line in String::from_utf8(corpus).unwrap().lines() {
            let row:serde_json::Value=serde_json::from_str(line).unwrap();
            let query=row["query"].as_str().unwrap();let contract=row["tools"].as_str().unwrap();
            if tools.is_empty(){tools=contract.to_string();}assert_eq!(tools,contract);
            let result=router.generate(&tokenizer,query,contract,memento_needle::GenerationOptions{max_gen_len:128,max_enc_len:1024,constrained:true},None);
            cases.push(match result {Ok(output)=>json!({"query":query,"output":output,"error":null}),Err(error)=>json!({"query":query,"output":null,"error":error.to_string()})});
            if cases.len()%30==0{eprintln!("oracle completed {} cases",cases.len());}
        }
        println!("{}",serde_json::to_string_pretty(&json!({"model_sha256":hex(&Sha256::digest(&bytes)),"corpus_sha256":"9ffeb303574fa6bd24718adc42c7a3d8c4632e3cf78685d14886c7b24b2ddca9","tools_json":tools,"cases":cases})).unwrap());
        return;
    }
    if std::env::args().nth(2).as_deref() == Some("generate") {
        let tokenizer_path=std::env::args().nth(3).expect("tokenizer required");
        let tools_path=std::env::args().nth(4).expect("tool contract required");
        let tools=std::fs::read_to_string(tools_path).unwrap();
        let tokenizer=memento_needle::NeedleTokenizer::from_model_path(tokenizer_path).unwrap();
        let router=memento_needle::RouterModel::from_ndl(&model).unwrap();
        let mut cases=Vec::new();
        for query in ["find Piclaw", "What is the current repository revision?", "show search paths for deployment", "read the title of /projects/piclaw.md", "What is the weather tomorrow?"] {
            let mut checkpoints=Vec::new();let mut cp=|label: &'static str| {checkpoints.push(label);Ok(())};
            let output=router.generate(&tokenizer,query,&tools,memento_needle::GenerationOptions::default(),Some(&mut cp)).unwrap();
            cases.push(json!({"query":query,"output":output,"checkpoints":checkpoints}));
        }
        println!("{}",serde_json::to_string_pretty(&json!({"model_sha256":hex(&Sha256::digest(&bytes)),"tools_json":tools,"cases":cases})).unwrap());
        return;
    }
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
