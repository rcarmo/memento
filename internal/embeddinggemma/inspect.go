package embeddinggemma

import (
	"encoding/json"
	"errors"
	"io"
)

// Inspect reports only architecture/numeric metadata and tensor descriptors.
// This foundation command intentionally does not claim to produce embeddings.
func Inspect(w io.Writer, path string) error {
	if w == nil {
		return errors.New("embeddinggemma: nil output")
	}
	f, err := Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	meta := f.Metadata()
	for k := range meta {
		if len(k) >= 10 && k[:10] == "tokenizer." {
			delete(meta, k)
		}
	}
	summary := struct {
		Metadata map[string]any `json:"metadata"`
		Tensors  []Tensor       `json:"tensors"`
		Status   string         `json:"status"`
	}{meta, f.Tensors(), "quantized-kernel-foundation; tokenizer/encoder not yet implemented"}
	e := json.NewEncoder(w)
	e.SetIndent("", "  ")
	return e.Encode(summary)
}
