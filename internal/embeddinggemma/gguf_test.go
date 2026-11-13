package embeddinggemma

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"github.com/rcarmo/memento/internal/simd"
	"os"
	"path/filepath"
	"testing"
)

func fixture(t *testing.T, mutate func(*bytes.Buffer)) string {
	t.Helper()
	var b bytes.Buffer
	put := func(v any) {
		if err := binary.Write(&b, binary.LittleEndian, v); err != nil {
			t.Fatal(err)
		}
	}
	str := func(s string) { put(uint64(len(s))); b.WriteString(s) }
	b.WriteString("GGUF")
	put(uint32(3))
	put(uint64(1))
	put(uint64(1))
	str("general.architecture")
	put(uint32(8))
	str("gemma-embedding2")
	str("test.weight")
	put(uint32(2))
	put(uint64(256))
	put(uint64(2))
	put(uint32(Q4K))
	put(uint64(0))
	for b.Len()%32 != 0 {
		b.WriteByte(0)
	}
	b.Write(encodedRows(Q4K, 256, 2))
	if mutate != nil {
		mutate(&b)
	}
	p := filepath.Join(t.TempDir(), "model.gguf")
	if err := os.WriteFile(p, b.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	return p
}
func TestGGUFLoadOwnership(t *testing.T) {
	p := fixture(t, nil)
	f, err := Open(p)
	if err != nil {
		t.Fatal(err)
	}
	if f.Metadata()["general.architecture"] != "gemma-embedding2" {
		t.Fatal(f.Metadata())
	}
	ts := f.Tensors()
	ts[0].Shape[0] = 1
	if f.Tensors()[0].Shape[0] != 256 {
		t.Fatal("mutable shape exposed")
	}
	m, err := f.Matrix("test.weight")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.Matrix("missing"); err == nil {
		t.Fatal("missing tensor accepted")
	}
	if err = m.ProjectTo(context.Background(), make([]float32, 2), make([]float32, 256), make([]float32, 256), scalarEngine()); err != nil {
		t.Fatal(err)
	}
	if err = f.Close(); err != nil {
		t.Fatal(err)
	}
	if err = f.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err = f.Matrix("test.weight"); err == nil {
		t.Fatal("closed file accepted")
	}
}
func TestGGUFRejectMalformed(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*bytes.Buffer)
	}{{"magic", func(b *bytes.Buffer) { b.Bytes()[0] = 'X' }}, {"version", func(b *bytes.Buffer) { b.Bytes()[4] = 9 }}, {"count", func(b *bytes.Buffer) { binary.LittleEndian.PutUint64(b.Bytes()[8:], ^uint64(0)) }}, {"truncated", func(b *bytes.Buffer) { b.Truncate(b.Len() - 10) }}, {"architecture", func(b *bytes.Buffer) { p := bytes.Index(b.Bytes(), []byte("gemma-embedding2")); b.Bytes()[p] = 'X' }}}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if f, err := Open(fixture(t, tc.mutate)); err == nil {
				f.Close()
				t.Fatal("malformed file accepted")
			}
		})
	}
	if _, err := Open(filepath.Join(t.TempDir(), "absent")); err == nil {
		t.Fatal("missing file")
	}
}
func TestRealQuantizedModel(t *testing.T) {
	path := os.Getenv("EMBEDDINGGEMMA_MODEL")
	if path == "" {
		t.Skip("set EMBEDDINGGEMMA_MODEL for pinned checkpoint")
	}
	f, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	counts := map[Type]int{}
	for _, v := range f.Tensors() {
		counts[v.Type]++
	}
	t.Logf("tensors=%d encodings=%v metadata=%v", len(f.Tensors()), counts, f.Metadata())
	for _, name := range []string{"token_embd.weight", "blk.0.attn_q.weight", "per_layer_model_proj.weight", "output.weight"} {
		m, err := f.Matrix(name)
		if err != nil {
			t.Fatal(err)
		}
		rows, cols := m.Shape()
		row := make([]float32, cols)
		if err = m.RowTo(row, rows/2); err != nil {
			t.Fatal(err)
		}
		if name == "token_embd.weight" {
			continue
		}
		x := make([]float32, cols)
		for i := range x {
			x[i] = float32(i%17-8) / 17
		}
		scalar := make([]float32, rows)
		auto := make([]float32, rows)
		if err = m.ProjectTo(context.Background(), scalar, x, row, scalarEngine()); err != nil {
			t.Fatal(err)
		}
		e, _ := simd.New("auto")
		if err = m.ProjectTo(context.Background(), auto, x, row, e); err != nil {
			t.Fatal(err)
		}
		for i := range auto {
			d := float64(auto[i] - scalar[i])
			if d < 0 {
				d = -d
			}
			den := float64(scalar[i])
			if den < 0 {
				den = -den
			}
			if d > 3e-4*(1+den) {
				t.Fatalf("%s row%d difference%g", name, i, d)
			}
		}
		t.Logf("%s %s shape=%dx%d backend=%s", name, m.Type(), rows, cols, e.Backend())
	}
}
func FuzzGGUFHeader(f *testing.F) {
	var seed bytes.Buffer
	seed.WriteString("GGUF")
	binary.Write(&seed, binary.LittleEndian, uint32(3))
	binary.Write(&seed, binary.LittleEndian, uint64(0))
	binary.Write(&seed, binary.LittleEndian, uint64(0))
	f.Add(seed.Bytes())
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 65536 {
			t.Skip()
		}
		p := parser{r: bufio.NewReader(bytes.NewReader(b)), remaining: int64(len(b))}
		_, _, _, _ = p.header(uint64(len(b)))
	})
}
