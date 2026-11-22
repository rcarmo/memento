package umcp

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func BenchmarkFileProtocolVersions(b *testing.B) {
	for _, enabled := range []bool{false, true} {
		name := "legacy"
		if enabled {
			name = "files"
		}
		b.Run(name, func(b *testing.B) {
			server := NewServer("bench")
			if enabled {
				server.AdditionalProtocolVersions = []string{"2026-07-28"}
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if !server.SupportsProtocolVersion("2025-03-26") {
					b.Fatal("version")
				}
			}
		})
	}
}
func BenchmarkAuxiliaryFileDownload(b *testing.B) {
	data := strings.Repeat("x", 1<<20)
	hooks := HTTPHooks{Route: func(context.Context, string, string, map[string]string, []byte, string) (*HTTPResponse, error) {
		return &HTTPResponse{Status: 200, BodyStream: io.NopCloser(strings.NewReader(data)), BodyLength: int64(len(data))}, nil
	}}
	transport, err := NewStreamableHTTP(NewServer("bench"), DefaultHTTPOptions(), hooks)
	if err != nil {
		b.Fatal(err)
	}
	defer transport.Close()
	host := httptest.NewServer(transport)
	defer host.Close()
	b.ReportAllocs()
	b.SetBytes(int64(len(data)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r, err := http.Get(host.URL + "/file")
		if err != nil {
			b.Fatal(err)
		}
		n, err := io.Copy(io.Discard, r.Body)
		r.Body.Close()
		if err != nil || n != int64(len(data)) {
			b.Fatal(n, err)
		}
	}
}
