package gte

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"
)

func chunkTokenizer(t testing.TB) *Tokenizer {
	t.Helper()
	vocab := make([]string, 110)
	for i := range vocab {
		vocab[i] = "[unused]"
	}
	vocab[104], vocab[105], vocab[106], vocab[107], vocab[108], vocab[109] = "word", "tail", "a", "##a", "#", "界"
	tok, err := NewTokenizer(vocab, 512)
	if err != nil {
		t.Fatal(err)
	}
	return tok
}
func TestChunkBoundsAndTail(t *testing.T) {
	tok := chunkTokenizer(t)
	text := strings.Repeat("word ", 1200) + "tail"
	chunks, err := tok.Chunk(text, 384, 64, 4096)
	if err != nil || len(chunks) != 4 || !strings.HasSuffix(chunks[3], "tail") {
		t.Fatal(len(chunks), err)
	}
	if len(strings.Fields(chunks[0])) != 384 || len(strings.Fields(chunks[3])) != 241 {
		t.Fatal("incorrect overlap")
	}
	for _, text := range []string{"", " \n\t", "WORD", strings.Repeat("界😀", 900), strings.Repeat("a", 600), strings.Repeat("word ", 900) + "\n\n# tail", strings.Repeat("word ", 200) + "\n# tail\n" + strings.Repeat("word ", 300), "word" + strings.Repeat(" ", 5000) + "tail"} {
		chunks, err = tok.Chunk(text, 384, 64, 4096)
		if err != nil {
			t.Fatal(err)
		}
		for _, c := range chunks {
			tokens, e := tok.Tokenize(c)
			if e != nil || len(tokens) > 386 || utf8.RuneCountInString(c) > 4096 {
				t.Fatal(len(tokens), len(c), e)
			}
		}
		if strings.HasSuffix(text, "tail") && !strings.HasSuffix(chunks[len(chunks)-1], "tail") {
			t.Fatal("tail dropped")
		}
	}
	for _, limits := range [][3]int{{0, 0, 1}, {511, 64, 4096}, {384, -1, 4096}, {384, 384, 4096}, {384, 64, 0}} {
		if _, err = tok.Chunk("word", limits[0], limits[1], limits[2]); err == nil {
			t.Fatal(limits)
		}
	}
	if _, err = tok.Chunk("\xff", 384, 64, 4096); err == nil {
		t.Fatal("utf8")
	}
	chunks, err = tok.Chunk(strings.Repeat("WORD ", 30), 10, 0, 12)
	if err != nil || len(chunks) != 15 {
		t.Fatal(len(chunks), err)
	}
}

func tokenizerBytes() []byte {
	var buf bytes.Buffer
	buf.WriteString("GTE1")
	for _, v := range []uint32{110, 384, 12, 12, 1536, 512} {
		_ = binary.Write(&buf, binary.LittleEndian, v)
	}
	for i := 0; i < 110; i++ {
		w := "word"
		_ = binary.Write(&buf, binary.LittleEndian, uint16(len(w)))
		buf.WriteString(w)
	}
	return buf.Bytes()
}
func TestLoadChunkTokenizer(t *testing.T) {
	raw := tokenizerBytes()
	path := filepath.Join(t.TempDir(), "model")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	tok, err := LoadTokenizer(path)
	if err != nil {
		t.Fatal(err)
	}
	model := &Model{tokenizer: tok}
	chunks, err := model.Chunk(strings.Repeat("word ", 600), 384, 64, 4096)
	if err != nil || len(chunks) != 2 {
		t.Fatal(chunks, err)
	}
	if _, err = LoadTokenizer(path + "missing"); err == nil {
		t.Fatal("missing")
	}
	for _, n := range []int{0, 27, 28, 29, 31} {
		if _, err = readTokenizer(bytes.NewReader(raw[:n])); err == nil {
			t.Fatal(n)
		}
	}
	for _, offset := range []int{0, 4, 24} {
		copy := append([]byte{}, raw...)
		binary.LittleEndian.PutUint32(copy[offset:], 0)
		if _, err = readTokenizer(bytes.NewReader(copy)); err == nil {
			t.Fatal(offset)
		}
	}
	raw[30] = 0xff
	if _, err = readTokenizer(bytes.NewReader(raw)); err == nil {
		t.Fatal("invalid vocab")
	}
}
func TestChunkRealTokenizer(t *testing.T) {
	path := "../../models/gte/gte-small.gtemodel"
	if _, err := os.Stat(path); err != nil {
		t.Skip("model not installed")
	}
	tok, err := LoadTokenizer(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{strings.Repeat("Réseau 世界 🌍 plaintext JSON SQL_foo = 12.34\n", 500) + "tailmarker", strings.Repeat("a", 20000) + "tailmarker", strings.Repeat("NBSP\u00a0WIDE\u3000DASH—。ΑΒΓ İCAFÉ 界😀\n", 300) + "tailmarker"} {
		chunks, err := tok.Chunk(text, 384, 64, 4096)
		if err != nil || len(chunks) < 2 {
			t.Fatal(err)
		}
		for _, chunk := range chunks {
			tokens, err := tok.Tokenize(chunk)
			if err != nil || len(tokens) > 386 || utf8.RuneCountInString(chunk) > 4096 {
				t.Fatal(len(tokens), err)
			}
		}
		if !strings.HasSuffix(chunks[len(chunks)-1], "tailmarker") {
			t.Fatal("tail lost")
		}
	}
}
func FuzzChunk(t *testing.F) {
	t.Add("word\n\n# heading\n世界 tail")
	tok := chunkTokenizer(t)
	t.Fuzz(func(t *testing.T, text string) {
		if len(text) > 8192 || !utf8.ValidString(text) {
			return
		}
		chunks, err := tok.Chunk(text, 384, 64, 4096)
		if err != nil {
			t.Fatal(err)
		}
		for _, c := range chunks {
			tokens, e := tok.Tokenize(c)
			if e != nil || len(tokens) > 386 || utf8.RuneCountInString(c) > 4096 {
				t.Fatal(len(tokens), e)
			}
		}
	})
}
