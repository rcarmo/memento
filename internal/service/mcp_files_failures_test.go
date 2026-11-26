package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rcarmo/memento/internal/access"
	"github.com/rcarmo/memento/internal/assets"
	"github.com/rcarmo/memento/internal/execute"
	"github.com/rcarmo/memento/umcp"
)

func TestMCPFilesStagingFailureHandling(t *testing.T) {
	h := newFileHarness(t)
	ctx := context.Background()
	store := h.jobs.Controls.Staging
	db := store.DB
	raw := submitZIP(t)
	sum := sha256.Sum256(raw)
	digest := hex.EncodeToString(sum[:])
	binder := func(*sql.Tx, string, bool) error { return nil }
	for _, c := range []struct {
		key, kind, digest string
		bind              func(*sql.Tx, string, bool) error
	}{
		{"", "docs", digest, binder}, {"blank-binder", "docs", digest, nil}, {"bad-kind", "BAD", digest, binder}, {"bad-digest", "docs", "bad", binder},
	} {
		if err := store.PutReadyAndBind(ctx, "actor", c.key, c.kind, "1.0.0", raw, c.digest, c.bind); err == nil {
			t.Fatalf("accepted %+v", c)
		}
	}
	if _, err := db.Exec(`CREATE TRIGGER test_fail_stage BEFORE INSERT ON staged_assets BEGIN SELECT RAISE(ABORT, 'fixture'); END`); err != nil {
		t.Fatal(err)
	}
	if err := store.PutReadyAndBind(ctx, "actor", "failed-insert", "docs", "1.0.0", raw, digest, binder); err == nil {
		t.Fatal("storage error ignored")
	}
	if _, err := db.Exec(`ALTER TABLE staged_assets RENAME TO inaccessible_staged_assets`); err != nil {
		t.Fatal(err)
	}
	if err := store.PutReadyAndBind(ctx, "actor", "failed-query", "docs", "1.0.0", raw, digest, binder); err == nil {
		t.Fatal("query error ignored")
	}
}

func TestMCPFilesUploadFailureHandling(t *testing.T) {
	h := newFileHarness(t)
	db := h.jobs.Controls.Staging.DB
	ctx := context.Background()
	raw := submitZIP(t)
	args := fileArguments(raw, "failures")
	token, headers := h.begin(args)
	lower := map[string]string{"authorization": "Bearer actor", "content-type": "application/zip", fileTokenHeader: token}
	do := func(want int, size int64, source io.Reader) {
		t.Helper()
		resp, err := h.files.handle(ctx, "POST", "/files/upload", lower, size, source)
		if err != nil || resp == nil || resp.Status != want {
			t.Fatalf("wanted %d: %+v %v", want, resp, err)
		}
	}
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := db.Exec(query, args...); err != nil {
			t.Fatal(err)
		}
	}
	if resp, err := h.files.handle(ctx, "GET", "/files/download", nil, 1, nil); err != nil || resp.Status != 400 {
		t.Fatal(resp, err)
	}
	_, canonical, err := parseFileArguments(args)
	if err != nil {
		t.Fatal(err)
	}
	key := transferDigest(token)
	exec("UPDATE mcp_file_transfers SET arguments='{' WHERE token_digest=?", key)
	do(400, int64(len(raw)), bytes.NewReader(raw))
	exec("UPDATE mcp_file_transfers SET arguments=? WHERE token_digest=?", canonical, key)
	do(400, int64(len(raw)), bytes.NewReader(nil))
	exec("UPDATE mcp_file_transfers SET principal='reader' WHERE token_digest=?", key)
	lower["authorization"] = "Bearer reader"
	do(403, int64(len(raw)), bytes.NewReader(raw))
	params := fileParams(fileUploadTool, args)
	params["requestState"] = token
	if r := fileResult(t, h.rpc("tools/call", "reader", params)); r["isError"] != true {
		t.Fatal(r)
	}
	exec("UPDATE mcp_file_transfers SET principal='actor' WHERE token_digest=?", key)
	lower["authorization"] = "Bearer actor"
	for call, want := range []int{404, 404} {
		n := 0
		base := time.Now()
		h.files.now = func() time.Time {
			n++
			if n >= call+2 {
				return base.Add(2 * time.Hour)
			}
			return base
		}
		do(want, int64(len(raw)), bytes.NewReader(raw))
	}
	h.files.now = time.Now
	do(403, int64(len(raw)), fileFixtureReader(func(p []byte) (int, error) {
		lower["authorization"] = "Bearer reader"
		return copy(p, raw), nil
	}))
	lower["authorization"] = "Bearer actor"
	exec(`CREATE TRIGGER test_fail_bind BEFORE UPDATE OF staged_id ON mcp_file_transfers BEGIN SELECT RAISE(ABORT, 'fixture'); END`)
	do(400, int64(len(raw)), bytes.NewReader(raw))
	exec(`DROP TRIGGER test_fail_bind`)
	if status, _ := h.request("POST", "/mcp/files/upload", "actor", raw, headers); status != 200 {
		t.Fatal(status)
	}
	var stage string
	if err := db.QueryRow(`SELECT staged_id FROM mcp_file_transfers WHERE token_digest=?`, key).Scan(&stage); err != nil {
		t.Fatal(err)
	}
	exec(`UPDATE mcp_file_transfers SET staged_id='missing' WHERE token_digest=?`, key)
	if r := fileResult(t, h.rpc("tools/call", "actor", params)); r["isError"] != true {
		t.Fatal(r)
	}
	exec(`UPDATE mcp_file_transfers SET staged_id=? WHERE token_digest=?`, stage, key)
	exec(`CREATE TRIGGER test_fail_confirm BEFORE UPDATE OF finalized ON mcp_file_transfers BEGIN SELECT RAISE(ABORT, 'fixture'); END`)
	if r := fileResult(t, h.rpc("tools/call", "actor", params)); r["isError"] != true {
		t.Fatal(r)
	}
	exec(`DROP TRIGGER test_fail_confirm`)
	exec(`CREATE TRIGGER test_ignore_confirm BEFORE UPDATE OF finalized ON mcp_file_transfers BEGIN SELECT RAISE(IGNORE); END`)
	fileRPCError(t, h.rpc("tools/call", "actor", params), -32602)
	exec(`DROP TRIGGER test_ignore_confirm`)
	if r := fileResult(t, h.rpc("tools/call", "actor", params)); r["isError"] == true {
		t.Fatal(r)
	}
	if r, _, e := h.files.confirmationFallback(ctx, token, true); e != nil || r.(map[string]any)["isError"] == true {
		t.Fatal(r, e)
	}
	bad := append([]byte(nil), raw...)
	bad[0] ^= 1
	do(400, int64(len(raw)), bytes.NewReader(bad))
}

func TestMCPFilesDownloadFailureHandling(t *testing.T) {
	h := newFileHarness(t)
	h.acceptPack()
	ctx := context.Background()
	db := h.jobs.Controls.Staging.DB
	args := map[string]any{"asset_kind": "docs", "id_or_path": "/a.md", "version": "1.0.0"}
	if r, _, _ := h.files.download(ctx, "", args, ""); r.(map[string]any)["isError"] != true {
		t.Fatal(r)
	}
	request := fileResult(t, h.rpc("tools/call", "actor", fileParams(fileDownloadTool, args)))
	token := request["_meta"].(map[string]any)[fileExtension].(map[string]any)["downloads"].([]any)[0].(map[string]any)["headers"].(map[string]any)["X-Memento-File-Token"].(string)
	record, err := h.files.record(ctx, token)
	if err != nil {
		t.Fatal(err)
	}
	headers := map[string]string{"authorization": "Bearer actor", fileTokenHeader: token}
	// A resolution failure never returns a download target.
	principal, ok := h.jobs.Identity.authorization.Principals["actor"]
	if !ok {
		t.Fatal("actor missing")
	}
	delete(h.jobs.Identity.authorization.Principals, "actor")
	status := func(want int) {
		t.Helper()
		r, e := h.files.handle(ctx, "GET", "/files/download", headers, 0, nil)
		if e != nil || r.Status != want {
			t.Fatalf("wanted %d: %+v %v", want, r, e)
		}
	}
	status(403)
	h.jobs.Identity.authorization.Principals["actor"] = principal
	if _, err = db.Exec(`UPDATE mcp_file_transfers SET arguments='{' WHERE token_digest=?`, transferDigest(token)); err != nil {
		t.Fatal(err)
	}
	status(400)
	if _, err = db.Exec(`UPDATE mcp_file_transfers SET arguments=?,size=size+1 WHERE token_digest=?`, record.Arguments, transferDigest(token)); err != nil {
		t.Fatal(err)
	}
	status(404)
	if _, err = db.Exec(`UPDATE mcp_file_transfers SET size=? WHERE token_digest=?`, record.Size, transferDigest(token)); err != nil {
		t.Fatal(err)
	}
	for _, limit := range []int{2, 3} {
		n := 0
		base := time.Now()
		h.files.now = func() time.Time {
			n++
			if n >= limit {
				return base.Add(2 * time.Hour)
			}
			return base
		}
		status(404)
	}
	h.files.now = time.Now
	if r, e := h.files.openArchive(ctx, ProposalActor{Policy: access.EffectivePolicy{}}, downloadArguments{}, 1); e == nil {
		t.Fatal(r)
	}
	h.files.random = failingFileRandom{}
	if r := fileResult(t, h.rpc("tools/call", "actor", fileParams(fileDownloadTool, args))); r["isError"] != true {
		t.Fatal(r)
	}
}

func TestMCPFilesIssueSQLFailures(t *testing.T) {
	for _, c := range []struct{ name, sql, needle string }{
		{"delete", `CREATE TRIGGER fail_delete BEFORE DELETE ON mcp_file_transfers BEGIN SELECT RAISE(ABORT, 'fail expire'); END`, "fail expire"},
		{"quota", `ALTER TABLE mcp_file_transfers DROP COLUMN finalized`, "finalized"},
	} {
		t.Run(c.name, func(t *testing.T) {
			h := newFileHarness(t)
			ctx := context.Background()
			db := h.jobs.Controls.Staging.DB
			if _, _, err := h.files.issue(ctx, "actor", fileDownloadTool, "{}", "", 1); err != nil {
				t.Fatal(err)
			}
			if _, err := db.Exec(c.sql); err != nil {
				t.Fatal(err)
			}
			h.files.now = func() time.Time { return time.Now().Add(2 * time.Hour) }
			if _, _, err := h.files.issue(ctx, "actor", fileDownloadTool, "{}", "", 1); err == nil || !strings.Contains(err.Error(), c.needle) {
				t.Fatal(fmt.Sprint(err))
			}
		})
	}
}

type fileFixtureReader func([]byte) (int, error)

func (f fileFixtureReader) Read(p []byte) (int, error) { return f(p) }

func TestMCPFilesOpenArchiveFailures(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	if f, err := openCheckedArchive(ctx, filepath.Join(dir, "absent"), 1, ""); err == nil || f != nil {
		t.Fatal(f, err)
	}
	path := filepath.Join(dir, "contents")
	if err := os.WriteFile(path, []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if f, err := openCheckedArchive(cancelled, path, 7, ""); err == nil || f != nil {
		t.Fatal(f, err)
	}
}

func TestMCPFilesRuntimeStorageFailure(t *testing.T) {
	ctx := context.Background()
	var cfg RuntimeConfig
	cfg.MCP = defaultMCPConfig()
	cfg.MCP.ExperimentalFiles = true
	cfg.Repository.RootPath = filepath.Join(t.TempDir(), "runtime")
	ops := defaultModelsOffBuildOps()
	ops.register = func(j *Jobs, _ *umcp.Server, _ string, _ execute.Limits) error {
		// Break only the staging handle after registration; runtime cleanup
		// still owns the original service connection and writer lease.
		j.Controls.Staging = &assets.StagingStore{}
		return nil
	}
	r, s, err := buildModelsOffRuntime(ctx, cfg, ModelsOffRuntimeOptions{Surface: "standard", Tokens: []BearerPrincipal{}}, ops)
	if err == nil || r != nil || s != nil {
		if r != nil {
			_ = r.Close(ctx)
		}
		t.Fatal(r, s, err)
	}
}
