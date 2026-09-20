# Attribution

This Go port is part of Memento, MIT licensed; see the repository LICENSE.

`umcp/shared.go` ports behaviour and helper logic from Rui Carmo's MIT-licensed uMCP (`umcp_shared.py`, reference commit recorded in `testdata/parity/baseline.json`). The original uMCP notice is retained below.

Scalar vector functions are ported from Memento's MIT-licensed Rust vector library, which attributes its original embedding/vector implementation to `rcarmo/go-gte`. See `docs/attribution.md` in the repository for the full model and code provenance. `gte/` restores model layout, tokenizer and scalar transformer algorithms from `rcarmo/go-gte` at `d2ffa3a5aaf7be72b178970f48c835c0d8fda5bf`, originally derived from antirez/gte-pure-C. It retains Memento's Unicode/bounds/cancellation behaviour. Memento's separately reviewed SSE2, AVX2 and NEON kernels are informed by the upstream assembly design; no BLAS or fast-math runtime is imported. No model weights are committed in this subtree. Model provenance and redistribution terms remain in the repository attribution and runtime-model manifest.

## Concept frontmatter (MIT)

`repository/frontmatter.go` and `repository/serialize.go` adapt detection, resolver and scalar-emission rules from python-frontmatter 1.3.0, PyYAML 6.0.3 and ruamel.yaml 0.18.17. Their MIT notices are retained under `repository/licenses/`. Changes use the pure-Go `go.yaml.in/yaml/v3 v3.0.4` node parser with PyYAML-compatible resolution and a focused metadata emitter. Pydantic 2.13.5/core 2.46.5 remain differential-test references only; no Python library is used at runtime.

## Markdown links (MIT)

`repository/links.go` adapts link validation and URI normalisation rules from markdown-it-py 3.0.0, markdown-it and mdurl 0.1.2, using Goldmark v1.7.16 for pure-Go CommonMark parsing and `golang.org/x/net/idna` for hostname conversion. The Python/project MIT notices are retained under `repository/licenses/`. Source extraction and rename logic comes from Memento; the parser/URI edge compatibility limits are tracked in the parity matrix.

## Git storage dependencies

Packed/loose Git object and reference decoding uses MIT-licensed go-git v5.19.2 and go-billy v5.9.0. Main publication uses Memento's own Git-compatible lock-file CAS; it does not use the dependency's advisory-lock ref writer. No Git subprocess is used by the Go runtime. Dependency versions and transitive licence sources remain recorded in `go.mod`/`go.sum` and the Go module cache; final distribution licence bundling is a packaging gate.

## SQLite control storage

Control databases use `modernc.org/sqlite v1.38.2` (BSD-3-Clause wrapper, generated Go SQLite implementation; upstream SQLite is public domain) without CGo or native extensions. Transitive dependencies are pinned in `go.mod`/`go.sum`; final distribution licence bundling remains a packaging gate. Memento's migration SQL and compatibility routines retain the repository MIT licence.

## Managed access cryptography

Access-key wrapping uses Go's standard AES-GCM and HMAC/SHA-256 plus BSD-licensed `golang.org/x/crypto/scrypt v0.53.0`, matching the Python cryptography reference's persisted format. All crypto fixtures use explicitly synthetic keys/tokens. Runtime secrets remain caller-supplied and are never committed.

## Asset MIME table

`assets/mime.json` captures Python mimetypes' lookup data plus the oracle host's Debian `media-types` table (public domain). The fixture records interpreter and source-file hashes; the copied media-types notice is in `assets/licenses/media-types.txt`. ZIP parsing uses the Go standard library (including bzip2), with Memento's MIT-licensed validation rules ported directly.

## SentencePiece port (Apache-2.0)

`sentencepiece/` is a modified Go port of the inference algorithms in [sentencepiece-rust 0.1.1](https://github.com/VoiceLessQ/sentencepiece-rust), itself based on Google's SentencePiece and Darts-clone read-side algorithms. The original Apache-2.0 licence is retained at `sentencepiece/licenses/Apache-2.0.txt`; this derived package is distributed under those terms, not relicensed as MIT by the repository's general notice.

Changes: Go protobuf/model reader, float32 BPE/Unigram and normaliser loops, explicit owned-return/context handling, and Go differential/fuzz tests. The reference's known user-defined normaliser-prefix limitation is preserved rather than silently replaced by a different tokenizer. No C++ wrapper, CGo or training runtime is imported.

## Synchronous HTTP parser (PSF licence)

`umcp/http_sync_parser.go` and `umcp/http_sync_connection.go` adapt parsing, connection and HTML-error behaviour from CPython 3.13.14 `http.server`, `http.client` and `email` modules. Copyright (c) 2001 Python Software Foundation; All Rights Reserved. The PSF licence and historical notices are retained in [`umcp/licenses/Python-3.13.14.txt`](umcp/licenses/Python-3.13.14.txt). Changes translate those routines into Go, add context cancellation and explicit stream ownership, and preserve the pinned reference through synthetic byte-stream fixtures. These files have no Python runtime dependency and retain the PSF terms rather than being relicensed by the repository's general MIT notice.

## Original Go GTE licence

MIT License

Copyright (c) 2026 Rui Carmo

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.

## uMCP licence

MIT License

Copyright (c) 2025 Rui Carmo

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.

## Proposal preview diffs

Proposal previews use `github.com/pmezard/go-difflib v1.0.0`, a pure-Go port of
Python's SequenceMatcher/unified diff algorithm, under its BSD-3-Clause licence.
Python splitlines boundaries and reference preview bytes are tested separately.

Copyright (c) 2013, Patrick Mezard
All rights reserved.

Redistribution and use in source and binary forms, with or without
modification, are permitted provided that the following conditions are
met:

    Redistributions of source code must retain the above copyright
notice, this list of conditions and the following disclaimer.
    Redistributions in binary form must reproduce the above copyright
notice, this list of conditions and the following disclaimer in the
documentation and/or other materials provided with the distribution.
    The names of its contributors may not be used to endorse or promote
products derived from this software without specific prior written
permission.

THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS
IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED
TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A
PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT
HOLDER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL,
SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED
TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA, OR
PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF
LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING
NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE OF THIS
SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
