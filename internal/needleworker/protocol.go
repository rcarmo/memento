// Package needleworker implements the bounded framed protocol used by the
// short-lived pure-Go Needle routing worker.
package needleworker

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/rcarmo/memento/internal/needle"
)

const DefaultMaxFrameBytes = 1 << 20

type Request struct {
	Query   string                   `json:"query"`
	Tools   string                   `json:"tools"`
	Options needle.GenerationOptions `json:"options"`
}
type Response struct {
	OK     bool   `json:"ok"`
	Output string `json:"output,omitempty"`
	Error  string `json:"error,omitempty"`
}

func WriteFrame(output io.Writer, value any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if len(raw) > DefaultMaxFrameBytes {
		return errors.New("Needle frame exceeds limit")
	}
	var size [4]byte
	binary.LittleEndian.PutUint32(size[:], uint32(len(raw)))
	if _, err = output.Write(size[:]); err != nil {
		return err
	}
	for len(raw) > 0 {
		n, writeErr := output.Write(raw)
		if writeErr != nil {
			return writeErr
		}
		if n <= 0 || n > len(raw) {
			return io.ErrShortWrite
		}
		raw = raw[n:]
	}
	if flush, ok := output.(interface{ Flush() error }); ok {
		return flush.Flush()
	}
	return nil
}
func ReadFrame(input io.Reader, value any, maxBytes uint32) error {
	var size [4]byte
	if _, err := io.ReadFull(input, size[:]); err != nil {
		return err
	}
	length := binary.LittleEndian.Uint32(size[:])
	if maxBytes == 0 {
		maxBytes = DefaultMaxFrameBytes
	}
	if length == 0 || length > maxBytes {
		return fmt.Errorf("invalid Needle frame size: %d", length)
	}
	raw := make([]byte, length)
	if _, err := io.ReadFull(input, raw); err != nil {
		return err
	}
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return fmt.Errorf("invalid Needle frame: %w", err)
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		return errors.New("invalid Needle frame: trailing JSON")
	}
	return nil
}

func ValidateRequest(request Request) error {
	if strings.TrimSpace(request.Query) == "" {
		return errors.New("Needle query must not be empty")
	}
	if len(request.Query) > 4096 || len(request.Tools) > 256<<10 {
		return errors.New("Needle request exceeds limit")
	}
	if request.Options.MaxGenerated < 1 || request.Options.MaxGenerated > 4096 || request.Options.MaxEncoded < 1 || request.Options.MaxEncoded > 4096 {
		return errors.New("Needle generation options are out of range")
	}
	return nil
}

type GenerateFunc func(string, string, needle.GenerationOptions) (string, error)

func Serve(input io.Reader, output io.Writer, generate GenerateFunc, maxBytes uint32) error {
	writer := bufio.NewWriter(output)
	for {
		var request Request
		err := ReadFrame(input, &request, maxBytes)
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			return nil
		}
		if err != nil {
			_ = WriteFrame(writer, Response{Error: err.Error()})
			return err
		}
		if err = ValidateRequest(request); err == nil {
			var result string
			result, err = generate(request.Query, request.Tools, request.Options)
			if err == nil {
				if writeErr := WriteFrame(writer, Response{OK: true, Output: result}); writeErr != nil {
					return writeErr
				}
				continue
			}
		}
		if writeErr := WriteFrame(writer, Response{Error: err.Error()}); writeErr != nil {
			return writeErr
		}
	}
}
