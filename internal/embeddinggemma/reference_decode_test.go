package embeddinggemma

import (
	"bufio"
	"encoding/binary"
	"io"
	"math"
	"os"
	"testing"
)

// Optional independent llama.cpp F32-dequantized weights diagnose format parity.
func TestReferenceDequantizedWeights(t *testing.T) {
	path := os.Getenv("EMBEDDINGGEMMA_F32_REFERENCE")
	if path == "" {
		t.Skip("no independent dequantized reference")
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	st, _ := f.Stat()
	p := parser{r: bufio.NewReaderSize(io.NewSectionReader(f, 0, min(st.Size(), maxHeaderBytes)), 64<<10), remaining: min(st.Size(), maxHeaderBytes)}
	_, ts, offset, err := p.header(uint64(st.Size()))
	if err != nil {
		t.Fatal(err)
	}
	m := realModel(t, "auto")
	var mx float64
	var bad string
	for _, r := range ts {
		orig := m.file.tensors[r.Name]
		if len(orig.Shape) != 2 {
			continue
		}
		if r.Type != F32 {
			t.Fatalf("reference tensor %s remains quantised (%d)", r.Name, r.Type)
		}
		mat, _ := m.file.Matrix(r.Name)
		for _, row := range []int{0, mat.rows / 2, mat.rows - 1} {
			want := make([]byte, mat.columns*4)
			_, err = f.ReadAt(want, int64(offset+r.Offset+uint64(row*mat.columns*4)))
			if err != nil {
				t.Fatal(err)
			}
			got := make([]float32, mat.columns)
			if err := mat.RowTo(got, row); err != nil {
				t.Fatal(err)
			}
			for j, x := range got {
				y := math.Float32frombits(binary.LittleEndian.Uint32(want[j*4:]))
				d := math.Abs(float64(x - y))
				if d > mx {
					mx = d
					bad = r.Name
				}
			}
		}
	}
	t.Log(mx, bad)
	if mx > 1e-5 {
		t.Fatal("dequant mismatch")
	}
}
