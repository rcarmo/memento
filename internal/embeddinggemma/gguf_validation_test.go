package embeddinggemma

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"math"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

type metaPair struct {
	k   string
	typ uint32
	v   any
}
type testTensor struct {
	name   string
	shape  []uint64
	typ    Type
	offset uint64
}

func headerFixture(t *testing.T, meta []metaPair, ts []testTensor, data []byte) []byte {
	t.Helper()
	var b bytes.Buffer
	put := func(v any) {
		if e := binary.Write(&b, binary.LittleEndian, v); e != nil {
			t.Fatal(e)
		}
	}
	str := func(s string) { put(uint64(len(s))); b.WriteString(s) }
	b.WriteString("GGUF")
	put(uint32(3))
	put(uint64(len(ts)))
	put(uint64(len(meta)))
	for _, m := range meta {
		str(m.k)
		put(m.typ)
		switch m.typ {
		case 8:
			str(m.v.(string))
		case 9:
			a := m.v.([]byte)
			b.Write(a)
		default:
			put(m.v)
		}
	}
	for _, v := range ts {
		str(v.name)
		put(uint32(len(v.shape)))
		for _, d := range v.shape {
			put(d)
		}
		put(uint32(v.typ))
		put(v.offset)
	}
	for b.Len()%32 != 0 {
		b.WriteByte(0)
	}
	b.Write(data)
	return b.Bytes()
}
func TestParserValidation(t *testing.T) {
	valid := []metaPair{{"general.architecture", 8, "gemma-embedding2"}}
	tensor := testTensor{"w", []uint64{256, 1}, Q4K, 0}
	for _, tc := range []struct {
		name string
		meta []metaPair
		ts   []testTensor
		size int
	}{
		{"duplicate-meta", append(append([]metaPair{}, valid...), valid...), []testTensor{tensor}, 144},
		{"alignment-type", append(append([]metaPair{}, valid...), metaPair{"general.alignment", 8, "32"}), []testTensor{tensor}, 144},
		{"zero-alignment", append(append([]metaPair{}, valid...), metaPair{"general.alignment", 4, uint32(0)}), []testTensor{tensor}, 144},
		{"bad-alignment", append(append([]metaPair{}, valid...), metaPair{"general.alignment", 4, uint32(3)}), []testTensor{tensor}, 144},
		{"big-alignment", append(append([]metaPair{}, valid...), metaPair{"general.alignment", 4, uint32(8192)}), []testTensor{tensor}, 144},
		{"duplicate-tensor", valid, []testTensor{tensor, tensor}, 288},
		{"empty-tensor", valid, []testTensor{{"", []uint64{256}, Q4K, 0}}, 144},
		{"rank-zero", valid, []testTensor{{"w", nil, F32, 0}}, 144},
		{"rank-five", valid, []testTensor{{"w", []uint64{1, 1, 1, 1, 1}, F32, 0}}, 144},
		{"dim-zero", valid, []testTensor{{"w", []uint64{0}, F32, 0}}, 144},
		{"dim-overflow", valid, []testTensor{{"w", []uint64{1 << 30, 2}, F32, 0}}, 144},
		{"row-unaligned", valid, []testTensor{{"w", []uint64{255}, Q4K, 0}}, 144},
		{"encoding", valid, []testTensor{{"w", []uint64{256}, 99, 0}}, 144},
		{"offset-unaligned", valid, []testTensor{{"w", []uint64{256}, Q4K, 1}}, 145},
		{"overlap", valid, []testTensor{tensor, {"v", []uint64{256}, Q4K, 32}}, 288},
		{"file-bounds", valid, []testTensor{tensor}, 32},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := headerFixture(t, tc.meta, tc.ts, make([]byte, tc.size))
			p := parser{r: bufio.NewReader(bytes.NewReader(b)), remaining: int64(len(b))}
			if _, _, _, err := p.header(uint64(len(b))); err == nil {
				t.Fatal("accepted malformed header")
			}
		})
	}
	b := headerFixture(t, valid, []testTensor{tensor}, make([]byte, 144))
	for i := 0; i < len(b)-144; i++ {
		p := parser{r: bufio.NewReader(bytes.NewReader(b[:i])), remaining: int64(i)}
		if _, _, _, err := p.header(uint64(i)); err == nil {
			t.Fatalf("accepted truncated header at%d", i)
		}
	}
}
func TestMetadataValues(t *testing.T) {
	for _, tc := range []struct {
		typ  uint64
		v    any
		want any
	}{{0, uint8(255), uint64(255)}, {1, int8(-2), int64(-2)}, {2, uint16(65535), uint64(65535)}, {3, int16(-3), int64(-3)}, {4, uint32(4), uint64(4)}, {5, int32(-5), int64(-5)}, {6, float32(1.25), float64(1.25)}, {7, uint8(1), true}, {10, uint64(10), uint64(10)}, {11, int64(-11), int64(-11)}, {12, float64(1.5), float64(1.5)}} {
		var b bytes.Buffer
		binary.Write(&b, binary.LittleEndian, tc.v)
		p := parser{r: bufio.NewReader(&b), remaining: int64(b.Len())}
		v, err := p.value(tc.typ, false)
		if err != nil || v != tc.want {
			t.Fatal(tc.typ, v, err)
		}
	}
	for _, tc := range []struct {
		typ   uint64
		raw   []byte
		array bool
	}{{7, []byte{2}, false}, {99, nil, false}, {9, nil, true}, {8, bytes.Repeat([]byte{255}, 8), false}, {9, bytes.Repeat([]byte{255}, 12), false}} {
		p := parser{r: bufio.NewReader(bytes.NewReader(tc.raw)), remaining: int64(len(tc.raw))}
		if _, err := p.value(tc.typ, tc.array); err == nil {
			t.Fatal("bad metadata accepted", tc.typ)
		}
	}
	for _, typ := range []uint32{8, 4} {
		var b bytes.Buffer
		binary.Write(&b, binary.LittleEndian, typ)
		binary.Write(&b, binary.LittleEndian, uint64(2))
		if typ == 8 {
			for _, s := range []string{"a", "b"} {
				binary.Write(&b, binary.LittleEndian, uint64(len(s)))
				b.WriteString(s)
			}
		} else {
			binary.Write(&b, binary.LittleEndian, uint32(1))
			binary.Write(&b, binary.LittleEndian, uint32(2))
		}
		p := parser{r: bufio.NewReader(&b), remaining: int64(b.Len())}
		v, err := p.value(9, false)
		if err != nil || v == nil {
			t.Fatal(v, err)
		}
	}
	p := parser{r: bufio.NewReader(bytes.NewReader(make([]byte, 3))), remaining: 3}
	if _, err := p.uint(3); err == nil {
		t.Fatal("invalid width")
	}
	p = parser{r: bufio.NewReader(bytes.NewReader([]byte{1})), remaining: 8}
	if _, err := p.uint(8); err != io.ErrUnexpectedEOF {
		t.Fatal(err)
	}
}
func TestMappedLifecycleAndErrors(t *testing.T) {
	if _, err := Open(t.TempDir()); err == nil {
		t.Fatal("directory")
	}
	p := filepath.Join(t.TempDir(), "short")
	os.WriteFile(p, []byte("GGUF"), 0600)
	if _, err := Open(p); err == nil {
		t.Fatal("short")
	}
	f, err := Open(fixture(t, nil))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	m, _ := f.Matrix("test.weight")
	if rows, cols := m.Shape(); rows != 2 || cols != 256 || m.Type() != Q4K {
		t.Fatal(rows, cols, m.Type())
	}
	f.tensors["vector"] = Tensor{Name: "vector", Shape: []uint64{256}, Type: Q4K}
	if _, err := f.Matrix("vector"); err == nil {
		t.Fatal("vector matrix")
	}
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				if err := m.ProjectTo(context.Background(), make([]float32, 2), make([]float32, 256), make([]float32, 256), scalarEngine()); err != nil {
					t.Error(err)
				}
			}
		}()
	}
	wg.Wait()
	if err := m.ProjectTo(context.Background(), nil, nil, nil, scalarEngine()); err == nil {
		t.Fatal("bad shape")
	}
	if _, err := f.Matrix("absent"); err == nil {
		t.Fatal("absent")
	}
	var nilSlice []float32
	if overlaps(nilSlice, nilSlice) {
		t.Fatal("empty overlap")
	}
	if overlaps(make([]float32, 1), make([]float32, 1)) {
		t.Fatal("distinct overlap")
	}
	if Type(97).String() != "unknown(97)" {
		t.Fatal("type string")
	}
	for _, typ := range []Type{F32, F16, Q8_0, Q4K, Q5K, Q6K, BF16} {
		if typ.String() == "" {
			t.Fatal(typ)
		}
	}
	if v := half(0xfc00); !math.IsInf(float64(v), -1) {
		t.Fatal(v)
	}
}
