package needle

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"math"
	"os"
	"reflect"
	"testing"
)

func TestRealNeedleModel(t *testing.T) {
	path := os.Getenv("NEEDLE_MODEL_PATH")
	if path == "" {
		t.Skip("set NEEDLE_MODEL_PATH for required model-parity CI")
	}
	raw, err := os.ReadFile("../testdata/parity/needle-real.json")
	if err != nil {
		t.Fatal(err)
	}
	var reference struct {
		ModelSHA string `json:"model_sha256"`
		Config   Config
		Metadata json.RawMessage
		Pieces   []Piece
		Tensors  []struct {
			Name     string
			Shape    []uint32
			Elements int
			RawSHA   string `json:"raw_sha256"`
			F32SHA   string `json:"float32_sha256"`
		}
	}
	if err = json.Unmarshal(raw, &reference); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(data)
	if hex.EncodeToString(hash[:]) != reference.ModelSHA {
		t.Fatal("model SHA mismatch")
	}
	m, err := FromBytes(data)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(m.Config(), reference.Config) || !reflect.DeepEqual(m.Pieces(), reference.Pieces) {
		t.Fatal("config/vocabulary differs from Rust")
	}
	var gotMeta, wantMeta any
	_ = json.Unmarshal(m.Metadata(), &gotMeta)
	_ = json.Unmarshal(reference.Metadata, &wantMeta)
	if !reflect.DeepEqual(gotMeta, wantMeta) {
		t.Fatal("metadata differs")
	}
	names := m.TensorNames()
	if len(names) != len(reference.Tensors) {
		t.Fatal("tensor count mismatch")
	}
	for i, expected := range reference.Tensors {
		tensor, ok := m.Tensor(expected.Name)
		if !ok || names[i] != expected.Name || !reflect.DeepEqual(tensor.Shape(), expected.Shape) {
			t.Fatal(expected.Name)
		}
		rawHash := sha256.Sum256(tensor.RawBF16())
		if hex.EncodeToString(rawHash[:]) != expected.RawSHA {
			t.Fatal("raw tensor mismatch", expected.Name)
		}
		values := tensor.Float32()
		if len(values) != expected.Elements {
			t.Fatal("element count mismatch")
		}
		encoded := make([]byte, len(values)*4)
		for i, v := range values {
			binary.LittleEndian.PutUint32(encoded[i*4:], math.Float32bits(v))
		}
		hash := sha256.Sum256(encoded)
		if hex.EncodeToString(hash[:]) != expected.F32SHA {
			t.Fatal("BF16 expansion differs", expected.Name)
		}
	}
	t.Logf("Verified %d tensors, %d tokenizer pieces and all BF16->float32 hashes against Rust", len(names), len(m.Pieces()))
}
