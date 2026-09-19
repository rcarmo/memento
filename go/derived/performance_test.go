package derived

import (
	"encoding/binary"
	"math"
	"testing"
)

func BenchmarkSemanticBlobCosine384(b *testing.B) {
	left := make([]float32, 384)
	blob := make([]byte, len(left)*4)
	for index := range left {
		left[index] = float32(index+1) / 384
		binary.LittleEndian.PutUint32(blob[index*4:], math.Float32bits(left[index]))
	}
	norm, err := semanticNorm(left)
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if _, err = semanticBlobCosine(left, norm, blob, norm, len(left)); err != nil {
			b.Fatal(err)
		}
	}
}

func TestSemanticCosineFailureBranches(t *testing.T) {
	valid := []float32{1}
	validBlob := semanticBlob(1)
	for _, test := range []struct {
		left      []float32
		leftNorm  float64
		blob      []byte
		rightNorm float64
		dim       int
	}{{valid, 1, validBlob, 1, 2}, {valid, 0, validBlob, 1, 1}, {valid, 1, validBlob, 0, 1}, {valid, 1, validBlob, math.NaN(), 1}, {valid, 1, semanticBlob(float32(math.NaN())), 1, 1}} {
		if _, err := semanticBlobCosine(test.left, test.leftNorm, test.blob, test.rightNorm, test.dim); err == nil {
			t.Fatal(test)
		}
	}
	for _, vector := range [][]float32{{}, {0}, {float32(math.NaN())}, {float32(math.Inf(1))}} {
		if _, err := semanticNorm(vector); err == nil {
			t.Fatal(vector)
		}
	}
}

func TestSemanticBlobCosineAllocationBudget(t *testing.T) {
	left := make([]float32, 384)
	blob := make([]byte, len(left)*4)
	for index := range left {
		left[index] = float32(index+1) / 384
		binary.LittleEndian.PutUint32(blob[index*4:], math.Float32bits(left[index]))
	}
	norm, err := semanticNorm(left)
	if err != nil {
		t.Fatal(err)
	}
	allocations := testing.AllocsPerRun(1000, func() {
		if _, err = semanticBlobCosine(left, norm, blob, norm, len(left)); err != nil {
			t.Fatal(err)
		}
	})
	if allocations != 0 {
		t.Fatalf("semantic blob cosine allocs/run = %v, want 0", allocations)
	}
}
