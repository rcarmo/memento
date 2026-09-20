package needle

import (
	"encoding/binary"
	"math"
	"testing"
)

func TestFP32AllHeaderValidationBranches(t *testing.T) {
	_, _, good := preparedFP32(t)
	configLength := int(binary.LittleEndian.Uint32(good[28:32]))
	headerLength := int(binary.LittleEndian.Uint32(good[12:16]))
	directory := fp32FixedSize + configLength
	nameLength := int(binary.LittleEndian.Uint16(good[directory : directory+2]))
	shapeAt := directory + 24 + nameLength
	mutations := []func([]byte) (int64, []byte){
		func(b []byte) (int64, []byte) { return fp32FixedSize - 1, b },
		func(b []byte) (int64, []byte) { return fp32MaxFileSize + 1, b },
		func(b []byte) (int64, []byte) { binary.LittleEndian.PutUint64(b[16:24], 123); return int64(len(b)), b },
		func(b []byte) (int64, []byte) {
			binary.LittleEndian.PutUint64(b[16:24], uint64(len(b)+1))
			return int64(len(b)), b
		},
		func(b []byte) (int64, []byte) {
			binary.LittleEndian.PutUint32(b[28:32], math.MaxUint32)
			return int64(len(b)), b
		},
		func(b []byte) (int64, []byte) {
			binary.LittleEndian.PutUint32(b[12:16], uint32(directory+23))
			return int64(len(b)), b
		},
		func(b []byte) (int64, []byte) {
			binary.LittleEndian.PutUint16(b[directory:directory+2], math.MaxUint16)
			return int64(len(b)), b
		},
		func(b []byte) (int64, []byte) {
			binary.LittleEndian.PutUint64(b[directory+16:directory+24], 3)
			return int64(len(b)), b
		},
		func(b []byte) (int64, []byte) {
			binary.LittleEndian.PutUint64(b[directory+8:directory+16], uint64(binary.LittleEndian.Uint64(b[16:24])-4))
			return int64(len(b)), b
		},
		func(b []byte) (int64, []byte) {
			binary.LittleEndian.PutUint64(b[directory+8:directory+16], uint64(len(b)))
			binary.LittleEndian.PutUint64(b[directory+16:directory+24], 2)
			return int64(len(b)), b
		},
		func(b []byte) (int64, []byte) { binary.LittleEndian.PutUint32(b[24:28], 2); return int64(len(b)), b },
		func(b []byte) (int64, []byte) {
			binary.LittleEndian.PutUint32(b[shapeAt:shapeAt+4], math.MaxUint32)
			binary.LittleEndian.PutUint32(b[shapeAt+4:shapeAt+8], math.MaxUint32)
			return int64(len(b)), b
		},
		func(b []byte) (int64, []byte) {
			// Duplicate the sole descriptor and extend the bounded directory.
			descriptor := append([]byte{}, b[directory:headerLength]...)
			copy(b[headerLength:], descriptor)
			binary.LittleEndian.PutUint32(b[24:28], 2)
			binary.LittleEndian.PutUint32(b[12:16], uint32(headerLength+len(descriptor)))
			return int64(len(b)), b
		},
		func(b []byte) (int64, []byte) {
			descriptor := append([]byte{}, b[directory:headerLength]...)
			descriptor[24]++
			copy(b[headerLength:], descriptor)
			binary.LittleEndian.PutUint32(b[24:28], 2)
			binary.LittleEndian.PutUint32(b[12:16], uint32(headerLength+len(descriptor)))
			return int64(len(b)), b
		},
	}
	for index, mutate := range mutations {
		candidate := append([]byte{}, good...)
		size, candidate := mutate(candidate)
		if _, _, err := parseFP32Header(candidate, size); err == nil {
			t.Fatalf("mutation %d accepted", index)
		}
	}
}
