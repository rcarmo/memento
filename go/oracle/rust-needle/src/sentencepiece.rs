use serde_json::{json, Value};
use sentencepiece_rust::SentencePieceProcessor;
fn var(mut n:u64)->Vec<u8>{let mut out=Vec::new();while n>=128{out.push((n as u8)|128);n>>=7;}out.push(n as u8);out}
fn number(n:u64,v:u64)->Vec<u8>{let mut out=var(n<<3);out.extend(var(v));out}
fn field(n:u64,b:&[u8])->Vec<u8>{let mut out=var(n<<3|2);out.extend(var(b.len()as u64));out.extend(b);out}
pub fn fixtures()->Value{
 let mut cases=Vec::new();
 let pieces=[("<unk>",0.0f32,2),("<s>",0.0,3),("</s>",0.0,3),("▁",-1.0,1),("a",-1.0,1),("b",-2.0,1),("ab",1.0,1),("▁ab",2.0,1),("aa",4.0,1),("aaa",3.0,1),("ba",4.0,1),("aba",8.0,1),("ababa",9.0,1),("custom",2.0,4),("<0xC3>",0.0,6),("<0xA9>",0.0,6)];
 for kind in [1u64,2] { for fallback in [false,true] {for flags in [0u64,1,2,3,4,5,6,7,8] {
  let mut bytes=Vec::new();
  for (text,score,typ) in pieces{let mut p=field(1,text.as_bytes());p.push(2<<3|5);p.extend(score.to_le_bytes());p.extend(number(3,typ));bytes.extend(field(1,&p));}
  let mut trainer=number(3,kind);trainer.extend(number(35,u64::from(fallback)));trainer.extend(number(24,if flags==8{1}else{0}));bytes.extend(field(2,&trainer));
  let mut norm=number(3,flags&1);norm.extend(number(4,(flags>>1)&1));norm.extend(number(5,(flags>>2)&1));bytes.extend(field(3,&norm));
  let model=SentencePieceProcessor::from_bytes(&bytes).unwrap();
  for text in ["", "ab", " aaab abababa ","custom","zz☃","é", "  a  b  ","\t\na\r"] {
   let ids=model.encode(text).unwrap();let decoded=model.decode(&ids).unwrap();
   cases.push(json!({"model_bytes":bytes,"text":text,"ids":ids,"decoded":decoded}));
  }
 }}}
 json!(cases)
}
