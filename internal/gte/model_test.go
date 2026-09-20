package gte

import (
	"bytes"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func syntheticModel(layers int) []byte {
	var b bytes.Buffer
	b.WriteString("GTE1")
	for _, n := range []int{110, 2, layers, 1, 3, 8} {
		_ = binary.Write(&b, binary.LittleEndian, uint32(n))
	}
	for _, word := range testVocab() {
		_ = binary.Write(&b, binary.LittleEndian, uint16(len(word)))
		b.WriteString(word)
	}
	// embedding block; 4 projections each with bias, two norms, two FFN projections.
	count := 110*2 + 8*2 + 2*2 + 2 + 2 + layers*(4*(2*2+2)+2*(2+2)+3*2+3+2*3+2) + 2*2 + 2
	for i := 0; i < count; i++ {
		_ = binary.Write(&b, binary.LittleEndian, float32(i%7)/10)
	}
	return b.Bytes()
}

func TestModelLoadAndValidation(t *testing.T) {
	good := syntheticModel(2)
	m, err := FromBytes(good)
	if err != nil {
		t.Fatal(err)
	}
	if m.Dim() != 2 || m.Config().NumLayers != 2 || len(m.layers) != 2 {
		t.Fatal(m.Config())
	}
	tokens, err := m.Tokenize("hello")
	if err != nil || len(tokens) != 3 {
		t.Fatal(tokens, err)
	}
	before := m.token[0]
	good[len(good)-1] ^= 1
	if m.token[0] != before {
		t.Fatal("model alias")
	}
	path := filepath.Join(t.TempDir(), "model.gte")
	if err = os.WriteFile(path, good, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = Load(path); err != nil {
		t.Fatal(err)
	}
	if _, err = Load(path + "-absent"); err == nil {
		t.Fatal("missing file accepted")
	}
	if _, err = FromBytes(append(good, []byte("trailing")...)); err != nil {
		t.Fatal("reference accepts trailing bytes", err)
	}
	for n := 0; n < len(good); n++ {
		if _, err = FromBytes(good[:n]); err == nil {
			t.Fatalf("truncation %d accepted", n)
		}
	}
	bad := append([]byte(nil), good...)
	bad[0] = '?'
	if _, err = FromBytes(bad); err == nil || err.Error() != "invalid model magic" {
		t.Fatal(err)
	}
	for _, c := range []struct {
		At      int
		Value   uint32
		Message string
	}{
		{4, 103, "invalid model: vocab_size must include reserved token id 103"},
		{8, 0, "invalid model: hidden_size and intermediate must be positive"},
		{20, 0, "invalid model: hidden_size and intermediate must be positive"},
		{16, 0, "invalid model: num_heads must be positive and divide hidden_size: 0 heads, 2 hidden"},
		{16, 3, "invalid model: num_heads must be positive and divide hidden_size: 3 heads, 2 hidden"},
		{24, 1, "invalid model: max_seq_len must be at least 2"},
		{4, 0xffffffff, "io error: unexpected end of file"},
		{8, 0xffffffff, "io error: unexpected end of file"},
		{20, 0xffffffff, "io error: unexpected end of file"},
		{12, 0xffffffff, "io error: unexpected end of file"},
		{24, 0xffffffff, "io error: unexpected end of file"},
	} {
		b := append([]byte(nil), good...)
		binary.LittleEndian.PutUint32(b[c.At:], c.Value)
		if _, err = FromBytes(b); err == nil || err.Error() != c.Message {
			t.Errorf("header %d/%d: %v", c.At, c.Value, err)
		}
	}
	bad = append([]byte(nil), good...)
	bad[30] = 0xff
	if _, err = FromBytes(bad); err == nil || err.Error() != "invalid model: vocabulary is not UTF-8" {
		t.Fatal(err)
	}
}

func TestModelReaderFailureRetention(t *testing.T) {
	r := modelReader{data: []byte{1, 2}}
	if r.take(-1) != nil || !errors.Is(r.err, errModelEOF) {
		t.Fatal(r.err)
	}
	if r.take(1) != nil || r.weights(1) != nil {
		t.Fatal("continued after error")
	}
}

func FuzzModelParser(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte("GTE1"))
	f.Add(syntheticModel(0))
	f.Add(syntheticModel(1))
	f.Fuzz(func(t *testing.T, data []byte) {
		m, err := FromBytes(data)
		if err != nil {
			return
		}
		if m.Dim() == 0 || m.Config().MaxSequence < 2 {
			t.Fatal(m.Config())
		}
	})
}
