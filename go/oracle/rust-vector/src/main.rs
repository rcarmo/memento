use serde_json::{json, Value};

fn main() {
    let mut out: Vec<Value> = Vec::new();
    for bytes in [vec![], vec![1], vec![0, 0, 128, 63], f32::NAN.to_le_bytes().to_vec(), f32::INFINITY.to_le_bytes().to_vec()] {
        let result = memento_vector::validate_f32le(&bytes);
        out.push(json!({"method":"validate", "bytes":bytes, "dimensions":result.as_ref().ok(), "error":result.err().map(|e|e.to_string())}));
    }
    for (left,right) in [(vec![],vec![]),(vec![1.,2.,3.],vec![3.,2.,1.]),(vec![1.,0.],vec![0.,1.]),(vec![1.,2.],vec![1.]),(vec![0.,0.],vec![1.,1.]),(vec![1.,-2.,0.5],vec![-3.,0.5,4.])] {
        for method in ["dot","cosine"] {
            let result = if method=="dot" {memento_vector::dot(&left,&right)} else {memento_vector::cosine(&left,&right)};
            out.push(json!({"method":method,"left":left,"right":right,"value":result.as_ref().ok(),"error":result.err().map(|e|e.to_string())}));
        }
        let mut output=right.clone();
        let result=memento_vector::axpy(0.5,&left,&mut output);
        out.push(json!({"method":"axpy","alpha":0.5,"left":left,"right":right,"output":output,"error":result.err().map(|e|e.to_string())}));
    }
    println!("{}",serde_json::to_string_pretty(&out).unwrap());
}
