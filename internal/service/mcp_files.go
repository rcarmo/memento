package service

// Experimental adapter for the files draft at upstream 8a859b8f268926fe4beb84d78565e7cc6f9fa617.
// This stages ZIP packs only; it never proposes or applies canonical changes.
import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/rcarmo/memento/internal/assets"
	"github.com/rcarmo/memento/internal/control"
	"github.com/rcarmo/memento/umcp"
)

const fileExtension = "io.modelcontextprotocol/files"
const fileProtocol = umcp.StatelessHTTPVersion
const fileUploadTool = "memory_asset_upload"
const fileDownloadTool = "memory_asset_download"
const fileTokenHeader = "x-memento-file-token"
const fileLifetime = time.Hour

// FileTransfers is explicitly enabled during construction. Records are shared
// in control SQLite, not tied to a protocol session or process-local signing key.
type FileTransfers struct {
	jobs        *Jobs
	now         func() time.Time
	gate        chan struct{}
	allowUpload bool
	random      io.Reader
}
type fileCapabilities struct{ Upload, Download, Declared bool }
type fileCapabilitiesKey struct{}
type transferRecord struct {
	Principal, Tool, Arguments, Capabilities string
	Expires, Size                            int64
	StagedID, Result                         string
}
type fileInput struct {
	ID, Digest string
	Size       int64
}
type downloadArguments struct{ Path, Kind, Version, Digest string }

// Field order matches canonical JSON map order; optional name preserves
// absent-versus-empty instance equality without cloning and sorting maps.
type canonicalFileArgument struct {
	Digest struct {
		Algorithm string `json:"algorithm"`
		Value     string `json:"value"`
	} `json:"digest"`
	ID       string  `json:"id"`
	MimeType string  `json:"mimeType"`
	Name     *string `json:"name,omitempty"`
	Size     int64   `json:"size"`
}
type canonicalUploadArguments struct {
	Kind    string                `json:"asset_kind"`
	File    canonicalFileArgument `json:"file"`
	Key     string                `json:"idempotency_key"`
	Version string                `json:"version"`
}

func NewFileTransfers(ctx context.Context, jobs *Jobs, server *umcp.Server, allowUpload bool) (*FileTransfers, error) {
	return newFileTransfers(ctx, jobs, server, allowUpload, fileTools())
}
func newFileTransfers(ctx context.Context, jobs *Jobs, server *umcp.Server, allowUpload bool, tools []umcp.Tool) (*FileTransfers, error) {
	if server == nil || jobs == nil || jobs.Identity == nil || jobs.Controls == nil || jobs.Controls.Staging == nil || jobs.Controls.Staging.DB == nil {
		return nil, errors.New("file staging unavailable")
	}
	_, err := jobs.Controls.Staging.DB.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS mcp_file_transfers (
 token_digest TEXT PRIMARY KEY, principal TEXT NOT NULL, tool TEXT NOT NULL,
 arguments TEXT NOT NULL, capabilities TEXT NOT NULL, expires INTEGER NOT NULL,
 size INTEGER NOT NULL, staged_id TEXT NOT NULL DEFAULT '', finalized INTEGER NOT NULL DEFAULT 0, result_json TEXT NOT NULL DEFAULT '')`)
	if err != nil {
		return nil, err
	}
	f := &FileTransfers{jobs: jobs, now: time.Now, gate: make(chan struct{}, 1), allowUpload: allowUpload, random: rand.Reader}
	server.AdditionalProtocolVersions = append(server.AdditionalProtocolVersions, fileProtocol)
	previous := server.Tools.Visible
	server.Tools.Visible = func(ctx context.Context, t umcp.Tool) bool {
		if previous != nil && !previous(ctx, t) {
			return false
		}
		caps, _ := ctx.Value(fileCapabilitiesKey{}).(fileCapabilities)
		switch t.Name {
		case fileUploadTool:
			return f.allowUpload && fileContext(ctx) && caps.Upload
		case fileDownloadTool:
			return fileContext(ctx) && caps.Download
		}
		return true
	}
	for _, tool := range tools {
		if err = server.Tools.Register(tool); err != nil {
			return nil, err
		}
	}
	server.SetHandler("tools/list", func(ctx context.Context, p map[string]any) (any, *umcp.RPCError, error) {
		result, rpcErr, err := server.Tools.List(context.WithValue(ctx, fileCapabilitiesKey{}, readFileCapabilities(p)), p)
		if fileContext(ctx) && rpcErr == nil && err == nil {
			if listing, ok := result.(map[string]any); ok {
				addFileDiscoveryMetadata(listing)
			}
		}
		return result, rpcErr, err
	})
	server.SetHandler("server/discover", func(ctx context.Context, _ map[string]any) (any, *umcp.RPCError, error) {
		if !fileContext(ctx) {
			return nil, &umcp.RPCError{Code: -32601, Message: "Method not found: server/discover"}, nil
		}
		settings := map[string]any{"maxInlineBytes": 0}
		if f.allowUpload {
			settings["upload"] = map[string]any{}
		}
		result := map[string]any{"instructions": server.Instructions, "supportedVersions": append([]string{fileProtocol}, umcp.ProtocolVersions()...), "capabilities": map[string]any{"tools": map[string]any{}, "extensions": map[string]any{fileExtension: settings}}}
		addFileDiscoveryMetadata(result)
		return result, nil, nil
	})
	server.SetHandler("tools/call", func(ctx context.Context, p map[string]any) (any, *umcp.RPCError, error) {
		name, _ := p["name"].(string)
		if name == fileUploadTool || name == fileDownloadTool {
			return f.call(ctx, p)
		}
		if _, exists := p["requestState"]; exists && fileContext(ctx) {
			return nil, invalidFileState(), nil
		}
		result, rpcErr, err := server.Tools.Call(ctx, p)
		if fileContext(ctx) && rpcErr == nil && err == nil {
			if m, ok := result.(map[string]any); ok {
				m["resultType"] = "complete"
			}
		}
		return result, rpcErr, err
	})
	return f, nil
}
func addFileDiscoveryMetadata(result map[string]any) {
	result["resultType"] = "complete"
	result["ttlMs"] = 0 // Capability- and principal-sensitive; do not cache.
	result["cacheScope"] = "private"
}

func fileContext(ctx context.Context) bool {
	c := umcp.Context(ctx)
	return c.Transport == "streamable-http" && c.ProtocolVersion == fileProtocol
}
func readFileCapabilities(p map[string]any) fileCapabilities {
	meta, _ := p["_meta"].(map[string]any)
	client, _ := meta["io.modelcontextprotocol/clientCapabilities"].(map[string]any)
	extensions, _ := client["extensions"].(map[string]any)
	settings, ok := extensions[fileExtension].(map[string]any)
	if !ok {
		return fileCapabilities{}
	}
	if revision, exists := settings["revision"]; exists && revision != "" {
		return fileCapabilities{}
	}
	_, upload := settings["upload"].(map[string]any)
	_, download := settings["download"].(map[string]any)
	return fileCapabilities{upload, download, true}
}
func missingFileCapability(name string) *umcp.RPCError {
	return &umcp.RPCError{Code: -32021, Message: "Required client file capability missing", Data: map[string]any{"requiredCapabilities": map[string]any{"extensions": map[string]any{fileExtension: map[string]any{name: map[string]any{}}}}}}
}
func invalidFileState() *umcp.RPCError {
	return &umcp.RPCError{Code: -32602, Message: "Invalid file requestState or binding"}
}
func fileFailure(message string) (any, *umcp.RPCError, error) {
	return map[string]any{"resultType": "complete", "isError": true, "content": []any{map[string]any{"type": "text", "text": message}}}, nil, nil
}
func fileComplete(value any) map[string]any {
	raw, _ := json.Marshal(value)
	return map[string]any{"resultType": "complete", "content": []any{map[string]any{"type": "text", "text": string(raw)}}, "structuredContent": value}
}
func (f *FileTransfers) call(ctx context.Context, p map[string]any) (any, *umcp.RPCError, error) {
	if !fileContext(ctx) {
		return nil, &umcp.RPCError{Code: -32601, Message: "Experimental file tools require negotiated HTTP support"}, nil
	}
	name, _ := p["name"].(string)
	if name == fileUploadTool && !f.allowUpload {
		return nil, &umcp.RPCError{Code: -32601, Message: "Tool not found: " + name}, nil
	}
	args, ok := p["arguments"].(map[string]any)
	if !ok {
		if _, retry := p["requestState"]; retry {
			return nil, invalidFileState(), nil
		}
		return fileFailure("File tool arguments must be an object")
	}
	principal, _, err := f.jobs.Identity.requestPrincipal(ctx)
	if err != nil {
		return fileFailure("File transfer not authorized")
	}
	caps := readFileCapabilities(p)
	capsText := fmt.Sprintf("%t/%t", caps.Upload, caps.Download)
	if name == fileDownloadTool {
		if _, exists := p["requestState"]; exists {
			return nil, invalidFileState(), nil
		}
		if !caps.Download {
			return nil, missingFileCapability("download"), nil
		}
		return f.download(ctx, principal.Name, args, capsText)
	}
	input, canonical, err := parseFileArguments(args)
	if err != nil {
		if _, retry := p["requestState"]; retry {
			return nil, invalidFileState(), nil
		}
		return fileFailure("Expected a non-empty ZIP pack with valid size, sha-256 digest and asset identity; inline data is disabled")
	}
	if state, exists := p["requestState"]; exists {
		token, ok := state.(string)
		if !ok {
			return nil, invalidFileState(), nil
		}
		record, err := f.record(ctx, token)
		if err != nil || record.Principal != principal.Name || record.Tool != name || record.Arguments != canonical || record.Capabilities != capsText {
			return nil, invalidFileState(), nil
		}
		if !hasFileRole(principal.Roles, "proposer") {
			return fileFailure("Proposer role required")
		}
		if record.Result != "" {
			return completedFileTransfer(record.Result)
		}
		if record.StagedID == "" {
			return fileFailure("Uploaded bytes have not been confirmed")
		}
		staged, size, err := f.jobs.Controls.Staging.Metadata(ctx, principal.Name, record.StagedID)
		if err != nil || size != input.Size || staged.SHA256 != input.Digest {
			// Another instance may have completed this call before a proposal
			// consumed the staged BLOB. Its immutable result is still valid.
			return f.confirmationFallback(ctx, token, false)
		}
		// The immutable staging write verified these exact bytes and computed the
		// digest. Confirm storage metadata instead of loading a second ZIP copy.
		result := fileComplete(stagedPayload(staged))
		// The fixed response consists only of strings, booleans and values
		// decoded from a validated manifest; all are JSON encodable.
		encoded, _ := json.Marshal(result)
		updated, err := f.jobs.Controls.Staging.DB.ExecContext(ctx, "UPDATE mcp_file_transfers SET finalized=1,result_json=? WHERE token_digest=? AND expires>? AND finalized=0", string(encoded), transferDigest(token), f.now().Unix())
		if err != nil {
			return fileFailure("Upload confirmation unavailable")
		}
		if count, err := updated.RowsAffected(); err != nil || count != 1 {
			return f.confirmationFallback(ctx, token, true)
		}
		// Staging is idempotent: retries return the same identity, even if a proposal
		// has since consumed it. No canonical mutation or second upload is performed.
		return result, nil, nil
	}
	if !hasFileRole(principal.Roles, "proposer") {
		return fileFailure("Proposer role required")
	}
	if !caps.Upload {
		return nil, missingFileCapability("upload"), nil
	}
	token, record, err := f.issue(ctx, principal.Name, name, canonical, capsText, input.Size)
	if err != nil {
		return fileFailure("File transfer quota or storage unavailable")
	}
	return map[string]any{"resultType": "upload_required", "requestState": token, "uploads": []any{map[string]any{"fileId": input.ID, "path": "/files/upload", "method": "POST", "credentials": "include", "expiresAt": time.Unix(record.Expires, 0).UTC().Format(time.RFC3339), "headers": map[string]string{"Content-Type": "application/zip", "X-Memento-File-Token": token}}}}, nil, nil
}
func (f *FileTransfers) confirmationFallback(ctx context.Context, token string, invalidState bool) (any, *umcp.RPCError, error) {
	if latest, err := f.record(ctx, token); err == nil && latest.Result != "" {
		return completedFileTransfer(latest.Result)
	}
	if invalidState {
		return nil, invalidFileState(), nil
	}
	return fileFailure("Uploaded pack could not be verified")
}

func completedFileTransfer(encoded string) (any, *umcp.RPCError, error) {
	var result map[string]any
	if err := json.Unmarshal([]byte(encoded), &result); err != nil || result == nil {
		return fileFailure("Upload confirmation unavailable")
	}
	return result, nil, nil
}
func hasFileRole(roles []string, role string) bool {
	for _, v := range roles {
		if v == role {
			return true
		}
	}
	return false
}
func parseFileArguments(args map[string]any) (fileInput, string, error) {
	bad := errors.New("invalid file metadata")
	var in fileInput
	if !onlyFileKeys(args, "asset_kind", "version", "idempotency_key", "file") {
		return in, "", bad
	}
	kind, k := args["asset_kind"].(string)
	version, v := args["version"].(string)
	key, i := args["idempotency_key"].(string)
	if !k || !v || !i || len(key) > 256 || len(kind) > 128 || len(version) > 128 || strings.TrimSpace(key) == "" {
		return in, "", bad
	}
	if assets.ValidateKind(kind) != nil {
		return in, "", bad
	}
	if _, err := assets.ParseStableSemver(version); err != nil {
		return in, "", bad
	}
	file, ok := args["file"].(map[string]any)
	if !ok || !onlyFileKeys(file, "id", "name", "mimeType", "size", "digest") {
		return in, "", bad
	}
	in.ID, ok = file["id"].(string)
	if !ok || len(in.ID) > 512 || utf8.RuneCountInString(in.ID) < 1 || utf8.RuneCountInString(in.ID) > 128 {
		return in, "", bad
	}
	if name, exists := file["name"]; exists {
		if text, ok := name.(string); !ok || len(text) > 1024 {
			return in, "", bad
		}
	}
	if file["mimeType"] != "application/zip" {
		return in, "", bad
	}
	n, ok := file["size"].(json.Number)
	if !ok || len(n) > 64 {
		return in, "", bad
	}
	size, integerErr := strconv.ParseInt(string(n), 10, 64)
	if integerErr == nil {
		in.Size = size
	} else {
		approximate, err := n.Float64()
		if err != nil || !(approximate >= 1 && approximate <= assets.MaxZIPBytes) {
			return in, "", bad
		}
		r, ok := new(big.Rat).SetString(string(n))
		if !ok || !r.IsInt() || !r.Num().IsInt64() {
			return in, "", bad
		}
		in.Size = r.Num().Int64()
	}
	if in.Size <= 0 || in.Size > assets.MaxZIPBytes {
		return in, "", bad
	}
	digest, ok := file["digest"].(map[string]any)
	if !ok || !onlyFileKeys(digest, "algorithm", "value") || digest["algorithm"] != "sha-256" {
		return in, "", bad
	}
	value, ok := digest["value"].(string)
	if !ok {
		return in, "", bad
	}
	decoded, ok := decodeFileDigest(value)
	if !ok {
		return in, "", bad
	}
	in.Digest = hex.EncodeToString(decoded[:])
	// Every other accepted field is a string/object. Normalize the sole numeric
	// field for JSON Schema instance equality (e.g. 10, 10.0 and 1e1).
	canonical := canonicalFileArgument{ID: in.ID, MimeType: "application/zip", Size: in.Size}
	canonical.Digest.Algorithm = "sha-256"
	canonical.Digest.Value = value
	if name, exists := file["name"]; exists {
		text := name.(string)
		canonical.Name = &text
	}
	raw, err := json.Marshal(canonicalUploadArguments{Kind: kind, File: canonical, Key: key, Version: version})
	return in, string(raw), err
}
func decodeFileDigest(value string) ([sha256.Size]byte, bool) {
	var digest [sha256.Size]byte
	if len(value) != base64.RawURLEncoding.EncodedLen(len(digest)) {
		return digest, false
	}
	for i := range len(value) {
		c := value[i]
		if !(c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-' || c == '_') {
			return digest, false
		}
	}
	n, err := base64.RawURLEncoding.Strict().Decode(digest[:], []byte(value))
	return digest, err == nil && n == len(digest)
}

func onlyFileKeys(m map[string]any, keys ...string) bool {
	for key := range m {
		found := false
		for _, allowed := range keys {
			found = found || key == allowed
		}
		if !found {
			return false
		}
	}
	return true
}
func transferDigest(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
func (f *FileTransfers) record(ctx context.Context, token string) (transferRecord, error) {
	var r transferRecord
	if _, ok := decodeFileDigest(token); !ok {
		return r, errors.New("invalid token")
	}
	err := f.jobs.Controls.Staging.DB.QueryRowContext(ctx, `SELECT principal,tool,arguments,capabilities,expires,size,staged_id,result_json FROM mcp_file_transfers WHERE token_digest=? AND expires>?`, transferDigest(token), f.now().Unix()).Scan(&r.Principal, &r.Tool, &r.Arguments, &r.Capabilities, &r.Expires, &r.Size, &r.StagedID, &r.Result)
	return r, err
}
func (f *FileTransfers) issue(ctx context.Context, principal, tool, args, caps string, size int64) (string, transferRecord, error) {
	random := make([]byte, 32)
	if _, err := io.ReadFull(f.random, random); err != nil {
		return "", transferRecord{}, err
	}
	token := base64.RawURLEncoding.EncodeToString(random)
	r := transferRecord{Principal: principal, Tool: tool, Arguments: args, Capabilities: caps, Size: size, Expires: f.now().Add(fileLifetime).Unix()}
	err := control.WithTransaction(ctx, f.jobs.Controls.Staging.DB, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, "DELETE FROM mcp_file_transfers WHERE expires<=?", f.now().Unix()); err != nil {
			return err
		}
		var total, pending int
		var reserved int64
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*),COALESCE(SUM(CASE WHEN principal=? AND finalized=0 AND tool='memory_asset_upload' THEN 1 ELSE 0 END),0),COALESCE(SUM(CASE WHEN finalized=0 AND tool='memory_asset_upload' THEN size ELSE 0 END),0) FROM mcp_file_transfers`, principal).Scan(&total, &pending, &reserved); err != nil {
			return err
		}
		if total >= 64 || (tool == fileUploadTool && (pending >= 4 || reserved+size > 2*assets.MaxZIPBytes)) {
			return errors.New("transfer quota")
		}
		_, err := tx.ExecContext(ctx, `INSERT INTO mcp_file_transfers(token_digest,principal,tool,arguments,capabilities,expires,size) VALUES(?,?,?,?,?,?,?)`, transferDigest(token), principal, tool, args, caps, r.Expires, size)
		return err
	})
	return token, r, err
}
func (f *FileTransfers) download(ctx context.Context, principal string, args map[string]any, caps string) (any, *umcp.RPCError, error) {
	if !onlyFileKeys(args, "id_or_path", "asset_kind", "version") {
		return fileFailure("Invalid asset download arguments")
	}
	path, p := args["id_or_path"].(string)
	kind, k := args["asset_kind"].(string)
	if !p || !k {
		return fileFailure("Asset identity required")
	}
	var version *string
	if value, exists := args["version"]; exists {
		v, ok := value.(string)
		if !ok {
			return fileFailure("Invalid asset version")
		}
		version = &v
	}
	actor, err := f.jobs.Identity.Context(ctx)
	if err != nil {
		return fileFailure("Asset download not authorized")
	}
	data, _, err := f.jobs.Controls.AssetGet(ctx, actor, AssetGetOptions{IDOrPath: path, AssetKind: kind, Version: version, View: "manifest"})
	if err != nil {
		return fileFailure("Asset download unavailable")
	}
	resolved := downloadArguments{data["concept_path"].(string), kind, data["version"].(string), data["zip_sha256"].(string)}
	raw, _ := json.Marshal(resolved)
	size := data["zip_bytes"].(int64)
	token, r, err := f.issue(ctx, principal, fileDownloadTool, string(raw), caps, size)
	if err != nil {
		return fileFailure("File transfer quota or storage unavailable")
	}
	digest, _ := hex.DecodeString(resolved.Digest)
	result := fileComplete(map[string]any{"asset_kind": kind, "version": resolved.Version, "size": size, "sha256": resolved.Digest})
	result["_meta"] = map[string]any{fileExtension: map[string]any{"downloads": []any{map[string]any{"name": "asset-" + kind + "-" + resolved.Version + ".zip", "mimeType": "application/zip", "size": size, "digest": map[string]any{"algorithm": "sha-256", "value": base64.RawURLEncoding.EncodeToString(digest)}, "path": "/files/download", "credentials": "include", "expiresAt": time.Unix(r.Expires, 0).UTC().Format(time.RFC3339), "headers": map[string]string{"X-Memento-File-Token": token}}}}}
	return result, nil, nil
}
