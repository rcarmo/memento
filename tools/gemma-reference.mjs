#!/usr/bin/env bun
/**
 * Independent EmbeddingGemma parity check.
 *
 * Usage:
 *   bun tools/gemma-reference.mjs --write-fixture
 *   GEMMA_NATIVE=/absolute/path/to/memento-gemma-embed bun tools/gemma-reference.mjs
 *
 * The fixture is a deliberate, F32-reference test asset. It is generated only
 * by --write-fixture and normal runs never rewrite it.
 */
import { createHash } from 'node:crypto';
import { constants, createReadStream } from 'node:fs';
import { access, mkdir, mkdtemp, readFile, realpath, rm, writeFile } from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const args = new Set(process.argv.slice(2));
if ([...args].some((arg) => arg !== '--write-fixture' && arg !== '--help')) {
  throw new Error('usage: bun tools/gemma-reference.mjs [--write-fixture]');
}
if (args.has('--help')) {
  console.log('usage: bun tools/gemma-reference.mjs [--write-fixture]');
  process.exit(0);
}

// The project helper resolves PROJECT_TMP_ROOT before it redirects child TMPDIR.
// Track a helper-created run so it can be removed with this completed command.
const inheritedRunRoot = process.env.MEMENTO_RUN_ROOT;
await import('./project-env.mjs');

const REPO = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const FIXTURE = path.join(REPO, 'internal/embeddinggemma/testdata/reference.json');
const PROJECT = 'memento-go';
const MODEL_SHA256 = 'ea905fd08e8061db77a0031cbb5096d459d7cfd2bdd0ad2ba66e483f16719493';
const LLAMA_CPP_REVISION = 'c3df4b7e22a1ac38da663552d19e649dd97ca936';
const TOKENIZE_URL = process.env.GEMMA_REFERENCE_TOKENIZE_URL ?? 'http://127.0.0.1:18091';
const F32_URL = process.env.GEMMA_REFERENCE_F32_URL ?? 'http://127.0.0.1:18092';
const TIMEOUT_MS = 180_000;
const VECTOR_LIMITS = { cosine: 1 - 1e-9, rms: 3e-7, max: 1e-6 };
const FIXTURE_LIMITS = { cosine: 0.99999999, rms: 0.0000001, max: 0.000001 };

const cancelled = new AbortController();
let activeProcess;
for (const signal of ['SIGINT', 'SIGTERM']) {
  process.on(signal, () => {
    cancelled.abort(new Error(`cancelled by ${signal}`));
    activeProcess?.kill('SIGINT');
  });
}

function check(condition, message) {
  if (!condition) throw new Error(message);
}

function promptFor({ text, role = 'raw', title = '' }) {
  switch (role) {
    case 'raw': return text;
    case 'query': return `task: search result | query: ${text}`;
    case 'document': return `title: ${title || 'none'} | text: ${text}`;
    default: throw new Error(`unknown role ${JSON.stringify(role)}`);
  }
}

// 1025 repeats tokenise to roughly 1027 tokens (including BOS/EOS) in the
// pinned tokenizer, exercising the +/-512 local-attention boundary.
function cases() {
  return [
    { name: 'empty', text: '', role: 'raw', dimensions: 768 },
    { name: 'hello-world', text: 'Hello world!', role: 'raw', dimensions: 768 },
    { name: 'unicode-cjk-emoji', text: 'café cafe\u0301 中文 😀', role: 'raw', dimensions: 768 },
    { name: 'whitespace', text: 'multiple   spaces\tand\nnewlines\n', role: 'raw', dimensions: 768 },
    { name: 'special-token-literal', text: '<|image|>', role: 'raw', dimensions: 768 },
    { name: 'query-role', text: 'Where is the library?', role: 'query', dimensions: 768 },
    { name: 'document-role', text: 'The library is beside the river.', role: 'document', title: 'Readme', dimensions: 768 },
    { name: 'matryoshka-128', text: 'Matryoshka dimensions are independently normalized.', role: 'raw', dimensions: 128 },
    { name: 'matryoshka-256', text: 'Matryoshka dimensions are independently normalized.', role: 'raw', dimensions: 256 },
    { name: 'matryoshka-512', text: 'Matryoshka dimensions are independently normalized.', role: 'raw', dimensions: 512 },
    { name: 'long-local-attention-window', text: 'hello '.repeat(1025), role: 'raw', dimensions: 768 },
  ].map((entry) => ({ ...entry, prompt: promptFor(entry) }));
}

function signalWithTimeout() {
  return AbortSignal.any([cancelled.signal, AbortSignal.timeout(TIMEOUT_MS)]);
}

async function postJSON(baseURL, endpoint, body) {
  let response;
  try {
    response = await fetch(new URL(endpoint, `${baseURL}/`), {
      method: 'POST',
      headers: { 'content-type': 'application/json' },
      body: JSON.stringify(body),
      signal: signalWithTimeout(),
    });
  } catch (error) {
    throw new Error(`${baseURL}${endpoint} request failed or exceeded ${TIMEOUT_MS / 1000}s: ${error.message}`);
  }
  const text = await response.text();
  if (!response.ok) throw new Error(`${baseURL}${endpoint}: HTTP ${response.status}: ${text.slice(0, 1000)}`);
  try {
    return JSON.parse(text);
  } catch (error) {
    throw new Error(`${baseURL}${endpoint}: invalid JSON: ${error.message}`);
  }
}

async function referenceTokens(prompt) {
  const json = await postJSON(TOKENIZE_URL, '/tokenize', {
    content: prompt, add_special: true, parse_special: true,
  });
  check(Array.isArray(json.tokens) && json.tokens.every(Number.isInteger), 'reference /tokenize returned no integer tokens');
  return json.tokens;
}

async function referenceEmbedding(prompt) {
  const json = await postJSON(F32_URL, '/embedding', { content: prompt, embedding_normalize: 2 });
  const vector = json?.[0]?.embedding?.[0];
  check(Array.isArray(vector) && vector.length === 768 && vector.every(Number.isFinite),
    'reference /embedding response is not [{embedding:[[768 finite F32 values]]}]');
  return vector;
}

function normalisedPrefix(vector, dimensions) {
  check(vector.length >= dimensions, `reference vector has ${vector.length}, need ${dimensions}`);
  const result = vector.slice(0, dimensions);
  let square = 0;
  for (const value of result) square += value * value;
  check(Number.isFinite(square) && square > 0, 'reference vector has invalid norm');
  const inverse = 1 / Math.sqrt(square);
  return result.map((value) => value * inverse);
}

function metrics(actual, expected) {
  check(actual.length === expected.length, `vector length ${actual.length}, expected ${expected.length}`);
  let dot = 0, actualSquare = 0, expectedSquare = 0, sumSquare = 0, max = 0;
  for (let i = 0; i < actual.length; i++) {
    const a = actual[i], b = expected[i];
    check(Number.isFinite(a) && Number.isFinite(b), `non-finite vector value at ${i}`);
    const delta = Math.abs(a - b);
    if (delta > max) max = delta;
    sumSquare += delta * delta;
    dot += a * b;
    actualSquare += a * a;
    expectedSquare += b * b;
  }
  return {
    max,
    rms: Math.sqrt(sumSquare / actual.length),
    cosine: dot / Math.sqrt(actualSquare * expectedSquare),
  };
}

function within(value, limits) {
  return value.cosine >= limits.cosine && value.rms <= limits.rms && value.max <= limits.max;
}

function formatMetrics(value) {
  return `cos=${value.cosine.toFixed(9)} rms=${value.rms.toExponential(3)} max=${value.max.toExponential(3)}`;
}

async function sha256(file) {
  return await new Promise((resolve, reject) => {
    const hash = createHash('sha256');
    const stream = createReadStream(file);
    stream.on('error', reject);
    stream.on('data', (chunk) => hash.update(chunk));
    stream.on('end', () => resolve(hash.digest('hex')));
  });
}

async function executable(file) {
  try {
    await access(file, constants.X_OK);
  } catch {
    throw new Error(`GEMMA_NATIVE must name the built memento-gemma-embed executable; cannot access ${file}`);
  }
}

async function runNative(native, model, entry) {
  const command = [native, '-model', model, '-text', entry.text, '-backend', process.env.GEMMA_BACKEND ?? 'auto',
    '-dimensions', String(entry.dimensions), '-role', entry.role];
  if (entry.title) command.push('-title', entry.title);
  const proc = Bun.spawn(command, { stdout: 'pipe', stderr: 'pipe', env: process.env });
  activeProcess = proc;
  const stdout = new Response(proc.stdout).text();
  const stderr = new Response(proc.stderr).text();
  let timedOut = false;
  const timer = setTimeout(() => {
    timedOut = true;
    proc.kill('SIGINT');
    setTimeout(() => proc.kill('SIGKILL'), 2_000).unref();
  }, TIMEOUT_MS);
  const onAbort = () => proc.kill('SIGINT');
  cancelled.signal.addEventListener('abort', onAbort, { once: true });
  const [exitCode, output, errors] = await Promise.all([proc.exited, stdout, stderr]);
  clearTimeout(timer);
  cancelled.signal.removeEventListener('abort', onAbort);
  activeProcess = undefined;
  if (timedOut) throw new Error(`${entry.name}: native command exceeded ${TIMEOUT_MS / 1000}s`);
  if (cancelled.signal.aborted) throw cancelled.signal.reason;
  if (exitCode !== 0) throw new Error(`${entry.name}: native exited ${exitCode}: ${errors.trim().slice(0, 2000)}`);
  let json;
  try {
    json = JSON.parse(output);
  } catch (error) {
    throw new Error(`${entry.name}: native returned invalid JSON: ${error.message}; stderr: ${errors.trim().slice(0, 500)}`);
  }
  check(Array.isArray(json.tokens) && json.tokens.every(Number.isInteger), `${entry.name}: native JSON lacks integer tokens`);
  check(Array.isArray(json.embedding) && json.embedding.length === entry.dimensions && json.embedding.every(Number.isFinite),
    `${entry.name}: native JSON lacks ${entry.dimensions} finite embedding values`);
  return json;
}

function fixtureDocument(entries) {
  return {
    format: 1,
    provenance: {
      llama_cpp_revision: LLAMA_CPP_REVISION,
      gelu: 'direct-f32-no-fp16-lookup',
      source_patch: 'ggml/src/ggml-cpu/vec.h: undefine GGML_GELU_FP16; src/llama-quant.cpp: remove per_layer_model_proj quantization exclusion so F32 conversion dequantizes all 2D weights',
      build: 'Release; GGML_NATIVE=OFF; GGML_CUDA=OFF; GGML_VULKAN=OFF; CPU 4 threads; --flash-attn off',
      quantized_model_sha256: MODEL_SHA256,
      tokenize: { url: TOKENIZE_URL, request: { add_special: true, parse_special: true } },
      embedding: { url: F32_URL, request: { embedding_normalize: 2 }, precision: 'F32 dequantized GGUF', dimensions: 768 },
    },
    cases: entries,
  };
}

function validateFixture(fixture) {
  check(fixture?.format === 1, 'fixture format must be 1');
  check(fixture?.provenance?.gelu === 'direct-f32-no-fp16-lookup', 'fixture must identify direct FP32 GELU reference');
  check(fixture?.provenance?.llama_cpp_revision === LLAMA_CPP_REVISION, 'fixture does not use the pinned llama.cpp revision');
  check(fixture?.provenance?.quantized_model_sha256 === MODEL_SHA256, 'fixture does not use the pinned quantized model SHA-256');
  check(Array.isArray(fixture.cases) && fixture.cases.length > 0, 'fixture has no cases');
  const expected = cases();
  check(fixture.cases.length === expected.length, 'fixture case set differs from harness case set; regenerate deliberately');
  for (let i = 0; i < expected.length; i++) {
    const got = fixture.cases[i], want = expected[i];
    for (const key of ['name', 'text', 'role', 'title', 'dimensions', 'prompt']) {
      check((got[key] ?? '') === (want[key] ?? ''), `fixture case ${i} ${key} differs from harness definition`);
    }
    check(Array.isArray(got.tokens) && got.tokens.every(Number.isInteger), `${got.name}: fixture tokens invalid`);
    check(Array.isArray(got.embedding) && got.embedding.length === 768 && got.embedding.every(Number.isFinite),
      `${got.name}: fixture F32 embedding invalid`);
  }
}

let generatedRunRoot = !inheritedRunRoot ? process.env.MEMENTO_RUN_ROOT : undefined;
let scratch;
try {
  const projectTmp = process.env.PROJECT_TMP_ROOT;
  check(path.isAbsolute(projectTmp ?? '') && path.basename(projectTmp) === PROJECT,
    'PROJECT_TMP_ROOT must be an absolute project-owned memento-go root');
  const runParent = path.join(projectTmp, 'runs', 'gemma-reference');
  await mkdir(runParent, { recursive: true });
  scratch = await mkdtemp(path.join(runParent, 'run-'));
  await mkdir(path.join(scratch, 'tmp'));
  process.env.TMPDIR = process.env.TMP = process.env.TEMP = path.join(scratch, 'tmp');

  if (args.has('--write-fixture')) {
    const entries = [];
    for (const entry of cases()) {
      const [tokens, embedding] = await Promise.all([referenceTokens(entry.prompt), referenceEmbedding(entry.prompt)]);
      entries.push({ ...entry, tokens, embedding });
      console.log(`reference ${entry.name}: tokens=${tokens.length} dims=${embedding.length}`);
    }
    const fixture = fixtureDocument(entries);
    validateFixture(fixture);
    await writeFile(FIXTURE, `${JSON.stringify(fixture, null, 2)}\n`);
    console.log(`wrote ${path.relative(REPO, FIXTURE)} (${entries.length} F32 reference cases)`);
  } else {
    const native = process.env.GEMMA_NATIVE;
    check(native, 'GEMMA_NATIVE is required (absolute path to built memento-gemma-embed)');
    check(path.isAbsolute(native), 'GEMMA_NATIVE must be absolute');
    await executable(native);
    const configuredModel = process.env.GEMMA_MODEL ?? path.join(REPO, 'models/embeddinggemma/embeddinggemma-2-UD-Q4_K_XL.gguf');
    const model = await realpath(configuredModel);
    check(path.isAbsolute(model), 'GEMMA_MODEL must resolve to an absolute path');
    const digest = await sha256(model);
    check(digest === MODEL_SHA256, `GEMMA_MODEL SHA-256 ${digest} is not pinned model ${MODEL_SHA256}`);
    const fixture = JSON.parse(await readFile(FIXTURE, 'utf8'));
    validateFixture(fixture);
    let failures = 0;
    for (const entry of fixture.cases) {
      const [liveTokens, liveEmbedding, nativeOutput] = await Promise.all([
        referenceTokens(entry.prompt), referenceEmbedding(entry.prompt), runNative(native, model, entry),
      ]);
      const fixtureReference = metrics(liveEmbedding, entry.embedding);
      const expected = normalisedPrefix(entry.embedding, entry.dimensions);
      const nativeReference = metrics(nativeOutput.embedding, expected);
      const tokensMatch = JSON.stringify(liveTokens) === JSON.stringify(entry.tokens) &&
        JSON.stringify(nativeOutput.tokens) === JSON.stringify(liveTokens);
      const referenceOK = within(fixtureReference, FIXTURE_LIMITS);
      const vectorsOK = within(nativeReference, VECTOR_LIMITS);
      const result = tokensMatch && referenceOK && vectorsOK;
      if (!result) failures++;
      console.log(`${result ? 'PASS' : 'FAIL'} ${entry.name} tokens=${liveTokens.length} ${formatMetrics(nativeReference)}` +
        `${referenceOK ? '' : ` fixture-drift(${formatMetrics(fixtureReference)})`}` +
        `${tokensMatch ? '' : ' token-mismatch'}`);
    }
    console.log(`limits: cosine>=${VECTOR_LIMITS.cosine} rms<=${VECTOR_LIMITS.rms} max<=${VECTOR_LIMITS.max}; failures=${failures}`);
    if (failures) process.exitCode = 1;
  }
} finally {
  activeProcess?.kill('SIGINT');
  if (scratch) await rm(scratch, { recursive: true, force: true });
  if (generatedRunRoot) await rm(generatedRunRoot, { recursive: true, force: true });
}
