package derived

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"strings"
)

type SemanticModelInfo struct {
	ModelID    string
	Dimensions int
	Revision   string
}
type SemanticClient interface {
	ModelInfo() SemanticModelInfo
	Embed(string) ([]float32, error)
}
type SemanticBatchClient interface {
	SemanticClient
	EmbedBatch([]string) ([][]float32, error)
}
type SemanticRefreshConfig struct {
	ModelID                             string
	Dimensions, MaxInputChars, MaxBatch int
	BeforeBatch                         func(context.Context) error // progressive admission between document chunks
}

func embeddingText(title string, description *string, body string) string {
	parts := []string{strings.TrimSpace(title)}
	if description != nil && *description != "" {
		parts = append(parts, strings.TrimSpace(*description))
	}
	if text := strings.TrimSpace(body); text != "" {
		parts = append(parts, text)
	}
	out := []string{}
	for _, part := range parts {
		if part != "" {
			out = append(out, part)
		}
	}
	return strings.Join(out, "\n\n")
}
func validateSemanticVector(values []float32, dimensions int) ([]byte, float64, error) {
	if len(values) != dimensions {
		return nil, 0, fmt.Errorf("embedding dimension mismatch: got %d, expected %d", len(values), dimensions)
	}
	blob := make([]byte, len(values)*4)
	norm := 0.0
	for index, value := range values {
		if math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) {
			return nil, 0, fmt.Errorf("embedding contains non-finite value at index %d", index)
		}
		norm += float64(value) * float64(value)
		binary.LittleEndian.PutUint32(blob[index*4:], math.Float32bits(value))
	}
	norm = math.Sqrt(norm)
	if math.IsNaN(norm) || math.IsInf(norm, 0) || norm <= 0 {
		return nil, 0, errors.New("embedding has zero or invalid norm")
	}
	return blob, norm, nil
}

func (i *Index) RefreshEmbeddingPaths(ctx context.Context, revision string, paths []string, config SemanticRefreshConfig, client SemanticClient) error {
	if client == nil {
		return errors.New("semantic embedding client is unavailable")
	}
	info := client.ModelInfo()
	if info.ModelID != config.ModelID || info.Dimensions != config.Dimensions {
		return errors.New("semantic embedding model metadata mismatch")
	}
	chunked, ok := client.(SemanticChunkClient)
	if !ok {
		return errors.New("semantic embedding client does not support chunking")
	}
	i.mu.Lock()
	i.chunkModel = info
	i.MaxInputChars = config.MaxInputChars
	i.mu.Unlock()
	return i.refreshChunks(ctx, revision, paths, config, chunked)
}
