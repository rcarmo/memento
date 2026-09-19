package derived

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
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
type SemanticRefreshConfig struct {
	ModelID                   string
	Dimensions, MaxInputChars int
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
	return i.withCore(ctx, true, func(s ContentStore) error {
		for _, path := range paths {
			var id, title, body string
			var description *string
			if err := s.DB.QueryRowContext(ctx, "SELECT id,title,description,body FROM concepts WHERE path=?", path).Scan(&id, &title, &description, &body); err != nil {
				return fmt.Errorf("embedding refresh path is unavailable: %s", path)
			}
			text := []rune(embeddingText(title, description, body))
			limit := config.MaxInputChars
			if limit <= 0 {
				limit = 4096
			}
			if len(text) > limit {
				text = text[:limit]
			}
			content := string(text)
			digest := fmt.Sprintf("%x", sha256.Sum256([]byte(content)))
			vector, embedErr := client.Embed(content)
			now := time.Now().UTC().Truncate(time.Second).Format(time.RFC3339)
			if embedErr != nil {
				if _, err := s.DB.ExecContext(ctx, `INSERT INTO concept_embeddings(concept_id,path,embedding_text_hash,model_id,dimensions,embedding_revision,status,model_revision,embedding_blob,embedding_norm,updated_at,error_message) VALUES(?,?,?,?,?,?,'error',?,NULL,NULL,?,?) ON CONFLICT(concept_id) DO UPDATE SET path=excluded.path,embedding_text_hash=excluded.embedding_text_hash,model_id=excluded.model_id,dimensions=excluded.dimensions,embedding_revision=excluded.embedding_revision,status='error',model_revision=excluded.model_revision,embedding_blob=NULL,embedding_norm=NULL,updated_at=excluded.updated_at,error_message=excluded.error_message`, id, path, digest, config.ModelID, config.Dimensions, revision, info.Revision, now, embedErr.Error()); err != nil {
					return err
				}
				continue
			}
			blob, norm, validationErr := validateSemanticVector(vector, config.Dimensions)
			if validationErr != nil {
				return validationErr
			}
			if _, err := s.DB.ExecContext(ctx, `INSERT INTO concept_embeddings(concept_id,path,embedding_text_hash,model_id,dimensions,embedding_revision,status,model_revision,embedding_blob,embedding_norm,updated_at,error_message) VALUES(?,?,?,?,?,?,'ready',?,?,?,?,NULL) ON CONFLICT(concept_id) DO UPDATE SET path=excluded.path,embedding_text_hash=excluded.embedding_text_hash,model_id=excluded.model_id,dimensions=excluded.dimensions,embedding_revision=excluded.embedding_revision,status='ready',model_revision=excluded.model_revision,embedding_blob=excluded.embedding_blob,embedding_norm=excluded.embedding_norm,updated_at=excluded.updated_at,error_message=NULL`, id, path, digest, config.ModelID, config.Dimensions, revision, info.Revision, blob, norm, now); err != nil {
				return err
			}
		}
		_, err := s.DB.ExecContext(ctx, "UPDATE concept_embeddings SET embedding_revision=? WHERE status='ready'", revision)
		if err != nil {
			return err
		}
		return setEmbeddingRevision(ctx, s.DB, revision)
	})
}
