//! Optional Vulkan-only FP32 backend. Uses the original GTE1 weights and CPU
//! tokenisation/pooling; no model conversion or low-precision arithmetic.
use crate::{GteError, Model};
use std::collections::HashMap;
use wgpu::util::DeviceExt;

#[derive(Debug, Clone, serde::Serialize)]
pub struct DeviceInfo {
    pub name: String,
    pub vendor: u32,
    pub device: u32,
    pub device_type: String,
    pub driver: String,
    pub driver_info: String,
}

pub struct Vulkan {
    device: wgpu::Device,
    queue: wgpu::Queue,
    layout: wgpu::BindGroupLayout,
    pipelines: HashMap<&'static str, wgpu::ComputePipeline>,
    pub info: DeviceInfo,
    max_buffer: u64,
}
#[allow(clippy::needless_pass_by_value)] // Accept owned wgpu errors in map_err.
fn error(message: impl ToString) -> GteError {
    GteError::Backend(message.to_string())
}

impl Vulkan {
    /// Hardware adapters only. No GL/CPU fallback and no optional shader features.
    #[allow(clippy::too_many_lines)] // One explicit adapter/pipeline initialisation boundary.
    pub fn new(selector: Option<&str>) -> Result<Self, GteError> {
        let instance = wgpu::Instance::new(&wgpu::InstanceDescriptor {
            backends: wgpu::Backends::VULKAN,
            ..Default::default()
        });
        let adapter = instance
            .enumerate_adapters(wgpu::Backends::VULKAN)
            .into_iter()
            .find(|a| {
                let i = a.get_info();
                matches!(
                    i.device_type,
                    wgpu::DeviceType::IntegratedGpu | wgpu::DeviceType::DiscreteGpu
                ) && selector.is_none_or(|s| i.name.to_lowercase().contains(&s.to_lowercase()))
            })
            .ok_or_else(|| {
                error("no matching hardware Vulkan adapter; software adapters are excluded")
            })?;
        let info = adapter.get_info();
        let limits = adapter.limits();
        if limits.max_storage_buffers_per_shader_stage < 4
            || limits.max_compute_invocations_per_workgroup < 64
        {
            return Err(error("Vulkan adapter lacks baseline compute limits"));
        }
        let max_buffer =
            u64::from(limits.max_storage_buffer_binding_size).min(limits.max_buffer_size);
        let required = wgpu::Limits::downlevel_defaults().using_resolution(limits.clone());
        let max_buffer = max_buffer
            .min(u64::from(required.max_storage_buffer_binding_size))
            .min(required.max_buffer_size);
        let (device, queue) = pollster::block_on(adapter.request_device(
            &wgpu::DeviceDescriptor {
                label: Some("memento-gte-vulkan"),
                required_features: wgpu::Features::empty(),
                required_limits: required,
                memory_hints: wgpu::MemoryHints::MemoryUsage,
            },
            None,
        ))
        .map_err(error)?;
        device.push_error_scope(wgpu::ErrorFilter::Validation);
        let entries: Vec<_> = (0..5)
            .map(|binding| wgpu::BindGroupLayoutEntry {
                binding,
                visibility: wgpu::ShaderStages::COMPUTE,
                ty: wgpu::BindingType::Buffer {
                    ty: if binding == 4 {
                        wgpu::BufferBindingType::Uniform
                    } else {
                        wgpu::BufferBindingType::Storage {
                            read_only: binding != 3,
                        }
                    },
                    has_dynamic_offset: false,
                    min_binding_size: None,
                },
                count: None,
            })
            .collect();
        let layout = device.create_bind_group_layout(&wgpu::BindGroupLayoutDescriptor {
            label: Some("gte-fp32"),
            entries: &entries,
        });
        let pl = device.create_pipeline_layout(&wgpu::PipelineLayoutDescriptor {
            label: None,
            bind_group_layouts: &[&layout],
            push_constant_ranges: &[],
        });
        let shader = device.create_shader_module(wgpu::ShaderModuleDescriptor {
            label: Some("gte-fp32"),
            source: wgpu::ShaderSource::Wgsl(include_str!("vulkan.wgsl").into()),
        });
        let mut pipelines = HashMap::new();
        for entry in [
            "linear", "norm", "residual", "gelu", "scores", "softmax", "values",
        ] {
            pipelines.insert(
                entry,
                device.create_compute_pipeline(&wgpu::ComputePipelineDescriptor {
                    label: Some(entry),
                    layout: Some(&pl),
                    module: &shader,
                    entry_point: Some(entry),
                    compilation_options: wgpu::PipelineCompilationOptions::default(),
                    cache: None,
                }),
            );
        }
        if let Some(e) = pollster::block_on(device.pop_error_scope()) {
            return Err(error(e));
        }
        Ok(Self {
            device,
            queue,
            layout,
            pipelines,
            max_buffer,
            info: DeviceInfo {
                name: info.name,
                vendor: info.vendor,
                device: info.device,
                device_type: format!("{:?}", info.device_type),
                driver: info.driver,
                driver_info: info.driver_info,
            },
        })
    }

    fn buffer(&self, values: &[f32]) -> Result<wgpu::Buffer, GteError> {
        if std::mem::size_of_val(values) as u64 > self.max_buffer {
            return Err(error("Vulkan buffer limit exceeded"));
        }
        Ok(self
            .device
            .create_buffer_init(&wgpu::util::BufferInitDescriptor {
                label: None,
                contents: bytemuck::cast_slice(values),
                usage: wgpu::BufferUsages::STORAGE | wgpu::BufferUsages::COPY_SRC,
            }))
    }
    fn empty(&self, count: usize) -> Result<wgpu::Buffer, GteError> {
        let size = count
            .checked_mul(4)
            .ok_or_else(|| error("Vulkan buffer overflow"))? as u64;
        if size > self.max_buffer {
            return Err(error("Vulkan buffer limit exceeded"));
        }
        Ok(self.device.create_buffer(&wgpu::BufferDescriptor {
            label: None,
            size,
            usage: wgpu::BufferUsages::STORAGE | wgpu::BufferUsages::COPY_SRC,
            mapped_at_creation: false,
        }))
    }
    fn run(
        &self,
        enc: &mut wgpu::CommandEncoder,
        kernel: &str,
        input: [&wgpu::Buffer; 3],
        out: &wgpu::Buffer,
        params: [u32; 8],
        groups: [u32; 3],
    ) {
        let uniform = self
            .device
            .create_buffer_init(&wgpu::util::BufferInitDescriptor {
                label: None,
                contents: bytemuck::cast_slice(&params),
                usage: wgpu::BufferUsages::UNIFORM,
            });
        let buffers = [input[0], input[1], input[2], out, &uniform];
        let entries: Vec<_> = buffers
            .iter()
            .enumerate()
            .map(|(i, b)| wgpu::BindGroupEntry {
                binding: i as u32,
                resource: b.as_entire_binding(),
            })
            .collect();
        let bindings = self.device.create_bind_group(&wgpu::BindGroupDescriptor {
            label: None,
            layout: &self.layout,
            entries: &entries,
        });
        let mut pass = enc.begin_compute_pass(&wgpu::ComputePassDescriptor {
            label: Some(kernel),
            timestamp_writes: None,
        });
        pass.set_pipeline(&self.pipelines[kernel]);
        pass.set_bind_group(0, &bindings, &[]);
        pass.dispatch_workgroups(groups[0], groups[1], groups[2]);
    }
    fn read(
        &self,
        enc: &mut wgpu::CommandEncoder,
        buffer: &wgpu::Buffer,
        count: usize,
    ) -> wgpu::Buffer {
        let out = self.device.create_buffer(&wgpu::BufferDescriptor {
            label: None,
            size: (count * 4) as u64,
            usage: wgpu::BufferUsages::MAP_READ | wgpu::BufferUsages::COPY_DST,
            mapped_at_creation: false,
        });
        enc.copy_buffer_to_buffer(buffer, 0, &out, 0, (count * 4) as u64);
        out
    }
    fn collect(&self, readback: &wgpu::Buffer) -> Result<Vec<f32>, GteError> {
        let (tx, rx) = std::sync::mpsc::channel();
        readback.slice(..).map_async(wgpu::MapMode::Read, move |r| {
            let _ = tx.send(r);
        });
        self.device.poll(wgpu::Maintain::Wait);
        rx.recv().map_err(error)?.map_err(error)?;
        let bytes = readback.slice(..).get_mapped_range();
        let values = bytemuck::cast_slice::<u8, f32>(&bytes).to_vec();
        drop(bytes);
        readback.unmap();
        if values.iter().any(|v| !v.is_finite()) {
            return Err(error("nonfinite Vulkan output"));
        }
        Ok(values)
    }

    /// Full transformer stays on the device, one layer submitted at a time to
    /// bound uploaded weights. The caller retains tokenisation and mean/L2 pooling.
    pub(crate) fn forward(
        &self,
        model: &Model,
        initial: &[f32],
        mask: &[bool],
        batch: usize,
        seq: usize,
    ) -> Result<Vec<f32>, GteError> {
        self.device.push_error_scope(wgpu::ErrorFilter::Validation);
        self.device.push_error_scope(wgpu::ErrorFilter::OutOfMemory);
        let result = self.forward_inner(model, initial, mask, batch, seq);
        self.device.poll(wgpu::Maintain::Wait);
        let oom = pollster::block_on(self.device.pop_error_scope());
        let validation = pollster::block_on(self.device.pop_error_scope());
        if let Some(e) = oom.or(validation) {
            return Err(error(e));
        }
        result
    }

    #[allow(clippy::too_many_lines, clippy::many_single_char_names)] // Tensor dimensions and one layer schedule.
    fn forward_inner(
        &self,
        model: &Model,
        initial: &[f32],
        mask: &[bool],
        batch: usize,
        seq: usize,
    ) -> Result<Vec<f32>, GteError> {
        let h = model.config.hidden_size;
        let rows = batch * seq;
        let inter = model.config.intermediate;
        if seq > 512
            || batch > 4
            || h != 384
            || inter != 1536
            || model.config.num_heads != 12
            || model.layers.len() != 12
        {
            return Err(error("Vulkan pretest shape limits exceeded"));
        }
        let heads = model.config.num_heads;
        let mask = self.buffer(
            &mask
                .iter()
                .map(|v| if *v { 1.0 } else { 0.0 })
                .collect::<Vec<_>>(),
        )?;
        let dummy = self.buffer(&[0.0])?;
        let mut hidden = self.buffer(initial)?;
        let mut enc = self
            .device
            .create_command_encoder(&wgpu::CommandEncoderDescriptor::default());
        let normed = self.empty(rows * h)?;
        let ew = self.buffer(&model.embed_ln_weight)?;
        let eb = self.buffer(&model.embed_ln_bias)?;
        let p = [rows as u32, 0, 0, seq as u32, heads as u32, h as u32, 0, 0];
        self.run(
            &mut enc,
            "norm",
            [&hidden, &ew, &eb],
            &normed,
            p,
            [rows.div_ceil(64) as u32, 1, 1],
        );
        self.queue.submit([enc.finish()]);
        hidden = normed;
        for layer in &model.layers {
            let mut enc = self
                .device
                .create_command_encoder(&wgpu::CommandEncoderDescriptor::default());
            let linear = |enc: &mut wgpu::CommandEncoder,
                          a: &wgpu::Buffer,
                          w: &[f32],
                          b: &[f32],
                          k: usize,
                          n: usize|
             -> Result<wgpu::Buffer, GteError> {
                let wb = self.buffer(w)?;
                let bb = self.buffer(b)?;
                let out = self.empty(rows * n)?;
                self.run(
                    enc,
                    "linear",
                    [a, &wb, &bb],
                    &out,
                    [rows as u32, n as u32, k as u32, 0, 0, 0, 0, 0],
                    [n.div_ceil(8) as u32, rows.div_ceil(8) as u32, 1],
                );
                Ok(out)
            };
            let q = linear(
                &mut enc,
                &hidden,
                &layer.query_weight,
                &layer.query_bias,
                h,
                h,
            )?;
            let k = linear(&mut enc, &hidden, &layer.key_weight, &layer.key_bias, h, h)?;
            let v = linear(
                &mut enc,
                &hidden,
                &layer.value_weight,
                &layer.value_bias,
                h,
                h,
            )?;
            let scores = self.empty(batch * heads * seq * seq)?;
            let probs = self.empty(batch * heads * seq * seq)?;
            self.run(
                &mut enc,
                "scores",
                [&q, &k, &mask],
                &scores,
                p,
                [
                    seq.div_ceil(8) as u32,
                    seq.div_ceil(8) as u32,
                    (batch * heads) as u32,
                ],
            );
            let mut sp = p;
            sp[0] = (batch * heads * seq) as u32;
            self.run(
                &mut enc,
                "softmax",
                [&scores, &dummy, &dummy],
                &probs,
                sp,
                [(batch * heads * seq).div_ceil(64) as u32, 1, 1],
            );
            let attn = self.empty(rows * h)?;
            self.run(
                &mut enc,
                "values",
                [&probs, &v, &mask],
                &attn,
                p,
                [h.div_ceil(8) as u32, seq.div_ceil(8) as u32, batch as u32],
            );
            let projected = linear(
                &mut enc,
                &attn,
                &layer.attn_output_weight,
                &layer.attn_output_bias,
                h,
                h,
            )?;
            let norm = |enc: &mut wgpu::CommandEncoder,
                        a: &wgpu::Buffer,
                        b: &wgpu::Buffer,
                        w: &[f32],
                        bias: &[f32]|
             -> Result<wgpu::Buffer, GteError> {
                let residual = self.empty(rows * h)?;
                let out = self.empty(rows * h)?;
                self.run(
                    enc,
                    "residual",
                    [a, b, &dummy],
                    &residual,
                    [(rows * h) as u32, 0, 0, 0, 0, 0, 0, 0],
                    [(rows * h).div_ceil(64) as u32, 1, 1],
                );
                let w = self.buffer(w)?;
                let bias = self.buffer(bias)?;
                self.run(
                    enc,
                    "norm",
                    [&residual, &w, &bias],
                    &out,
                    p,
                    [rows.div_ceil(64) as u32, 1, 1],
                );
                Ok(out)
            };
            let after_attn = norm(
                &mut enc,
                &projected,
                &hidden,
                &layer.attn_ln_weight,
                &layer.attn_ln_bias,
            )?;
            let ff = linear(
                &mut enc,
                &after_attn,
                &layer.ffn_inter_weight,
                &layer.ffn_inter_bias,
                h,
                inter,
            )?;
            let activated = self.empty(rows * inter)?;
            self.run(
                &mut enc,
                "gelu",
                [&ff, &dummy, &dummy],
                &activated,
                [(rows * inter) as u32, 0, 0, 0, 0, 0, 0, 0],
                [(rows * inter).div_ceil(64) as u32, 1, 1],
            );
            let ffout = linear(
                &mut enc,
                &activated,
                &layer.ffn_output_weight,
                &layer.ffn_output_bias,
                inter,
                h,
            )?;
            hidden = norm(
                &mut enc,
                &ffout,
                &after_attn,
                &layer.ffn_ln_weight,
                &layer.ffn_ln_bias,
            )?;
            self.queue.submit([enc.finish()]);
            self.device.poll(wgpu::Maintain::Wait);
        }
        let mut enc = self
            .device
            .create_command_encoder(&wgpu::CommandEncoderDescriptor::default());
        let readback = self.read(&mut enc, &hidden, rows * h);
        self.queue.submit([enc.finish()]);
        self.collect(&readback)
    }
}
