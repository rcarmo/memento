use memento_needle::{GenerationOptions, Model, NeedleTokenizer, RouterModel};
use serde::{Deserialize, Serialize};
use std::env;
use std::io::{self, BufRead};
use std::time::Instant;

#[derive(Deserialize)]
struct Input {
    query: String,
    tools: String,
}

#[derive(Serialize)]
struct Output {
    output: String,
    latency_ms: f64,
}

fn main() -> Result<(), Box<dyn std::error::Error>> {
    let args = env::args().collect::<Vec<_>>();
    if args.len() != 3 {
        return Err("usage: needle-eval MODEL TOKENIZER".into());
    }
    let model = Model::from_path(&args[1])?;
    let router = RouterModel::from_ndl(&model)?;
    let tokenizer = NeedleTokenizer::from_model_path(&args[2])?;
    for line in io::stdin().lock().lines() {
        let input: Input = serde_json::from_str(&line?)?;
        let started = Instant::now();
        let output = router.generate(
            &tokenizer,
            &input.query,
            &input.tools,
            GenerationOptions {
                max_gen_len: 128,
                max_enc_len: 1024,
                constrained: true,
            },
            None,
        )?;
        println!(
            "{}",
            serde_json::to_string(&Output {
                output,
                latency_ms: started.elapsed().as_secs_f64() * 1000.0,
            })?
        );
    }
    Ok(())
}
