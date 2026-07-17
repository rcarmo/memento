#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/../.."
MODEL_DIR=/tmp/go-gte/models/gte-small
OUT_MODEL=tests/fixtures/gte-small.gtemodel
OUT_JSON=tests/fixtures/go_parity.json
TMP_GO=$(mktemp /tmp/memento-go-parity-XXXX.go)
trap 'rm -f "$TMP_GO"' EXIT

if [[ ! -f "$MODEL_DIR/model.safetensors" ]]; then
  echo "model artifact not found: $MODEL_DIR/model.safetensors" >&2
  exit 1
fi

python3 /tmp/go-gte/convert_model.py "$MODEL_DIR" "$OUT_MODEL"

cat > "$TMP_GO" <<'GOEOF'
package main

import (
  "encoding/json"
  "os"
  "github.com/rcarmo/gte-go/gte"
)

const (
  tokenUNK = 100
  tokenCLS = 101
  tokenSEP = 102
)

func isPunctuation(b byte) bool {
  return (b >= 33 && b <= 47) || (b >= 58 && b <= 64) || (b >= 91 && b <= 96) || (b >= 123 && b <= 126)
}

func isWhitespace(b byte) bool {
  return b == ' ' || b == '\t' || b == '\n' || b == '\r'
}

func basicTokenize(text string) []string {
  tokens := []string{}
  i := 0
  for i < len(text) {
    for i < len(text) && isWhitespace(text[i]) { i++ }
    if i >= len(text) { break }
    start := i
    if isPunctuation(text[i]) {
      i++
    } else {
      for i < len(text) && !isWhitespace(text[i]) && !isPunctuation(text[i]) { i++ }
    }
    src := text[start:i]
    lowered := []byte(src)
    changed := false
    for j, c := range lowered {
      if c >= 'A' && c <= 'Z' {
        lowered[j] = c + 32
        changed = true
      }
    }
    if changed {
      tokens = append(tokens, string(lowered))
    } else {
      tokens = append(tokens, src)
    }
  }
  return tokens
}

func wordpieceTokenize(word string, vocab map[string]int, out []int) []int {
  if word == "" { return out }
  start := 0
  for start < len(word) {
    end := len(word)
    found := -1
    foundEnd := start
    for start < end {
      candidate := word[start:end]
      if start > 0 { candidate = "##" + candidate }
      if id, ok := vocab[candidate]; ok {
        found = id
        foundEnd = end
        break
      }
      end--
    }
    if found < 0 {
      out = append(out, tokenUNK)
      start++
    } else {
      out = append(out, found)
      start = foundEnd
    }
  }
  return out
}

func tokenize(text string, vocab map[string]int, maxSeqLen int) []int {
  basic := basicTokenize(text)
  tokens := make([]int, 0, maxSeqLen)
  tokens = append(tokens, tokenCLS)
  for _, tok := range basic {
    if len(tokens) >= maxSeqLen-1 { break }
    prev := len(tokens)
    tokens = wordpieceTokenize(tok, vocab, tokens)
    if len(tokens) > maxSeqLen-1 {
      tokens = tokens[:prev]
      break
    }
  }
  if len(tokens) < maxSeqLen { tokens = append(tokens, tokenSEP) }
  return tokens
}

func main() {
  model, err := gte.Load(os.Args[1])
  if err != nil { panic(err) }
  vocabMap := map[string]int{}
  for i, token := range model.Vocab { vocabMap[token] = i }
  texts := []string{"Hello world", "I love cats", "The stock market crashed", "Hello, worlds!"}
  type Item struct {
    Text string `json:"text"`
    Tokens []int `json:"tokens"`
    Embedding []float32 `json:"embedding"`
  }
  items := make([]Item, 0, len(texts))
  for _, text := range texts {
    emb, err := model.Embed(text)
    if err != nil { panic(err) }
    items = append(items, Item{Text: text, Tokens: tokenize(text, vocabMap, model.MaxLen()), Embedding: emb})
  }
  enc := json.NewEncoder(os.Stdout)
  enc.SetIndent("", "  ")
  if err := enc.Encode(map[string]any{"source":"/tmp/go-gte","license":"MIT","items":items}); err != nil { panic(err) }
}
GOEOF
(
  cd /tmp/go-gte
  go run "$TMP_GO" "$OLDPWD/$OUT_MODEL" > "$OLDPWD/$OUT_JSON"
)
