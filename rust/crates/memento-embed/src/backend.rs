//! Backend selection is local to the bounded embedding subprocess. CPU stays default.
use memento_gte::{BatchOptions, GteError, Model};
use serde::Serialize;

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Backend {
    Cpu,
    Vulkan,
    Auto,
}
impl std::str::FromStr for Backend {
    type Err = String;
    fn from_str(s: &str) -> Result<Self, Self::Err> {
        match s {
            "cpu" => Ok(Self::Cpu),
            "vulkan" => Ok(Self::Vulkan),
            "auto" => Ok(Self::Auto),
            _ => Err("backend must be cpu, vulkan or auto".into()),
        }
    }
}
#[derive(Debug, Clone, Serialize, serde::Deserialize)]
pub struct BackendInfo {
    pub requested: String,
    pub selected: String,
    pub device: Option<String>,
    pub fallback_reason: Option<String>,
}

pub struct Engine {
    pub model: Model,
    mode: Backend,
    pub info: BackendInfo,
    #[cfg(feature = "vulkan")]
    gpu: Option<memento_gte::vulkan::Vulkan>,
}
impl Engine {
    pub fn new(model: Model, mode: Backend, selector: Option<&str>) -> Result<Self, GteError> {
        let info = BackendInfo {
            requested: format!("{mode:?}").to_lowercase(),
            selected: "cpu".into(),
            device: None,
            fallback_reason: None,
        };
        let mut engine = Self {
            model,
            mode,
            info,
            #[cfg(feature = "vulkan")]
            gpu: None,
        };
        if mode != Backend::Cpu {
            #[cfg(feature = "vulkan")]
            let probe = memento_gte::vulkan::Vulkan::new(selector);
            #[cfg(not(feature = "vulkan"))]
            let probe: Result<(), GteError> = {
                let _ = selector;
                Err(GteError::Backend(
                    "binary built without Vulkan feature".into(),
                ))
            };
            match probe {
                #[cfg(feature = "vulkan")]
                Ok(gpu) => {
                    engine.info.selected = "vulkan".into();
                    engine.info.device = Some(gpu.info.name.clone());
                    let probe_text = vec!["Memento Vulkan embedding self-test.".to_string()];
                    let reference =
                        engine
                            .model
                            .embed_batch(&probe_text, BatchOptions::default(), None)?;
                    let check = engine
                        .model
                        .embed_batch_vulkan(&probe_text, BatchOptions::default(), &gpu)
                        .and_then(|v| check_parity(&reference, &v, engine.model.dim()));
                    match check {
                        Ok(()) => engine.gpu = Some(gpu),
                        Err(e) if mode == Backend::Auto => {
                            engine.info.selected = "cpu".into();
                            engine.info.fallback_reason = Some(e.to_string());
                        }
                        Err(e) => return Err(e),
                    }
                }
                #[cfg(not(feature = "vulkan"))]
                Ok(()) => unreachable!(),
                Err(e) if mode == Backend::Auto => {
                    engine.info.fallback_reason = Some(e.to_string());
                }
                Err(e) => return Err(e),
            }
        }
        Ok(engine)
    }
    pub fn embed(&mut self, texts: &[String]) -> Result<Vec<Vec<f32>>, GteError> {
        let options = BatchOptions {
            max_batch: Some(16),
            max_chars_per_input: Some(65536),
        };
        #[cfg(feature = "vulkan")]
        if let Some(gpu) = &self.gpu {
            match self.model.embed_batch_vulkan(texts, options, gpu) {
                Ok(v) => return Ok(v),
                Err(e) if self.mode == Backend::Auto => {
                    // Drop all partial output; replay this whole read-only batch on CPU.
                    self.info.fallback_reason = Some(e.to_string());
                    self.info.selected = "cpu".into();
                    self.gpu = None;
                }
                Err(e) => return Err(e),
            }
        }
        #[cfg(not(feature = "vulkan"))]
        let _ = self.mode;
        self.model.embed_batch(texts, options, None)
    }
}

#[cfg(any(feature = "vulkan", test))]
fn check_parity(
    reference: &[Vec<f32>],
    candidate: &[Vec<f32>],
    dimensions: usize,
) -> Result<(), GteError> {
    if reference.len() != 1
        || candidate.len() != 1
        || reference[0].len() != dimensions
        || candidate[0].len() != dimensions
    {
        return Err(GteError::Backend("Vulkan self-test shape mismatch".into()));
    }
    if reference[0]
        .iter()
        .zip(&candidate[0])
        .any(|(a, b)| !a.is_finite() || !b.is_finite() || (a - b).abs() > 0.001)
    {
        return Err(GteError::Backend("Vulkan self-test mismatch".into()));
    }
    Ok(())
}
#[cfg(test)]
mod tests {
    use super::check_parity;
    #[test]
    fn parity_rejects_shape_and_nonfinite_outputs() {
        let reference = vec![vec![1.0, 0.0]];
        for candidate in [
            vec![],
            vec![vec![]],
            vec![vec![1.0]],
            vec![vec![1.0, 0.0, 0.0]],
            vec![vec![f32::NAN, 0.0]],
        ] {
            assert!(check_parity(&reference, &candidate, 2).is_err());
        }
        assert!(check_parity(&reference, &reference, 2).is_ok());
    }
}
