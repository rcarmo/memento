package umcp

import (
	"context"
	"io"
	"testing"
)

func BenchmarkParsedHTTPRequestRequest1KiB(b *testing.B) {
	request := &ParsedHTTPRequest{
		Method:  "POST",
		Target:  "/mcp",
		Version: "HTTP/1.1",
		Headers: map[string]string{"host": "localhost", "content-type": "application/json"},
		Body:    make([]byte, 1024),
	}
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		value := request.request(context.Background(), "127.0.0.1:1")
		_, _ = io.Copy(io.Discard, value.Body)
	}
}
