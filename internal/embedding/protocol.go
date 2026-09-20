// Package embedding implements the framed GTE worker interface used by Memento.
package embedding

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"strings"

	"github.com/rcarmo/memento/internal/gte"
	"github.com/rcarmo/memento/internal/vector"
)

// Request represents the three source methods. Unknown fields are ignored.
type Request struct {
	Method string   `json:"method"`
	ID     *string  `json:"id"`
	Text   string   `json:"text"`
	Texts  []string `json:"texts"`
}

// Header uses explicit JSON nulls and preserves the Rust field ordering.
type Header struct {
	ID            *string `json:"id"`
	OK            bool    `json:"ok"`
	Method        string  `json:"method"`
	Dimensions    *int    `json:"dimensions"`
	Count         *int    `json:"count"`
	PayloadLength int     `json:"payload_len"`
	Error         *string `json:"error"`
}

// Frame owns its response header and little-endian float32 payload.
type Frame struct {
	Header  Header
	Payload []byte
}

// ReadRequest decodes one request; maxBytes is an explicit caller limit. Zero
// preserves the source's u32 limit. Truncated headers/bodies return EOF-family
// errors; callers must stop, not replay already-consumed framing bytes.
func ReadRequest(input io.Reader, maxBytes uint32) (Request, error) {
	var size [4]byte
	if _, err := io.ReadFull(input, size[:]); err != nil {
		return Request{}, err
	}
	length := binary.LittleEndian.Uint32(size[:])
	if maxBytes > 0 && length > maxBytes {
		return Request{}, fmt.Errorf("frame too large: %d", length)
	}
	// Read incrementally: a short stream with a huge length cannot force a huge
	// allocation before the bytes arrive. The actual command supplies a limit.
	raw, err := io.ReadAll(io.LimitReader(input, int64(length)))
	if err != nil {
		return Request{}, err
	}
	if uint32(len(raw)) != length {
		return Request{}, io.ErrUnexpectedEOF
	}
	return ParseRequest(raw)
}

// ParseRequest validates required values without Go's null-to-zero coercion.
// Invalid JSON diagnostics are Go-specific for now; exact malformed serde error
// text remains a documented compatibility gate.
func ParseRequest(raw []byte) (Request, error) {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil {
		return Request{}, fmt.Errorf("json error: %w", err)
	}
	var r Request
	method, ok := object["method"]
	if !ok {
		return r, errors.New("json error: missing field `method`")
	}
	if string(method) == "null" {
		return r, errors.New("json error: method must be a string")
	}
	if err := json.Unmarshal(method, &r.Method); err != nil {
		return r, fmt.Errorf("json error: %w", err)
	}
	if rawID, ok := object["id"]; ok {
		if err := json.Unmarshal(rawID, &r.ID); err != nil {
			return r, fmt.Errorf("json error: %w", err)
		}
	}
	switch r.Method {
	case "info":
	case "embed":
		value, ok := object["text"]
		if !ok {
			return r, errors.New("json error: missing field `text`")
		}
		if string(value) == "null" {
			return r, errors.New("json error: text must be a string")
		}
		if err := json.Unmarshal(value, &r.Text); err != nil {
			return r, fmt.Errorf("json error: %w", err)
		}
	case "embed_batch":
		value, ok := object["texts"]
		if !ok {
			return r, errors.New("json error: missing field `texts`")
		}
		var texts []json.RawMessage
		if string(value) == "null" {
			return r, errors.New("json error: texts must be an array")
		}
		if err := json.Unmarshal(value, &texts); err != nil {
			return r, fmt.Errorf("json error: %w", err)
		}
		r.Texts = make([]string, len(texts))
		for i, v := range texts {
			if string(v) == "null" {
				return r, errors.New("json error: text must be a string")
			}
			if err := json.Unmarshal(v, &r.Texts[i]); err != nil {
				return r, fmt.Errorf("json error: %w", err)
			}
		}
	default:
		return r, fmt.Errorf("json error: unknown variant `%s`, expected one of `info`, `embed`, `embed_batch`", r.Method)
	}
	return r, nil
}

// Handle calls the real scalar GTE implementation and checks output finiteness.
func Handle(model *gte.Model, request Request) (Frame, error) {
	dimension := model.Dim()
	count := 0
	var payload []byte
	switch request.Method {
	case "info":
	case "embed":
		values, err := model.Embed(request.Text)
		if err != nil {
			return Frame{}, fmt.Errorf("gte error: %w", err)
		}
		payload, err = vector.EncodeF32LE(values)
		if err != nil {
			return Frame{}, err
		}
		count = 1
	case "embed_batch":
		embeddings, err := model.EmbedBatch(request.Texts, gte.BatchOptions{}, nil)
		if err != nil {
			return Frame{}, fmt.Errorf("gte error: %w", err)
		}
		count = len(embeddings)
		for _, values := range embeddings {
			raw, err := vector.EncodeF32LE(values)
			if err != nil {
				return Frame{}, err
			}
			payload = append(payload, raw...)
		}
	default:
		return Frame{}, fmt.Errorf("invalid method: %s", request.Method)
	}
	return Frame{Header: Header{ID: request.ID, OK: true, Method: request.Method, Dimensions: &dimension, Count: &count, PayloadLength: len(payload)}, Payload: payload}, nil
}

// WriteFrame writes the source u32 total/header lengths and f32le payload. It
// flushes explicit buffered writers just as the source flushes after each reply.
func WriteFrame(output io.Writer, frame Frame) error {
	return WriteResponse(output, frame.Header, bytes.NewReader(frame.Payload), uint64(len(frame.Payload)))
}

// WriteResponse writes a bounded response with a streamed payload, allowing
// length validation before any allocation for an oversized result.
func WriteResponse(output io.Writer, response Header, payload io.Reader, payloadLength uint64) error {
	// Header contains only JSON-safe primitive values, so marshaling cannot fail.
	header, _ := json.Marshal(response)
	total, err := frameSize(uint64(len(header)), payloadLength)
	if err != nil {
		return err
	}
	var prefix [8]byte
	binary.LittleEndian.PutUint32(prefix[:4], uint32(total))
	binary.LittleEndian.PutUint32(prefix[4:], uint32(len(header)))
	for _, part := range [][]byte{prefix[:], header} {
		for len(part) > 0 {
			n, err := output.Write(part)
			if err != nil {
				return err
			}
			if n <= 0 || n > len(part) {
				return io.ErrShortWrite
			}
			part = part[n:]
		}
	}
	if _, err := io.CopyN(output, payload, int64(payloadLength)); err != nil {
		return err
	}
	if f, ok := output.(interface{ Flush() error }); ok {
		return f.Flush()
	}
	return nil
}

// ErrorFrame reproduces the command's method=error/id=null failure envelope.
func ErrorFrame(err error) Frame {
	message := err.Error()
	return Frame{Header: Header{Method: "error", Error: &message}}
}

// Serve handles multiple frames on the same stream. A rejected oversized frame
// ends the stream after the error response because it has not consumed its body.
func Serve(input io.Reader, output io.Writer, model *gte.Model, maxBytes uint32) error {
	for {
		request, err := ReadRequest(input, maxBytes)
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			return nil
		}
		if err != nil {
			if writeErr := WriteFrame(output, ErrorFrame(err)); writeErr != nil {
				return writeErr
			}
			if !strings.HasPrefix(err.Error(), "json error: ") {
				return err
			}
			continue
		}
		frame, err := Handle(model, request)
		if err != nil {
			frame = ErrorFrame(err)
		}
		if err = WriteFrame(output, frame); err != nil {
			return err
		}
	}
}
func frameSize(header, payload uint64) (uint64, error) {
	if header > math.MaxUint32-4 || payload > math.MaxUint32-4-header {
		return 0, fmt.Errorf("frame too large")
	}
	return 4 + header + payload, nil
}
