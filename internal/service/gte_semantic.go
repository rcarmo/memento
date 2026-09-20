package service

import (
	"crypto/sha256"
	"fmt"
	"os"

	"github.com/rcarmo/memento/internal/derived"
	"github.com/rcarmo/memento/internal/gte"
)

type gteSemanticModel interface {
	Dim() int
	Embed(string) ([]float32, error)
	EmbedBatch([]string, gte.BatchOptions, gte.Checkpoint) ([][]float32, error)
}
type GTESemanticClient struct {
	Model                   gteSemanticModel
	Info                    derived.SemanticModelInfo
	MaxBatch, MaxInputBytes int
}

func LoadGTESemanticClient(path, modelID string, dimensions, maxBatch, maxInput int) (*GTESemanticClient, error) {
	return loadConfiguredGTESemanticClient(path, modelID, dimensions, maxBatch, maxInput, os.ReadFile, func(raw []byte) (gteSemanticModel, error) { return gte.FromBytes(raw) }, os.LookupEnv)
}
func loadConfiguredGTESemanticClient(path, modelID string, dimensions, maxBatch, maxInput int, read func(string) ([]byte, error), decode func([]byte) (gteSemanticModel, error), lookup func(string) (string, bool)) (*GTESemanticClient, error) {
	client, err := loadGTESemanticClient(path, modelID, dimensions, maxBatch, maxInput, read, decode)
	if err != nil {
		return nil, err
	}
	if value, ok := lookup("MEMENTO_SIMD"); ok && value != "" {
		model, supported := client.Model.(interface{ SetSIMD(string) error })
		if !supported {
			return nil, fmt.Errorf("embedding model does not support SIMD configuration")
		}
		if err = model.SetSIMD(value); err != nil {
			return nil, err
		}
	}
	return client, nil
}
func loadGTESemanticClient(path, modelID string, dimensions, maxBatch, maxInput int, read func(string) ([]byte, error), decode func([]byte) (gteSemanticModel, error)) (*GTESemanticClient, error) {
	raw, err := read(path)
	if err != nil {
		return nil, err
	}
	model, err := decode(raw)
	if err != nil {
		return nil, err
	}
	if model.Dim() != dimensions {
		return nil, fmt.Errorf("embedding model dimension mismatch: got %d, expected %d", model.Dim(), dimensions)
	}
	digest := fmt.Sprintf("%x", sha256.Sum256(raw))
	return &GTESemanticClient{Model: model, Info: derived.SemanticModelInfo{ModelID: modelID, Dimensions: dimensions, Revision: digest}, MaxBatch: maxBatch, MaxInputBytes: maxInput}, nil
}
func (c *GTESemanticClient) ModelInfo() derived.SemanticModelInfo { return c.Info }
func (c *GTESemanticClient) Embed(text string) ([]float32, error) {
	if c == nil || c.Model == nil {
		return nil, fmt.Errorf("embedding model is unavailable")
	}
	if c.MaxInputBytes > 0 && len(text) > c.MaxInputBytes {
		return nil, fmt.Errorf("input too large: %d chars > %d", len(text), c.MaxInputBytes)
	}
	return c.Model.Embed(text)
}
func (c *GTESemanticClient) EmbedBatch(texts []string) ([][]float32, error) {
	if c == nil || c.Model == nil {
		return nil, fmt.Errorf("embedding model is unavailable")
	}
	maxBatch, maxInput := c.MaxBatch, c.MaxInputBytes
	return c.Model.EmbedBatch(texts, gte.BatchOptions{MaxBatch: &maxBatch, MaxInputBytes: &maxInput}, nil)
}
