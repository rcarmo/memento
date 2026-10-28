# Quantised layout provenance

`quant.go` adapts GGUF Q4_K/Q5_K/Q6_K row-decoding layouts from `loader/gguf/quant_matrix.go` in [rcarmo/go-pherence](https://github.com/rcarmo/go-pherence), inspected at commit `2c283159`. The original package's immutable quantised-matrix and caller-workspace design also informed this implementation. No workspace source import or native runtime dependency is used.

The fused amd64 Q4 kernels and bounded GGUF parser are implemented here. Existing `internal/simd` supplies CPU capability dispatch and decoded-row fallback dots.

## MIT License

Copyright (c) 2026 Rui Carmo

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
