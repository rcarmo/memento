package embedding

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/rcarmo/memento/go/gte"
)

func modelBytes() []byte {
	var b bytes.Buffer
	b.WriteString("GTE1")
	for _, n := range []uint32{104, 2, 0, 1, 2, 4} {
		_ = binary.Write(&b, binary.LittleEndian, n)
	}
	for i := 0; i < 104; i++ {
		word := fmt.Sprintf("t%d", i)
		_ = binary.Write(&b, binary.LittleEndian, uint16(len(word)))
		b.WriteString(word)
	}
	for i := 0; i < 104*2+4*2+2*2+2+2+2*2+2; i++ {
		_ = binary.Write(&b, binary.LittleEndian, float32(i%7)/10)
	}
	return b.Bytes()
}
func testModel(t *testing.T) *gte.Model {
	t.Helper()
	m, err := gte.FromBytes(modelBytes())
	if err != nil {
		t.Fatal(err)
	}
	return m
}
func requestBytes(text string) []byte {
	out := binary.LittleEndian.AppendUint32(nil, uint32(len(text)))
	return append(out, text...)
}
func decodeFrames(t *testing.T, data []byte) []Frame {
	t.Helper()
	out := []Frame{}
	for len(data) > 0 {
		if len(data) < 8 {
			t.Fatal("short frame")
		}
		total, header := int(binary.LittleEndian.Uint32(data)), int(binary.LittleEndian.Uint32(data[4:]))
		if total+4 > len(data) || header > total-4 {
			t.Fatal("frame bounds")
		}
		var h Header
		if err := json.Unmarshal(data[8:8+header], &h); err != nil {
			t.Fatal(err)
		}
		out = append(out, Frame{Header: h, Payload: append([]byte(nil), data[8+header:total+4]...)})
		data = data[total+4:]
	}
	return out
}

func TestRequestValidation(t *testing.T) {
	for _, raw := range []string{`{"method":"info"}`, `{"id":null,"method":"info","ignored":1}`, `{"method":"embed","id":"x","text":"界"}`, `{"method":"embed_batch","texts":[]}`, `{"method":"embed_batch","texts":["a",""]}`} {
		r, err := ReadRequest(bytes.NewReader(requestBytes(raw)), 1000)
		if err != nil || r.Method == "" {
			t.Fatal(r, err)
		}
	}
	for _, raw := range []string{"{", "[]", "null", "{}", `{"method":null}`, `{"method":1}`, `{"method":"info","id":1}`, `{"method":"unknown"}`, `{"method":"embed"}`, `{"method":"embed","text":null}`, `{"method":"embed","text":1}`, `{"method":"embed_batch"}`, `{"method":"embed_batch","texts":null}`, `{"method":"embed_batch","texts":{}}`, `{"method":"embed_batch","texts":[null]}`, `{"method":"embed_batch","texts":[1]}`} {
		if _, err := ParseRequest([]byte(raw)); err == nil {
			t.Fatal(raw)
		}
	}
	for _, data := range [][]byte{nil, {1, 0}, requestBytes("{}")[0:5]} {
		if _, err := ReadRequest(bytes.NewReader(data), 1000); !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
			t.Fatal(err)
		}
	}
	if _, err := ReadRequest(bytes.NewReader(requestBytes("{}")), 1); err == nil || err.Error() != "frame too large: 2" {
		t.Fatal(err)
	}
	if _, err := ReadRequest(io.MultiReader(bytes.NewReader([]byte{1, 0, 0, 0}), badReader{}), 2); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatal(err)
	}
}

type badReader struct{}

func (badReader) Read([]byte) (int, error) { return 0, io.ErrClosedPipe }

type failingWriter struct{ failAt, calls int }

func (w *failingWriter) Write(data []byte) (int, error) {
	w.calls++
	if w.calls == w.failAt {
		return 0, io.ErrClosedPipe
	}
	return len(data), nil
}

type partialWriter struct{ bytes.Buffer }

func (w *partialWriter) Write(data []byte) (int, error) {
	if len(data) > 1 {
		data = data[:1]
	}
	return w.Buffer.Write(data)
}

type shortWriter struct{ n int }

func (w shortWriter) Write([]byte) (int, error) { return w.n, nil }

type flushWriter struct {
	bytes.Buffer
	err error
}

func (w *flushWriter) Flush() error { return w.err }

func TestHandleFramesAndStream(t *testing.T) {
	m := testModel(t)
	id := "test"
	for _, request := range []Request{{Method: "info"}, {Method: "embed", ID: &id, Text: "hello"}, {Method: "embed_batch", Texts: []string{"hello", "界"}}, {Method: "embed_batch", Texts: []string{}}} {
		frame, err := Handle(m, request)
		if err != nil {
			t.Fatal(err)
		}
		if !frame.Header.OK || *frame.Header.Dimensions != 2 || frame.Header.PayloadLength != len(frame.Payload) || frame.Header.Method != request.Method {
			t.Fatal(frame)
		}
		var wire bytes.Buffer
		if err = WriteFrame(&wire, frame); err != nil {
			t.Fatal(err)
		}
		frames := decodeFrames(t, wire.Bytes())
		if len(frames) != 1 || !reflect.DeepEqual(frames[0].Header, frame.Header) || !bytes.Equal(frames[0].Payload, frame.Payload) {
			t.Fatal(frames)
		}
	}
	for _, request := range []Request{{Method: "bad"}, {Method: "embed", Text: "\xff"}, {Method: "embed_batch", Texts: []string{"\xff"}}} {
		if _, err := Handle(m, request); err == nil {
			t.Fatal(request)
		}
	}
	// The source panics on non-finite outputs; the Go boundary fails cleanly.
	nan := modelBytes()
	for i := len(nan) - 4; i >= len(nan)-24; i -= 4 {
		binary.LittleEndian.PutUint32(nan[i:], math.Float32bits(float32(math.NaN())))
	}
	// Embedding norm weights are just before the six unused pooler floats.
	binary.LittleEndian.PutUint32(nan[len(nan)-32:], math.Float32bits(float32(math.NaN())))
	invalid, err := gte.FromBytes(nan)
	if err != nil {
		t.Fatal(err)
	}
	for _, request := range []Request{{Method: "embed", Text: "a"}, {Method: "embed_batch", Texts: []string{"a"}}} {
		if _, err := Handle(invalid, request); err == nil {
			t.Fatal("nonfinite output accepted")
		}
	}
	input := append(requestBytes(`{"method":"info"}`), requestBytes(`{"method":"unknown"}`)...)
	input = append(input, requestBytes(`{"method":"embed_batch","texts":["a"]}`)...)
	var output bytes.Buffer
	if err = Serve(bytes.NewReader(input), &output, m, 1000); err != nil {
		t.Fatal(err)
	}
	frames := decodeFrames(t, output.Bytes())
	if len(frames) != 3 || frames[1].Header.OK || frames[1].Header.Method != "error" || frames[1].Header.ID != nil {
		t.Fatal(frames)
	}
	output.Reset()
	if err = Serve(bytes.NewReader(requestBytes(`{"method":"info"}`)), &output, m, 1); err == nil {
		t.Fatal("oversize did not stop")
	}
	output.Reset()
	if err = Serve(badReader{}, &output, m, 1000); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatal(err)
	}
	output.Reset()
	if err = Serve(bytes.NewReader(requestBytes(`{"method":"embed","text":"a"}`)), &output, invalid, 1000); err != nil {
		t.Fatal(err)
	}
	if decodeFrames(t, output.Bytes())[0].Header.OK {
		t.Fatal("model error not returned")
	}
	for _, input := range [][]byte{requestBytes(`{"method":"info"}`), requestBytes(`{`)} {
		if err = Serve(bytes.NewReader(input), &failingWriter{failAt: 1}, m, 1000); !errors.Is(err, io.ErrClosedPipe) {
			t.Fatal(err)
		}
	}
}

func TestWriteFailures(t *testing.T) {
	frame, err := Handle(testModel(t), Request{Method: "embed", Text: "a"})
	if err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= 3; i++ {
		if err = WriteFrame(&failingWriter{failAt: i}, frame); !errors.Is(err, io.ErrClosedPipe) {
			t.Fatal(i, err)
		}
	}
	for _, n := range []int{0, 999} {
		if err = WriteFrame(shortWriter{n}, frame); !errors.Is(err, io.ErrShortWrite) {
			t.Fatal(n, err)
		}
	}
	for _, e := range []error{nil, io.ErrClosedPipe} {
		if err = WriteFrame(&flushWriter{err: e}, frame); !errors.Is(err, e) {
			t.Fatal(err)
		}
	}
	partial := &partialWriter{}
	if err = WriteFrame(partial, frame); err != nil {
		t.Fatal(err)
	}
	if len(decodeFrames(t, partial.Bytes())) != 1 {
		t.Fatal("partial writes lost bytes")
	}
	if _, err = frameSize(math.MaxUint32, 0); err == nil {
		t.Fatal("oversize header")
	}
	if _, err = frameSize(1, math.MaxUint32); err == nil {
		t.Fatal("oversize payload")
	}
}

func TestRustWorkerProtocolParity(t *testing.T) {
	data, err := os.ReadFile("../testdata/parity/embedding-protocol.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Request string
		Header  Header
		Values  []float32
	}
	if err = json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	m := testModel(t)
	for _, c := range cases {
		request, err := ParseRequest([]byte(c.Request))
		if err != nil {
			t.Fatal(err)
		}
		frame, err := Handle(m, request)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(c.Header, frame.Header) {
			t.Fatal(c.Header, frame.Header)
		}
		for i, value := range c.Values {
			actual := math.Float32frombits(binary.LittleEndian.Uint32(frame.Payload[i*4:]))
			if math.Abs(float64(actual-value)) > 1e-6 {
				t.Fatal(actual, value)
			}
		}
	}
}

func FuzzFrame(f *testing.F) {
	f.Add(requestBytes(`{"method":"info"}`))
	f.Add([]byte{})
	f.Add([]byte{255, 255, 255, 255})
	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = ReadRequest(bytes.NewReader(data), 4<<20)
		if len(data) < 4096 {
			_, _ = ParseRequest(data)
		}
	})
}

func TestErrorFrame(t *testing.T) {
	frame := ErrorFrame(errors.New("failed"))
	if frame.Header.Error == nil || !strings.Contains(*frame.Header.Error, "failed") || frame.Header.OK {
		t.Fatal(frame)
	}
}

func TestStreamedResponseRejectsOversize(t *testing.T) {
	if err := WriteResponse(io.Discard, Header{}, strings.NewReader(""), math.MaxUint32); err == nil {
		t.Fatal("oversize response accepted")
	}
	if err := WriteResponse(io.Discard, Header{}, strings.NewReader(""), 1); !errors.Is(err, io.EOF) {
		t.Fatal(err)
	}
}
