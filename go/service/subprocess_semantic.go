package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"time"

	"github.com/rcarmo/memento/go/derived"
	"github.com/rcarmo/memento/go/embedding"
)

type subprocessSemanticClient struct {
	WorkerPath, ModelPath   string
	Info                    derived.SemanticModelInfo
	MaxBatch, MaxInputBytes int
	Timeout                 time.Duration
	command                 func(context.Context, string, ...string) *exec.Cmd
	run                     func(context.Context, []byte) ([]byte, []byte, error)
}

func LoadSubprocessSemanticClient(config SemanticSearchConfig) (*subprocessSemanticClient, error) {
	if config.ModelPath == nil {
		return nil, errors.New("semantic model path is required")
	}
	revision, err := fileSHA256(*config.ModelPath)
	if err != nil {
		return nil, err
	}
	return &subprocessSemanticClient{
		WorkerPath: config.WorkerPath,
		ModelPath:  *config.ModelPath,
		Info: derived.SemanticModelInfo{
			ModelID:    config.ModelID,
			Dimensions: config.Dimensions,
			Revision:   revision,
		},
		MaxBatch:      config.MaxBatchSize,
		MaxInputBytes: config.MaxInputChars,
		Timeout:       time.Duration(config.WorkerTimeoutSeconds * float64(time.Second)),
		command:       exec.CommandContext,
	}, nil
}
func fileSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	return readerSHA256(file)
}
func readerSHA256(reader io.Reader) (string, error) {
	digest := sha256.New()
	if _, err := io.Copy(digest, reader); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", digest.Sum(nil)), nil
}
func (c *subprocessSemanticClient) ModelInfo() derived.SemanticModelInfo { return c.Info }
func (c *subprocessSemanticClient) Embed(text string) ([]float32, error) {
	values, err := c.EmbedBatch([]string{text})
	if err != nil {
		return nil, err
	}
	return values[0], nil
}
func (c *subprocessSemanticClient) EmbedBatch(texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return [][]float32{}, nil
	}
	if len(texts) > c.MaxBatch {
		return nil, fmt.Errorf("embedding batch has %d items; maximum is %d", len(texts), c.MaxBatch)
	}
	for _, text := range texts {
		if len(text) > c.MaxInputBytes {
			return nil, errors.New("embedding input exceeds configured character limit")
		}
	}
	request := embedding.Request{Method: "embed_batch", Texts: texts}
	raw, _ := json.Marshal(request)
	wire := make([]byte, 4+len(raw))
	binary.LittleEndian.PutUint32(wire, uint32(len(raw)))
	copy(wire[4:], raw)
	ctx, cancel := context.WithTimeout(context.Background(), c.Timeout)
	defer cancel()
	var stdout, stderr []byte
	var err error
	if c.run != nil {
		stdout, stderr, err = c.run(ctx, wire)
	} else {
		command := c.command(ctx, c.WorkerPath, c.ModelPath)
		command.Stdin = bytes.NewReader(wire)
		var output, errors bytes.Buffer
		command.Stdout, command.Stderr = &output, &errors
		err = command.Run()
		stdout, stderr = output.Bytes(), errors.Bytes()
	}
	if err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("embedding worker failed: %w", ctx.Err())
		}
		return nil, fmt.Errorf("embedding worker failed: %w: %s", err, stderr)
	}
	return decodeEmbeddingResponse(stdout, len(texts), c.Info.Dimensions)
}
func decodeEmbeddingResponse(wire []byte, count, dimensions int) ([][]float32, error) {
	if len(wire) < 8 {
		return nil, errors.New("embedding worker returned a truncated frame")
	}
	total, headerLength := int(binary.LittleEndian.Uint32(wire)), int(binary.LittleEndian.Uint32(wire[4:]))
	if total+4 != len(wire) || headerLength > total-4 {
		return nil, errors.New("embedding worker returned an invalid frame")
	}
	var header embedding.Header
	if err := json.Unmarshal(wire[8:8+headerLength], &header); err != nil {
		return nil, errors.New("embedding worker returned invalid JSON")
	}
	if !header.OK || header.Dimensions == nil || header.Count == nil || *header.Dimensions != dimensions || *header.Count != count {
		if header.Error != nil {
			return nil, errors.New(*header.Error)
		}
		return nil, errors.New("embedding worker shape mismatch")
	}
	payload := wire[8+headerLength:]
	return decodeEmbeddingPayload(payload, header.PayloadLength, count, dimensions)
}
func decodeEmbeddingPayload(payload []byte, headerLength, count, dimensions int) ([][]float32, error) {
	if err := validatePayloadLength(headerLength, len(payload), count, dimensions); err != nil {
		return nil, err
	}
	result := make([][]float32, count)
	for row := range result {
		result[row] = make([]float32, dimensions)
		for index := range result[row] {
			offset := (row*dimensions + index) * 4
			value := math.Float32frombits(binary.LittleEndian.Uint32(payload[offset:]))
			if !finiteFloat32(value) {
				return nil, errors.New("embedding worker returned non-finite value")
			}
			result[row][index] = value
		}
	}
	return result, nil
}
func validatePayloadLength(headerLength, actual, count, dimensions int) error {
	if headerLength != actual || actual != count*dimensions*4 {
		return errors.New("embedding worker payload length mismatch")
	}
	return nil
}
func finiteFloat32(value float32) bool {
	return !math.IsNaN(float64(value)) && !math.IsInf(float64(value), 0)
}
