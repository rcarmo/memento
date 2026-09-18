package service

import (
	"embed"
	"io/fs"
	"path"
	"strings"

	"github.com/rcarmo/memento/go/umcp"
)

//go:embed graph_static/* graph_static/vendor/*
var graphStatic embed.FS

var graphStaticTypes = map[string]string{".css": "text/css; charset=utf-8", ".html": "text/html; charset=utf-8", ".js": "text/javascript; charset=utf-8", ".json": "application/json; charset=utf-8", ".svg": "image/svg+xml"}

func graphStaticResponse(relative, prefix string) *umcp.HTTPResponse {
	if relative == "" || strings.HasPrefix(relative, "/") {
		return graphNotFound()
	}
	parts := strings.Split(relative, "/")
	for _, part := range parts {
		if part == "" || part == "." || part == ".." {
			return graphNotFound()
		}
	}
	body, err := fs.ReadFile(graphStatic, "graph_static/"+relative)
	if err != nil {
		return graphNotFound()
	}
	if relative == "index.html" {
		body = []byte(strings.ReplaceAll(string(body), "__GRAPH_PREFIX__", prefix))
	}
	mime := graphStaticTypes[strings.ToLower(path.Ext(relative))]
	if mime == "" {
		mime = "application/octet-stream"
	}
	return &umcp.HTTPResponse{Status: 200, Body: body, ContentType: &mime, Headers: graphHeaders}
}
