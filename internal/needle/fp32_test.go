package needle

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func preparedFP32(t *testing.T) (string, []byte, []byte) {
	t.Helper()
	directory := t.TempDir()
	source := filepath.Join(directory, "model.ndl")
	destination := filepath.Join(directory, "model.nfp32")
	raw := goodModel()
	if err := os.WriteFile(source, raw, 0600); err != nil {
		t.Fatal(err)
	}
	if err := PrepareFP32(source, destination); err != nil {
		t.Fatal(err)
	}
	prepared, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	return destination, raw, prepared
}

func TestFP32PreparationAndMapping(t *testing.T) {
	destination, raw, first := preparedFP32(t)
	source := filepath.Join(t.TempDir(), "model.ndl")
	if err := os.WriteFile(source, raw, 0600); err != nil {
		t.Fatal(err)
	}
	secondPath := filepath.Join(t.TempDir(), "second.nfp32")
	if err := PrepareFP32(source, secondPath); err != nil {
		t.Fatal(err)
	}
	second, err := os.ReadFile(secondPath)
	if err != nil || !bytes.Equal(first, second) {
		t.Fatal(err, len(first), len(second))
	}
	stat, err := os.Stat(destination)
	if err != nil || stat.Mode().Perm() != 0644 {
		t.Fatal(stat, err)
	}
	info, err := ReadFP32Info(destination)
	if err != nil || info.TensorCount != 1 || info.SourceSHA256 != sha256.Sum256(raw) || info.FileSize != int64(len(first)) {
		t.Fatal(info, err)
	}
	if err = VerifyFP32(destination); err != nil {
		t.Fatal(err)
	}
	corrupt := append([]byte{}, first...)
	corrupt[len(corrupt)-1] ^= 1
	corruptPath := filepath.Join(t.TempDir(), "corrupt.nfp32")
	if err = os.WriteFile(corruptPath, corrupt, 0600); err != nil {
		t.Fatal(err)
	}
	if err = VerifyFP32(corruptPath); err == nil {
		t.Fatal("checksum")
	}
	mapped, err := LoadMappedFP32(destination)
	if err != nil {
		t.Fatal(err)
	}
	defer mapped.Close()
	original, err := FromBytes(raw)
	if err != nil {
		t.Fatal(err)
	}
	if mapped.Config() != original.Config() || mapped.SourceSHA256() != info.SourceSHA256 || !reflect.DeepEqual(mapped.TensorNames(), original.TensorNames()) {
		t.Fatal(mapped.Config(), mapped.TensorNames())
	}
	for _, name := range original.TensorNames() {
		tensor, _ := original.Tensor(name)
		want := tensor.Float32()
		got, tensorErr := mapped.tensor(name, tensor.shape)
		if tensorErr != nil || len(got) != len(want) {
			t.Fatal(name, tensorErr)
		}
		for index := range got {
			if math.Float32bits(got[index]) != math.Float32bits(want[index]) {
				t.Fatal(name, index)
			}
		}
	}
	if _, err = mapped.tensor("missing", nil); err == nil {
		t.Fatal("missing tensor")
	}
	if _, err = mapped.tensor(mapped.TensorNames()[0], []uint32{99}); err == nil {
		t.Fatal("shape mismatch")
	}
	if err = mapped.Close(); err != nil {
		t.Fatal(err)
	}
	if err = mapped.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestFP32HeaderFailures(t *testing.T) {
	_, _, good := preparedFP32(t)
	headerLength := int(binary.LittleEndian.Uint32(good[12:16]))
	cases := map[string]func([]byte) (int64, []byte){
		"short":   func(_ []byte) (int64, []byte) { return 5, []byte("short") },
		"magic":   func(b []byte) (int64, []byte) { b[0] = 'X'; return int64(len(b)), b },
		"version": func(b []byte) (int64, []byte) { binary.LittleEndian.PutUint32(b[8:12], 2); return int64(len(b)), b },
		"bounds": func(b []byte) (int64, []byte) {
			binary.LittleEndian.PutUint32(b[12:16], fp32MaxHeader+1)
			return int64(len(b)), b
		},
		"config-length": func(b []byte) (int64, []byte) {
			binary.LittleEndian.PutUint32(b[28:32], uint32(headerLength))
			return int64(len(b)), b
		},
		"config-json": func(b []byte) (int64, []byte) { b[fp32FixedSize] = 0xff; return int64(len(b)), b },
		"count": func(b []byte) (int64, []byte) {
			binary.LittleEndian.PutUint32(b[24:28], 999999)
			return int64(len(b)), b
		},
		"name-length": func(b []byte) (int64, []byte) {
			c := fp32FixedSize + int(binary.LittleEndian.Uint32(b[28:32]))
			binary.LittleEndian.PutUint16(b[c:c+2], 0)
			return int64(len(b)), b
		},
		"rank": func(b []byte) (int64, []byte) {
			c := fp32FixedSize + int(binary.LittleEndian.Uint32(b[28:32]))
			binary.LittleEndian.PutUint16(b[c+2:c+4], 17)
			return int64(len(b)), b
		},
		"name-utf8": func(b []byte) (int64, []byte) {
			c := fp32FixedSize + int(binary.LittleEndian.Uint32(b[28:32]))
			b[c+24] = 0xff
			return int64(len(b)), b
		},
		"zero-shape": func(b []byte) (int64, []byte) {
			c := fp32FixedSize + int(binary.LittleEndian.Uint32(b[28:32]))
			n := int(binary.LittleEndian.Uint16(b[c : c+2]))
			binary.LittleEndian.PutUint32(b[c+24+n:c+28+n], 0)
			return int64(len(b)), b
		},
		"bounds-tensor": func(b []byte) (int64, []byte) {
			c := fp32FixedSize + int(binary.LittleEndian.Uint32(b[28:32]))
			binary.LittleEndian.PutUint64(b[c+8:c+16], uint64(len(b)+4))
			return int64(len(b)), b
		},
		"trailing-directory": func(b []byte) (int64, []byte) {
			binary.LittleEndian.PutUint32(b[12:16], uint32(headerLength+1))
			return int64(len(b)), b
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			candidate := append([]byte{}, good...)
			size, candidate := mutate(candidate)
			if _, _, err := parseFP32Header(candidate, size); err == nil {
				t.Fatal("accepted")
			}
		})
	}
}

func TestMappedRouterLifecycle(t *testing.T) {
	path := filepath.Join(t.TempDir(), "router.nfp32")
	model := routerFixture(13)
	if err := prepareFP32Model(model, [32]byte{1}, path, defaultFP32PrepareOps()); err != nil {
		t.Fatal(err)
	}
	router, err := LoadMappedRouter(path)
	if err != nil {
		t.Fatal(err)
	}
	if router.Router == nil || router.model == nil {
		t.Fatal(router)
	}
	if err = router.Close(); err != nil {
		t.Fatal(err)
	}
	badPath := filepath.Join(t.TempDir(), "bad-router.nfp32")
	bad := routerFixture(13)
	delete(bad.tensors, "embedding.embedding")
	if err = prepareFP32Model(bad, [32]byte{2}, badPath, defaultFP32PrepareOps()); err != nil {
		t.Fatal(err)
	}
	if _, err = LoadMappedRouter(badPath); err == nil {
		t.Fatal("bad router")
	}
	if err = (&MappedRouter{model: &MappedFP32Model{}}).Close(); err != nil {
		t.Fatal(err)
	}
}

func TestFP32Failures(t *testing.T) {
	directory := t.TempDir()
	if err := PrepareFP32(filepath.Join(directory, "missing"), filepath.Join(directory, "out")); err == nil {
		t.Fatal("missing accepted")
	}
	badSource := filepath.Join(directory, "bad.ndl")
	if err := os.WriteFile(badSource, []byte("bad"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := PrepareFP32(badSource, filepath.Join(directory, "out")); err == nil {
		t.Fatal("bad accepted")
	}
	for name, raw := range map[string][]byte{"short": []byte("short"), "magic": append([]byte("XXXXXXXX"), make([]byte, fp32FixedSize-8)...)} {
		path := filepath.Join(directory, name)
		if err := os.WriteFile(path, raw, 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := ReadFP32Info(path); err == nil {
			t.Fatal(name)
		}
		if _, err := LoadMappedFP32(path); err == nil {
			t.Fatal("mapped", name)
		}
	}
	if _, err := ReadFP32Info(directory); err == nil {
		t.Fatal("directory")
	}
	if _, err := readFP32InfoWith("ignored", func(string) (fp32ReadFile, error) { return nil, os.ErrPermission }); err == nil {
		t.Fatal("info open")
	}
	if _, err := readFP32InfoWith("ignored", func(string) (fp32ReadFile, error) { return os.Open(directory) }); err == nil {
		t.Fatal("info stat/read")
	}
	header := make([]byte, fp32FixedSize)
	copy(header, fp32Magic)
	binary.LittleEndian.PutUint32(header[8:12], fp32Version)
	binary.LittleEndian.PutUint32(header[12:16], fp32FixedSize+10)
	if _, err := readFP32InfoFrom(bytes.NewReader(header), fp32FixedSize+10); err == nil {
		t.Fatal("header read")
	}
	if err := VerifyFP32(filepath.Join(directory, "missing")); err == nil {
		t.Fatal("verify missing")
	}
	empty := filepath.Join(directory, "empty")
	if err := os.WriteFile(empty, nil, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := mapReadOnly(empty); err == nil {
		t.Fatal("empty map")
	}
	if err := unmapReadOnly(nil); err != nil {
		t.Fatal(err)
	}
	if _, err := mapReadOnlyWith("ignored", func(string) (mmapFile, error) { return nil, os.ErrPermission }); err == nil {
		t.Fatal("open")
	}
	if _, err := mapReadOnlyWith("ignored", func(string) (mmapFile, error) { return os.Open(directory) }); err == nil {
		t.Fatal("stat")
	}
	file, err := os.Open(empty)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if _, err := mapFileReadOnly(file, -1); err == nil {
		t.Fatal("mmap")
	}
	if err := verifyFP32With("ignored", func(string) ([]byte, error) { return nil, os.ErrPermission }); err == nil {
		t.Fatal("verify map")
	}
	if err := verifyFP32With("ignored", func(string) ([]byte, error) { return []byte("bad"), nil }); err == nil {
		t.Fatal("verify header")
	}
	if err := (*MappedRouter)(nil).Close(); err != nil {
		t.Fatal(err)
	}
}
