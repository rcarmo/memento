package service

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"os"
	"strings"
	"sync"

	"github.com/rcarmo/memento/internal/access"
	"github.com/rcarmo/memento/internal/repository"
	"github.com/rcarmo/memento/umcp"
)

// HTTPRoute exposes only fixed children of the configured MCP endpoint. The
// transport passes an unread, length-limited body on both reference servers.
func (f *FileTransfers) HTTPRoute() *umcp.HTTPBodyRoute {
	return &umcp.HTTPBodyRoute{Match: func(path string) bool { return path == "/files/upload" || path == "/files/download" }, Handle: f.handle}
}

func (f *FileTransfers) handle(ctx context.Context, method, path string, headers map[string]string, size int64, body io.Reader) (*umcp.HTTPResponse, error) {
	fail := func(status int) (*umcp.HTTPResponse, error) {
		return stagingJSON(map[string]any{"error": "file transfer unavailable"}, status)
	}
	upload := path == "/files/upload"
	if (!upload || method == "OPTIONS") && size != 0 {
		return fail(400)
	}
	if method == "OPTIONS" {
		return &umcp.HTTPResponse{Status: 204, Headers: [][2]string{{"Access-Control-Allow-Methods", "GET, POST, OPTIONS"}, {"Access-Control-Allow-Headers", "Authorization, Content-Type, X-Memento-File-Token"}, {"Cache-Control", "no-store"}}}, nil
	}
	if (upload && method != "POST") || (!upload && method != "GET") {
		return fail(405)
	}
	principal, err := f.jobs.Identity.AuthenticateHeaders(ctx, headers)
	if err != nil || principal == nil {
		return fail(401)
	}
	if upload && !f.allowUpload {
		return fail(403)
	}
	token := headers[fileTokenHeader]
	r, err := f.record(ctx, token)
	if err != nil || r.Principal != principal.Name {
		return fail(404)
	}
	if (upload && r.Tool != fileUploadTool) || (!upload && r.Tool != fileDownloadTool) {
		return fail(404)
	}
	select {
	case f.gate <- struct{}{}:
	default:
		return fail(429)
	}
	release := true
	defer func() {
		if release {
			<-f.gate
		}
	}()
	if upload {
		if !hasFileRole(principal.Roles, "proposer") {
			return fail(403)
		}
		content, _, err := mime.ParseMediaType(headers["content-type"])
		if err != nil || content != "application/zip" {
			return fail(415)
		}
		var args map[string]any
		decoder := json.NewDecoder(strings.NewReader(r.Arguments))
		decoder.UseNumber()
		if decoder.Decode(&args) != nil {
			return fail(400)
		}
		input, _, err := parseFileArguments(args)
		if err != nil || size != input.Size || size != r.Size {
			return fail(400)
		}
		if f.now().Unix() >= r.Expires {
			return fail(404)
		}
		// Allocate exactly the already-authorized size, once, after admission.
		raw := make([]byte, int(size))
		if _, err := io.ReadFull(checkedFileReader{ctx, body}, raw); err != nil {
			return fail(400)
		}
		if f.now().Unix() >= r.Expires {
			return fail(404)
		}
		active, err := f.jobs.Identity.AuthenticateHeaders(ctx, headers)
		if err != nil || active == nil || active.Name != principal.Name || !hasFileRole(active.Roles, "proposer") {
			return fail(403)
		}
		// A completed logical call may accept the same bytes again even after
		// its staged asset is consumed. It does not write another asset.
		if r.Result != "" {
			digest := sha256.Sum256(raw)
			if hex.EncodeToString(digest[:]) != input.Digest {
				return fail(400)
			}
			return stagingJSON(map[string]any{"uploaded": true}, 200)
		}
		err = f.jobs.Controls.Staging.PutReadyAndBind(ctx, principal.Name, "mcp-files:"+args["idempotency_key"].(string), args["asset_kind"].(string), args["version"].(string), raw, input.Digest, func(tx *sql.Tx, stagedID string, reused bool) error {
			// Only retries of the logical call that owns these bytes can reuse
			// them. The old pack could otherwise expire or be consumed before
			// the new call confirms, even if it is ready right now.
			result, err := tx.ExecContext(ctx, `UPDATE mcp_file_transfers SET staged_id=? WHERE token_digest=? AND expires>? AND (NOT ? OR staged_id=?)`, stagedID, transferDigest(token), f.now().Unix(), reused, stagedID)
			if err != nil {
				return err
			}
			// modernc SQLite returns a concrete affected-row count on success.
			if n, _ := result.RowsAffected(); n != 1 {
				return errExpiredFileTransfer
			}
			return nil
		})
		if errors.Is(err, errExpiredFileTransfer) {
			return fail(404)
		}
		if err != nil {
			return fail(400)
		}
		// Do not expose staged_asset_id before MCP confirms the upload on retry.
		return stagingJSON(map[string]any{"uploaded": true}, 200)
	}
	if f.now().Unix() >= r.Expires {
		return fail(404)
	}
	policy, err := f.jobs.Identity.ResolvePolicy(ctx, *principal)
	if err != nil {
		return fail(403)
	}
	var args downloadArguments
	if json.Unmarshal([]byte(r.Arguments), &args) != nil {
		return fail(400)
	}
	stream, err := f.openArchive(ctx, ProposalActor{Policy: policy}, args, r.Size)
	if err != nil {
		return fail(404)
	}
	if f.now().Unix() >= r.Expires {
		_ = stream.Close()
		return fail(404)
	}
	release = false
	contentType := "application/zip"
	return &umcp.HTTPResponse{Status: 200, BodyStream: &fileTransferStream{ReadCloser: stream, release: func() { <-f.gate }}, BodyLength: r.Size, ContentType: &contentType, Headers: [][2]string{{"Cache-Control", "no-store"}, {"X-Content-Type-Options", "nosniff"}, {"Content-Disposition", "attachment; filename=asset.zip"}}}, nil
}

var errExpiredFileTransfer = errors.New("file transfer expired before staging commit")

type fileTransferStream struct {
	io.ReadCloser
	once    sync.Once
	release func()
}

func (s *fileTransferStream) Close() error {
	var err error
	s.once.Do(func() { err = s.ReadCloser.Close(); s.release() })
	return err
}

type checkedFileReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r checkedFileReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(p)
}

func (f *FileTransfers) openArchive(ctx context.Context, actor ProposalActor, args downloadArguments, size int64) (*os.File, error) {
	if err := access.RequireRole(actor.Policy, "reader"); err != nil {
		return nil, err
	}
	var file *os.File
	err := repository.WithTransactionLock(ctx, f.jobs.Controls.Queue.Paths, func() error {
		c := f.jobs.Controls
		var archivePath string
		stat := func(path string) (os.FileInfo, error) {
			archivePath = path // Already validated by readAcceptedAsset, under this lock.
			return os.Stat(path)
		}
		data, err := c.readAcceptedAsset(actor, AssetGetOptions{IDOrPath: args.Path, AssetKind: args.Kind, Version: &args.Version, ExpectedSHA256: &args.Digest, View: "manifest"}, 0, stat, nil)
		if err != nil {
			return err
		}
		if data["zip_bytes"] != size {
			return errors.New("asset size changed")
		}
		file, err = openCheckedArchive(ctx, archivePath, size, args.Digest)
		return err
	})
	return file, err
}

func openCheckedArchive(ctx context.Context, path string, size int64, digest string) (*os.File, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	hash := sha256.New()
	n, err := io.Copy(hash, checkedFileReader{ctx, io.LimitReader(file, size+1)})
	if err == nil && (n != size || hex.EncodeToString(hash.Sum(nil)) != digest) {
		err = errors.New("asset integrity changed")
	}
	if err == nil {
		_, err = file.Seek(0, io.SeekStart)
	}
	if err != nil {
		_ = file.Close()
		return nil, err
	}
	return file, nil
}
