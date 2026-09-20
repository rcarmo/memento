package service

import (
	"embed"
	"io/fs"
	"strings"

	"github.com/rcarmo/memento/umcp"
)

//go:embed admin_static/*
var adminStatic embed.FS

var adminReadFile = fs.ReadFile

func adminNotFound() *umcp.HTTPResponse {
	return &umcp.HTTPResponse{Status: 404, Headers: adminHeaders}
}
func adminStaticResponse(relative string) *umcp.HTTPResponse {
	if relative == "" || strings.HasPrefix(relative, "/") {
		return adminNotFound()
	}
	for _, part := range strings.Split(relative, "/") {
		if part == "" || part == "." || part == ".." {
			return adminNotFound()
		}
	}
	mime := ""
	switch relative {
	case "index.html":
		mime = "text/html; charset=utf-8"
	case "app.js":
		mime = "text/javascript; charset=utf-8"
	default:
		return adminNotFound()
	}
	body, err := adminReadFile(adminStatic, "admin_static/"+relative)
	if err != nil {
		return adminNotFound()
	}
	return &umcp.HTTPResponse{Status: 200, Body: body, ContentType: &mime, Headers: adminHeaders}
}
