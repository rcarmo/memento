package service

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"unicode/utf8"

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
	MaxBatch, MaxInputChars int
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
	return &GTESemanticClient{Model: model, Info: derived.SemanticModelInfo{ModelID: modelID, Dimensions: dimensions, Revision: digest}, MaxBatch: maxBatch, MaxInputChars: maxInput}, nil
}
func (c *GTESemanticClient) Chunk(text string, tokens, overlap, chars int) ([]string, error) {
	model, ok := c.Model.(interface {
		Chunk(string, int, int, int) ([]string, error)
	})
	if !ok {
		return nil, fmt.Errorf("embedding model does not support chunking")
	}
	return model.Chunk(text, tokens, overlap, chars)
}

func (c *GTESemanticClient) ModelInfo() derived.SemanticModelInfo { return c.Info }
func (c *GTESemanticClient) Embed(text string) ([]float32, error) {
	if c == nil || c.Model == nil {
		return nil, fmt.Errorf("embedding model is unavailable")
	}
	if c.MaxInputChars > 0 && utf8.RuneCountInString(text) > c.MaxInputChars {
		return nil, fmt.Errorf("input too large: %d chars > %d", utf8.RuneCountInString(text), c.MaxInputChars)
	}
	return c.Model.Embed(text)
}
func (c *GTESemanticClient) EmbedBatch(texts []string) ([][]float32, error) {
	return c.EmbedBatchContext(context.Background(), texts)
}
func (c *GTESemanticClient) EmbedBatchContext(ctx context.Context, texts []string) ([][]float32, error) {
	if c == nil || c.Model == nil {
		return nil, fmt.Errorf("embedding model is unavailable")
	}
	for _, text := range texts {
		if c.MaxInputChars > 0 && utf8.RuneCountInString(text) > c.MaxInputChars {
			return nil, fmt.Errorf("input too large: %d chars > %d", utf8.RuneCountInString(text), c.MaxInputChars)
		}
	}
	maxBatch := c.MaxBatch
	// Service limits are Unicode characters; the low-level GTE API retains its
	// historical byte-limit option, which must not be reused for this setting.
	return c.Model.EmbedBatch(texts, gte.BatchOptions{MaxBatch: &maxBatch}, func(string) error { return ctx.Err() })
}
