package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"unicode"

	"github.com/rcarmo/memento/internal/access"
	"github.com/rcarmo/memento/internal/assets"
	"github.com/rcarmo/memento/internal/pyjson"
	"github.com/rcarmo/memento/umcp"
)

// StagingHTTP is an auxiliary route: the HTTP transport must not authenticate
// it first. Upload tickets authenticate /upload; other staging paths require a
// proposer bearer. Non-staging paths return nil for the next route handler.
type StagingHTTP struct {
	Store        *assets.StagingStore
	Authenticate func(context.Context, map[string]string) (*access.Principal, error)
}

func (h StagingHTTP) Handle(ctx context.Context, method, target string, headers map[string]string, body []byte, _ string) (*umcp.HTTPResponse, error) {
	route, err := umcp.SplitRequestTarget(target)
	if err != nil {
		return nil, err
	}
	path := route.Path
	if path != "/assets/staging" && !strings.HasPrefix(path, "/assets/staging/") {
		return nil, nil
	}
	payload, status, err := h.handle(ctx, method, path, headers, body)
	if err != nil {
		var staged *assets.StagedAssetError
		var pack *assets.ValidationError
		var syntax *json.SyntaxError
		if !errors.As(err, &staged) && !errors.As(err, &pack) && !errors.As(err, &syntax) {
			return nil, err
		}
		payload, status = map[string]any{"error": err.Error()}, 400
	}
	return stagingJSON(payload, status)
}
func (h StagingHTTP) handle(ctx context.Context, method, path string, headers map[string]string, body []byte) (map[string]any, int, error) {
	ticketUpload := method == "POST" && path == "/assets/staging/upload"
	var principal *access.Principal
	if !ticketUpload {
		var err error
		principal, err = h.Authenticate(ctx, headers)
		if err != nil {
			return nil, 0, err
		}
		proposer := false
		if principal != nil {
			for _, role := range principal.Roles {
				if role == "proposer" {
					proposer = true
				}
			}
		}
		if !proposer {
			return map[string]any{"error": "proposer bearer credential required"}, 401, nil
		}
	}
	if ticketUpload || (method == "POST" && path == "/assets/staging") {
		content, _, _ := strings.Cut(headers["content-type"], ";")
		if strings.TrimFunc(content, pythonHTTPWhitespace) != "application/zip" {
			return map[string]any{"error": "Content-Type must be application/zip"}, 415, nil
		}
		if len(body) > assets.MaxZIPBytes {
			return map[string]any{"error": "ZIP archive exceeds maximum encoded size"}, 413, nil
		}
		var asset assets.StagedAsset
		var replayed bool
		var err error
		if ticketUpload {
			asset, replayed, err = h.Store.PutWithTicket(ctx, headers["x-memento-upload-ticket"], body)
		} else {
			asset, replayed, err = h.Store.Put(ctx, principal.Name, headers["idempotency-key"], headers["x-memento-asset-kind"], headers["x-memento-asset-version"], body)
		}
		if err != nil {
			return nil, 0, err
		}
		payload := stagedPayload(asset)
		payload["replayed"] = replayed
		status := 201
		if replayed {
			status = 200
		}
		return payload, status, nil
	}
	if method == "GET" && strings.HasPrefix(path, "/assets/staging/") {
		asset, err := h.Store.Get(ctx, principal.Name, strings.TrimPrefix(path, "/assets/staging/"), false)
		if err != nil {
			return nil, 0, err
		}
		return stagedPayload(asset), 200, nil
	}
	return map[string]any{"error": "not found"}, 404, nil
}
func pythonHTTPWhitespace(r rune) bool { return unicode.IsSpace(r) || r >= 0x1c && r <= 0x1f }
func stagedPayload(asset assets.StagedAsset) map[string]any {
	// Fixed concrete staging/manifest fields contain no unsupported JSON values.
	raw, _ := json.Marshal(asset.PublicPayload())
	value, _ := pyjson.Parse(string(raw))
	return value.(map[string]any)
}
func stagingJSON(payload map[string]any, status int) (*umcp.HTTPResponse, error) {
	// ASCII escapes/spacing match source dumps; object key order is sorted in Go.
	raw, err := pyjson.Dumps(payload)
	if err != nil {
		return nil, err
	}
	contentType := "application/json; charset=utf-8"
	return &umcp.HTTPResponse{Status: status, Body: []byte(raw), ContentType: &contentType, Headers: [][2]string{{"Cache-Control", "no-store"}, {"X-Content-Type-Options", "nosniff"}}}, nil
}
