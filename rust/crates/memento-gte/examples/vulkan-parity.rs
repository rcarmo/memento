#[cfg(feature = "vulkan")]
fn main() -> Result<(), Box<dyn std::error::Error>> {
    use memento_gte::{vulkan::Vulkan, BatchOptions, Model};
    use std::time::Instant;
    let args: Vec<String> = std::env::args().collect();
    let path = args
        .get(1)
        .ok_or("usage: vulkan-parity MODEL [DEVICE_SUBSTRING]")?;
    let start = Instant::now();
    let gpu = Vulkan::new(args.get(2).map(String::as_str))?;
    eprintln!(
        "adapter={} init_ms={}",
        serde_json::to_string(&gpu.info)?,
        start.elapsed().as_millis()
    );
    let model = Model::from_path(path)?;
    let cases: Vec<Vec<String>> = vec![
        vec!["Shared durable memory and reviewed proposals.".into()],
        vec![
            String::new(),
            "The network switch is in the rack.".into(),
            "Olá mundo! 東京 — café.".into(),
        ],
        vec!["Embedding inference uses attention and matrix multiplication. ".repeat(45)],
        vec!["Memory namespace proposal review asset archive. ".repeat(180)],
    ];
    for (case, texts) in cases.iter().enumerate() {
        let t = Instant::now();
        let cpu = model.embed_batch(texts, BatchOptions::default(), None)?;
        let cpu_ms = t.elapsed().as_secs_f64() * 1000.0;
        let t = Instant::now();
        let vk = model.embed_batch_vulkan(texts, BatchOptions::default(), &gpu)?;
        let vk_ms = t.elapsed().as_secs_f64() * 1000.0;
        let mut minimum_cosine = 1.0f64;
        let mut maximum_error = 0.0f32;
        for (a, b) in cpu.iter().zip(&vk) {
            let mut dot = 0.0f64;
            let mut an = 0.0f64;
            let mut bn = 0.0f64;
            for (x, y) in a.iter().zip(b) {
                dot += f64::from(*x) * f64::from(*y);
                an += f64::from(*x).powi(2);
                bn += f64::from(*y).powi(2);
                maximum_error = maximum_error.max((x - y).abs());
            }
            minimum_cosine = minimum_cosine.min(dot / (an * bn).sqrt());
        }
        println!(
            "{}",
            serde_json::json!({"case":case,"batch":texts.len(),"cpu_ms":cpu_ms,"vulkan_ms":vk_ms,"cosine":minimum_cosine,"max_abs_error":maximum_error,"device":gpu.info})
        );
        if minimum_cosine < 0.99999 || maximum_error > 0.001 {
            return Err("Vulkan parity failed".into());
        }
    }
    Ok(())
}
#[cfg(not(feature = "vulkan"))]
fn main() {
    eprintln!("build with --features vulkan");
    std::process::exit(1);
}
