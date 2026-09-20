package derived

import "errors"

type SemanticEmbeddingResult struct {
	Vector []float32
	Err    error
}

func EmbedSemanticBatch(client SemanticClient, texts []string, maxBatch int) []SemanticEmbeddingResult {
	if maxBatch <= 0 {
		maxBatch = 16
	}
	results := make([]SemanticEmbeddingResult, 0, len(texts))
	for start := 0; start < len(texts); start += maxBatch {
		end := min(len(texts), start+maxBatch)
		batch := texts[start:end]
		if batched, ok := client.(SemanticBatchClient); ok {
			vectors, err := batched.EmbedBatch(batch)
			if err == nil && len(vectors) == len(batch) {
				for _, vector := range vectors {
					results = append(results, SemanticEmbeddingResult{Vector: vector})
				}
				continue
			}
			if err == nil {
				err = errors.New("embedding client returned mismatched batch length")
			}
			if len(batch) == 1 {
				results = append(results, SemanticEmbeddingResult{Err: err})
				continue
			}
		}
		for _, text := range batch {
			vector, err := client.Embed(text)
			results = append(results, SemanticEmbeddingResult{Vector: vector, Err: err})
		}
	}
	return results
}
