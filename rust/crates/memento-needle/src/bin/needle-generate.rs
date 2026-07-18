use memento_needle::{GenerationOptions, Model, NeedleTokenizer, RouterModel};
use std::env;

fn main() -> Result<(), Box<dyn std::error::Error>> {
    let args = env::args().collect::<Vec<_>>();
    if args.len() != 5 {
        return Err("usage: needle-generate MODEL TOKENIZER QUERY TOOLS_JSON".into());
    }
    let model = Model::from_path(&args[1])?;
    let router = RouterModel::from_ndl(&model)?;
    let tokenizer = NeedleTokenizer::from_model_path(&args[2])?;
    let result = router.generate(
        &tokenizer,
        &args[3],
        &args[4],
        GenerationOptions {
            max_gen_len: 128,
            max_enc_len: 1024,
            constrained: true,
        },
        None,
    )?;
    println!("{result}");
    Ok(())
}
