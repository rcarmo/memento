package needleworker

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/rcarmo/memento/internal/needle"
)

func TestFrameRoundTripAndValidation(t *testing.T) {
	request := Request{Query: "status", Tools: "[]", Options: needle.DefaultGenerationOptions()}
	var raw bytes.Buffer
	if err := WriteFrame(&raw, request); err != nil {
		t.Fatal(err)
	}
	var decoded Request
	if err := ReadFrame(&raw, &decoded, DefaultMaxFrameBytes); err != nil || decoded.Query != request.Query {
		t.Fatal(decoded, err)
	}
	if err := ValidateRequest(decoded); err != nil {
		t.Fatal(err)
	}
	for _, request := range []Request{
		{Options: needle.DefaultGenerationOptions()},
		{Query: "x", Tools: strings.Repeat("x", 300<<10), Options: needle.DefaultGenerationOptions()},
		{Query: "x", Options: needle.GenerationOptions{MaxGenerated: 0, MaxEncoded: 1}},
		{Query: "x", Options: needle.GenerationOptions{MaxGenerated: 1, MaxEncoded: 5000}},
	} {
		if err := ValidateRequest(request); err == nil {
			t.Fatal(request)
		}
	}
}

func TestFrameFailures(t *testing.T) {
	var raw bytes.Buffer
	binary.Write(&raw, binary.LittleEndian, uint32(DefaultMaxFrameBytes+1))
	if err := ReadFrame(&raw, &Request{}, DefaultMaxFrameBytes); err == nil {
		t.Fatal("oversized")
	}
	raw.Reset()
	binary.Write(&raw, binary.LittleEndian, uint32(2))
	raw.WriteString("{")
	if err := ReadFrame(&raw, &Request{}, DefaultMaxFrameBytes); !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatal(err)
	}
	raw.Reset()
	binary.Write(&raw, binary.LittleEndian, uint32(2))
	raw.WriteString("{}")
	if err := ReadFrame(&raw, &Request{}, DefaultMaxFrameBytes); err != nil {
		t.Fatal(err)
	}
	if err := WriteFrame(io.Discard, struct {
		Value chan int `json:"value"`
	}{make(chan int)}); err == nil {
		t.Fatal("marshal")
	}
	if err := WriteFrame(io.Discard, strings.Repeat("x", DefaultMaxFrameBytes+1)); err == nil {
		t.Fatal("large")
	}
	if err := WriteFrame(badWriter{}, Response{OK: true}); err == nil {
		t.Fatal("write")
	}
	if err := WriteFrame(&stagedWriter{remaining: 1}, Response{OK: true}); err == nil {
		t.Fatal("body write")
	}
	if err := WriteFrame(shortWriter{}, Response{OK: true}); !errors.Is(err, io.ErrShortWrite) {
		t.Fatal(err)
	}
	buffer := bufio.NewWriterSize(badWriter{}, 1)
	if err := WriteFrame(buffer, Response{OK: true}); err == nil {
		t.Fatal("flush")
	}
	for _, wire := range [][]byte{{0, 0, 0, 0}, {3, 0, 0, 0, '{', '}', 'x'}, {14, 0, 0, 0, '{', '"', 'u', 'n', 'k', 'n', 'o', 'w', 'n', '"', ':', '1', '}'}} {
		var request Request
		if err := ReadFrame(bytes.NewReader(wire), &request, 0); err == nil {
			t.Fatal(wire)
		}
	}
}

func TestServeBoundaries(t *testing.T) {
	if err := Serve(bytes.NewReader(nil), io.Discard, nil, DefaultMaxFrameBytes); err != nil {
		t.Fatal(err)
	}
	var invalid bytes.Buffer
	if err := WriteFrame(&invalid, Request{Options: needle.DefaultGenerationOptions()}); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := Serve(&invalid, &output, nil, DefaultMaxFrameBytes); err != nil {
		t.Fatal(err)
	}
	var response Response
	if err := ReadFrame(&output, &response, DefaultMaxFrameBytes); err != nil || response.Error == "" {
		t.Fatal(response, err)
	}
	oversized := bytes.NewReader([]byte{0xff, 0xff, 0xff, 0x7f})
	if err := Serve(oversized, io.Discard, nil, DefaultMaxFrameBytes); err == nil {
		t.Fatal("oversized")
	}
	var valid bytes.Buffer
	request := Request{Query: "query", Tools: "[]", Options: needle.DefaultGenerationOptions()}
	_ = WriteFrame(&valid, request)
	output.Reset()
	if err := Serve(&valid, &output, func(query, tools string, options needle.GenerationOptions) (string, error) { return "result", nil }, DefaultMaxFrameBytes); err != nil {
		t.Fatal(err)
	}
	if err := ReadFrame(&output, &response, DefaultMaxFrameBytes); err != nil || !response.OK || response.Output != "result" {
		t.Fatal(response, err)
	}
	valid.Reset()
	_ = WriteFrame(&valid, request)
	output.Reset()
	if err := Serve(&valid, &output, func(string, string, needle.GenerationOptions) (string, error) { return "", errors.New("generate") }, DefaultMaxFrameBytes); err != nil {
		t.Fatal(err)
	}
	if err := ReadFrame(&output, &response, DefaultMaxFrameBytes); err != nil || response.Error != "generate" {
		t.Fatal(response, err)
	}
	valid.Reset()
	_ = WriteFrame(&valid, request)
	if err := Serve(&valid, badWriter{}, func(string, string, needle.GenerationOptions) (string, error) { return "result", nil }, DefaultMaxFrameBytes); err == nil {
		t.Fatal("success write")
	}
	valid.Reset()
	_ = WriteFrame(&valid, request)
	if err := Serve(&valid, badWriter{}, func(string, string, needle.GenerationOptions) (string, error) { return "", errors.New("generate") }, DefaultMaxFrameBytes); err == nil {
		t.Fatal("error write")
	}
	invalidJSON := bytes.NewReader([]byte{1, 0, 0, 0, '{'})
	if err := Serve(invalidJSON, io.Discard, nil, DefaultMaxFrameBytes); err == nil {
		t.Fatal("json")
	}
}

type badWriter struct{}

func (badWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

type shortWriter struct{}

func (shortWriter) Write([]byte) (int, error) { return 0, nil }

type stagedWriter struct{ remaining int }

func (w *stagedWriter) Write(p []byte) (int, error) {
	if w.remaining == 0 {
		return 0, io.ErrClosedPipe
	}
	w.remaining--
	return len(p), nil
}
